// docker compose 部署器
// 流程: build 新镜像 → 停上一版本(down --remove-orphans) → up -d 新版本 → 健康检查
// 健康检查失败回滚: 先 down 失败的新版本, 再 up -d 上一版本
package deployer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AaronConlon/upy/internal/cmdrun"
	"github.com/AaronConlon/upy/internal/log"
)

// DockerComposeDeployer Docker Compose 部署器
type DockerComposeDeployer struct {
	ctx         *DeployContext
	composeFile string
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
	if envFile := SharedEnvFile(d.ctx.Root); envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	return append(args, extra...)
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
			log.Info("上一版本 compose 文件已不存在，跳过停机")
		}
		return nil
	}
	log.Step("正在停止上一版本 compose 服务 (" + d.ctx.PreviousVersion + ", down --remove-orphans)...")
	args := append([]string{"compose"}, d.composeArgsFor(prevDir, "down", "--remove-orphans")...)
	if err := cmdrun.Run("docker", args, cmdrun.Options{}); err != nil {
		// 停不掉旧栈就不启动新栈: 旧栈仍在运行, 立即中止部署是安全状态
		return fmt.Errorf("停止上一版本失败，为避免新旧栈冲突已中止部署: %v", err)
	}
	log.Ok("上一版本已停止 (" + d.ctx.PreviousVersion + ")")
	return nil
}

// Activate 执行 docker compose up -d
func (d *DockerComposeDeployer) Activate() error {
	log.Step("正在启动 compose 服务...")
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
	prevDir := d.prevReleaseDir()
	if prevDir == "" {
		log.Info("没有可回滚的上一版本")
		return nil
	}

	// 先停掉失败的新版本栈 (best-effort: 新版本可能尚未启动, down 幂等)
	log.Step("回滚: 停止失败的新版本 compose...")
	downArgs := append([]string{"compose"}, d.composeArgs("down", "--remove-orphans")...)
	if err := cmdrun.Run("docker", downArgs, cmdrun.Options{}); err != nil {
		log.Warn("停止新版本失败，继续尝试回滚: " + err.Error())
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
