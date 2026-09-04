// docker container 部署器
// 镜像名 deploy/<project>:<version>; 容器名固定 deploy-<project>
// Prepare: docker build (失败不停旧容器); Activate: 停删旧容器后 docker run
package deployer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AaronConlon/uply/internal/cmdrun"
	"github.com/AaronConlon/uply/internal/log"
)

// DockerContainerDeployer Docker 容器部署器
type DockerContainerDeployer struct {
	ctx       *DeployContext
	image     string
	container string
}

// NewDockerContainer 创建容器部署器
func NewDockerContainer(ctx *DeployContext) *DockerContainerDeployer {
	return &DockerContainerDeployer{
		ctx:       ctx,
		image:     "deploy/" + Slug(ctx.ProjectName) + ":" + Slug(ctx.Version),
		container: "deploy-" + Slug(ctx.ProjectName),
	}
}

// Prepare 构建镜像 (失败不影响旧容器)
func (d *DockerContainerDeployer) Prepare() error {
	df := d.ctx.Manifest.Docker.Dockerfile
	log.Step("正在构建 Docker 镜像 " + d.image + "...")
	// -f 必须用绝对路径: docker CLI 的 dockerfile 路径相对当前工作目录解析
	dfAbs := filepath.Join(d.ctx.ReleaseDir, df)
	if err := cmdrun.Run("docker", []string{"build", "-f", dfAbs, "-t", d.image, d.ctx.ReleaseDir}, cmdrun.Options{Cwd: d.ctx.ReleaseDir}); err != nil {
		return err
	}
	log.Ok("镜像构建完成")
	return nil
}

func (d *DockerContainerDeployer) stopAndRemove() {
	cmdrun.Run("docker", []string{"stop", d.container}, cmdrun.Options{AllowNonZero: true})
	cmdrun.Run("docker", []string{"rm", "-f", d.container}, cmdrun.Options{AllowNonZero: true})
}

// runArgs 组装 docker run 参数: 端口 / 数据卷 / 网络 / env-file
func (d *DockerContainerDeployer) runArgs(image string) []string {
	args := []string{"run", "-d", "--name", d.container, "--restart", "unless-stopped"}
	port := d.ctx.Manifest.Docker.Port
	if port != "" {
		for _, p := range strings.Split(port, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				args = append(args, "-p", p)
			}
		}
	}
	// 默认持久化数据卷: <root>/shared/data:/data (SQLite 等跨版本存活)
	dataDir := filepath.Join(d.ctx.Root, "shared", "data")
	os.MkdirAll(dataDir, 0o755)
	args = append(args, "-v", dataDir+":/data")
	for _, v := range d.ctx.Manifest.Docker.Volumes {
		args = append(args, "-v", v)
	}
	if net := d.ctx.Manifest.Docker.Network; net != "" {
		args = append(args, "--network", net)
	}
	if envFile := SharedEnvFile(d.ctx.Root); envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, image)
	return args
}

// Activate 停旧删旧后启动新容器
func (d *DockerContainerDeployer) Activate() error {
	log.Step("正在启动容器 " + d.container + "...")
	d.stopAndRemove()
	if err := cmdrun.Run("docker", d.runArgs(d.image), cmdrun.Options{}); err != nil {
		return err
	}
	log.Ok("容器已启动")
	return nil
}

// HealthCheck 健康检查
func (d *DockerContainerDeployer) HealthCheck() error {
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

// Rollback 停删失败容器, 有 previous 则用上一版镜像重启
func (d *DockerContainerDeployer) Rollback() error {
	log.Step("回滚: 停止失败容器...")
	d.stopAndRemove()
	if d.ctx.PreviousVersion == "" {
		log.Info("没有可回滚的上一版本")
		return nil
	}
	prevImage := "deploy/" + Slug(d.ctx.ProjectName) + ":" + Slug(d.ctx.PreviousVersion)
	log.Step("回滚: 重新启动上一版本 " + prevImage + "...")
	if err := cmdrun.Run("docker", d.runArgs(prevImage), cmdrun.Options{}); err != nil {
		return fmt.Errorf("回滚同样失败: %v", err)
	}
	log.Ok("已回滚到 " + d.ctx.PreviousVersion)
	return nil
}
