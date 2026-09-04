// docker compose 部署器
// 流程: build 新镜像 → 停上一版本(down --remove-orphans) → up -d 新版本 → 健康检查
// 健康检查失败回滚: 先 down 失败的新版本, 再 up -d 上一版本
package deployer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AaronConlon/upy/internal/cmdrun"
	"github.com/AaronConlon/upy/internal/log"
)

// DockerComposeDeployer Docker Compose 部署器
type DockerComposeDeployer struct {
	ctx                 *DeployContext
	composeFile         string
	previousWasRunning  bool
	previousStopped     bool
	activationAttempted bool
}

// NewDockerCompose 创建 compose 部署器
func NewDockerCompose(ctx *DeployContext) *DockerComposeDeployer {
	return &DockerComposeDeployer{
		ctx:         ctx,
		composeFile: ctx.Manifest.Docker.ComposeFile,
	}
}

// composeArgsFor 针对指定目录组装 compose 参数 (当前版本 / 上一版本目录均可用)
func (d *DockerComposeDeployer) composeArgsFor(dir string, extra ...string) []string {
	f := filepath.Join(dir, d.composeFile)
	args := []string{"-f", f}
	if envFile := releaseEnvFile(dir); envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	if envFile := SharedEnvFile(d.ctx.Root); envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	return append(args, extra...)
}

// releaseEnvFile 返回 bundle 解压后的 release .env；只允许携带 CI 生成的非敏感默认配置。
// SharedEnvFile 会在其后传入，按 Docker Compose 优先级覆盖同名变量。
func releaseEnvFile(dir string) string {
	p := filepath.Join(dir, ".env")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func (d *DockerComposeDeployer) composeArgs(extra ...string) []string {
	return d.composeArgsFor(d.ctx.ReleaseDir, extra...)
}

// prevReleaseDir 上一版本目录, 不存在返回空串
func (d *DockerComposeDeployer) prevReleaseDir() string {
	if d.ctx.PreviousVersion == "" {
		return ""
	}
	dir := filepath.Join(d.ctx.Root, "releases", d.ctx.PreviousVersion)
	if _, err := os.Stat(filepath.Join(dir, d.composeFile)); err != nil {
		return ""
	}
	return dir
}

// Prepare 执行 docker compose build (构建失败不影响仍在运行的旧栈)
func (d *DockerComposeDeployer) Prepare() error {
	log.Step("正在执行 docker compose build (" + d.composeFile + ")...")
	if err := cmdrun.Run("docker", append([]string{"compose"}, d.composeArgs("build")...), cmdrun.Options{}); err != nil {
		return err
	}
	log.Ok("compose 构建完成")
	return nil
}

// Deactivate 新镜像构建成功后、启动新版本前, 对上一版本执行 down --remove-orphans:
// 释放端口 / 容器名 / 孤儿容器, 避免新旧栈共存冲突。首次部署 (无上一版本) 跳过。
func (d *DockerComposeDeployer) Deactivate() error {
	prevDir := d.prevReleaseDir()
	if prevDir == "" {
		if d.ctx.PreviousVersion == "" {
			log.Info("首次部署，无旧版本需要停止")
		} else {
			log.Info("上一版本 compose 文件已不存在，检查同名残留容器")
			if err := d.removeOrphanedNamedContainers(); err != nil {
				return err
			}
		}
		return nil
	}
	running, err := d.composeHasRunningService(prevDir)
	if err != nil {
		return fmt.Errorf("无法检查上一版本运行状态，已中止部署: %v", err)
	}
	d.previousWasRunning = running
	if !running {
		log.Info("上一版本服务在部署前未运行，将仅清理残留容器，不会在回滚时重新启动")
	}
	log.Step("正在停止上一版本 compose 服务 (" + d.ctx.PreviousVersion + ", down --remove-orphans)...")
	args := append([]string{"compose"}, d.composeArgsFor(prevDir, "down", "--remove-orphans")...)
	if err := cmdrun.Run("docker", args, cmdrun.Options{}); err != nil {
		// 停不掉旧栈就不启动新栈: 旧栈仍在运行, 立即中止部署是安全状态
		return fmt.Errorf("停止上一版本失败，为避免新旧栈冲突已中止部署: %v", err)
	}
	d.previousStopped = true
	log.Ok("上一版本已停止 (" + d.ctx.PreviousVersion + ")")
	return nil
}

// removeOrphanedNamedContainers 清理上一版本 compose 文件已丢失时遗留的显式
// container_name。只删除带 com.docker.compose.project 标签的容器；无该标签说明
// 它并非 Compose 管理，自动删除可能误伤用户自己的容器，因此直接中止部署。
func (d *DockerComposeDeployer) removeOrphanedNamedContainers() error {
	names, err := d.configuredContainerNames()
	if err != nil {
		return fmt.Errorf("无法读取当前 compose 的容器配置，已中止部署: %v", err)
	}
	for _, name := range names {
		id, composeProject, found, err := dockerContainerByName(name)
		if err != nil {
			return fmt.Errorf("无法检查同名容器 %q，已中止部署: %v", name, err)
		}
		if !found {
			continue
		}
		if composeProject == "" {
			return fmt.Errorf("容器名 %q 已被非 Compose 容器占用；为避免误删已中止部署，请确认后手动执行 docker rm -f %s", name, name)
		}
		log.Step("清理已丢失上一版本配置的 Compose 残留容器 " + name + "...")
		if err := cmdrun.Run("docker", []string{"rm", "-f", id}, cmdrun.Options{}); err != nil {
			return fmt.Errorf("清理残留容器 %q 失败，已中止部署: %v", name, err)
		}
		log.Ok("已清理 Compose 残留容器 " + name)
	}
	return nil
}

// configuredContainerNames 使用 docker compose config 取得变量展开后的显式
// container_name。未设置 container_name 的服务由 Compose 按项目名管理，不会有
// 全局同名冲突，不能也无需在缺少旧配置时猜测删除。
func (d *DockerComposeDeployer) configuredContainerNames() ([]string, error) {
	args := append([]string{"compose"}, d.composeArgs("config", "--format", "json")...)
	out, err := cmdrun.Output("docker", args, cmdrun.Options{})
	if err != nil {
		return nil, err
	}
	var config struct {
		Services map[string]struct {
			ContainerName string `json:"container_name"`
		} `json:"services"`
	}
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		return nil, fmt.Errorf("docker compose config 输出不是 JSON: %v", err)
	}
	seen := make(map[string]struct{})
	names := make([]string, 0)
	for _, service := range config.Services {
		name := strings.TrimSpace(service.ContainerName)
		if name == "" {
			continue
		}
		if !dockerContainerName.MatchString(name) {
			return nil, fmt.Errorf("compose 的 container_name %q 不合法", name)
		}
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names, nil
}

var dockerContainerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// dockerContainerByName 返回精确名称匹配的容器 ID 和 Compose 项目标签。
func dockerContainerByName(name string) (id, composeProject string, found bool, err error) {
	format := "{{.ID}}\t{{.Label \"com.docker.compose.project\"}}"
	out, err := cmdrun.Output("docker", []string{"ps", "-a", "--filter", "name=^/" + name + "$", "--format", format}, cmdrun.Options{})
	if err != nil {
		return "", "", false, err
	}
	line := strings.TrimSpace(out)
	if line == "" {
		return "", "", false, nil
	}
	parts := strings.SplitN(line, "\t", 2)
	id = strings.TrimSpace(parts[0])
	if id == "" {
		return "", "", false, fmt.Errorf("docker ps 返回了无效容器记录")
	}
	if len(parts) == 2 {
		composeProject = strings.TrimSpace(parts[1])
	}
	return id, composeProject, true, nil
}

// composeHasRunningService 只查看 running 的 service；不存在容器或全部停机都返回 false。
func (d *DockerComposeDeployer) composeHasRunningService(dir string) (bool, error) {
	args := append([]string{"compose"}, d.composeArgsFor(dir, "ps", "--status", "running", "--services")...)
	out, err := cmdrun.Output("docker", args, cmdrun.Options{})
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// Activate 执行 docker compose up -d
func (d *DockerComposeDeployer) Activate() error {
	log.Step("正在启动 compose 服务...")
	d.activationAttempted = true
	if err := cmdrun.Run("docker", append([]string{"compose"}, d.composeArgs("up", "-d")...), cmdrun.Options{}); err != nil {
		return err
	}
	log.Ok("compose 服务已启动")
	return nil
}

// HealthCheck 健康检查
func (d *DockerComposeDeployer) HealthCheck() error {
	hc := d.ctx.Manifest.Healthcheck
	if hc == nil {
		log.Info("未配置健康检查，跳过")
		return nil
	}
	log.Step("正在健康检查 " + hc.URL + "...")
	if err := PollHealth(hc.URL, HealthcheckTimeoutSec(d.ctx.Manifest)); err != nil {
		return err
	}
	log.Ok("健康检查通过")
	return nil
}

// Rollback 健康检查失败后的回滚: 先 down 失败的新版本 (释放端口), 再 up -d 上一版本
func (d *DockerComposeDeployer) Rollback() error {
	// 构建或旧栈停机阶段失败时，新版本尚未启动；不能触碰当前栈。
	if !d.activationAttempted {
		log.Info("新版本尚未尝试启动，无需回滚")
		return nil
	}

	// 先停掉失败的新版本栈 (best-effort: up 可能只创建了部分容器, down 幂等)。
	log.Step("回滚: 停止失败的新版本 compose...")
	downArgs := append([]string{"compose"}, d.composeArgs("down", "--remove-orphans")...)
	if err := cmdrun.Run("docker", downArgs, cmdrun.Options{}); err != nil {
		log.Warn("停止新版本失败，继续尝试回滚: " + err.Error())
	}

	prevDir := d.prevReleaseDir()
	if prevDir == "" {
		log.Info("没有可回滚的上一版本")
		return nil
	}
	// 旧服务尚未被 down 时，不应重启它。
	if !d.previousStopped {
		log.Info("上一版本尚未被停止，无需重新启动")
		return nil
	}
	if !d.previousWasRunning {
		log.Info("上一版本在部署前未运行，保持停止状态")
		return nil
	}

	// 再启动上一版本
	log.Step("回滚: 使用上一版本重新 compose up " + d.ctx.PreviousVersion + "...")
	upArgs := append([]string{"compose"}, d.composeArgsFor(prevDir, "up", "-d")...)
	if err := cmdrun.Run("docker", upArgs, cmdrun.Options{}); err != nil {
		return fmt.Errorf("回滚同样失败: %v", err)
	}
	log.Ok("已回滚到 " + d.ctx.PreviousVersion)
	return nil
}
