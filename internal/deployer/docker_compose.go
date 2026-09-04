// docker compose 部署器
package deployer

import (
	"fmt"
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

func (d *DockerComposeDeployer) composeArgs(extra ...string) []string {
	f := filepath.Join(d.ctx.ReleaseDir, d.composeFile)
	args := []string{"-f", f}
	if envFile := SharedEnvFile(d.ctx.Root); envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	return append(args, extra...)
}

// Prepare 执行 docker compose build
func (d *DockerComposeDeployer) Prepare() error {
	log.Step("正在执行 docker compose build (" + d.composeFile + ")...")
	if err := cmdrun.Run("docker", append([]string{"compose"}, d.composeArgs("build")...), cmdrun.Options{}); err != nil {
		return err
	}
	log.Ok("compose 构建完成")
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

// Rollback best-effort: 到 previous release 目录再 up -d
func (d *DockerComposeDeployer) Rollback() error {
	if d.ctx.PreviousVersion == "" {
		log.Info("没有可回滚的上一版本")
		return nil
	}
	prevDir := filepath.Join(d.ctx.Root, "releases", d.ctx.PreviousVersion)
	log.Step("回滚: 使用上一版本重新 compose up " + d.ctx.PreviousVersion + "...")
	f := filepath.Join(prevDir, d.composeFile)
	args := []string{"compose", "-f", f}
	if envFile := SharedEnvFile(d.ctx.Root); envFile != "" {
		args = append(args, "--env-file", envFile)
	}
	args = append(args, "up", "-d")
	if err := cmdrun.Run("docker", args, cmdrun.Options{}); err != nil {
		return fmt.Errorf("回滚同样失败: %v", err)
	}
	log.Ok("已回滚到 " + d.ctx.PreviousVersion)
	return nil
}
