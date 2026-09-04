// static 部署器: Prepare 校验 dist/index.html; Activate 原子切换 current -> dist 目录
package deployer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AaronConlon/upy/internal/log"
)

// StaticDeployer 静态站点部署器
type StaticDeployer struct{ ctx *DeployContext }

// NewStatic 创建静态部署器
func NewStatic(ctx *DeployContext) *StaticDeployer { return &StaticDeployer{ctx: ctx} }

func (s *StaticDeployer) directory() string {
	if s.ctx.Manifest.Static != nil && strings.TrimSpace(s.ctx.Manifest.Static.Directory) != "" {
		return s.ctx.Manifest.Static.Directory
	}
	return "dist"
}

// Prepare 校验静态目录与 index.html
func (s *StaticDeployer) Prepare() error {
	dir := filepath.Join(s.ctx.ReleaseDir, s.directory())
	idx := filepath.Join(dir, "index.html")
	log.Step("正在校验静态 bundle...")
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("bundle 中缺少静态目录: %s", s.directory())
	}
	if _, err := os.Stat(idx); err != nil {
		return fmt.Errorf("静态 bundle 缺少入口文件 (期望位于 %s/index.html)", s.directory())
	}
	log.Ok("静态 bundle 校验通过 (" + s.directory() + "/)")
	return nil
}

// Activate 原子切换 current 软链
func (s *StaticDeployer) Activate() error {
	log.Step("正在激活静态版本...")
	target, err := filepath.Abs(filepath.Join(s.ctx.ReleaseDir, s.directory()))
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("静态版本目录不存在: %s", target)
	}
	if err := AtomicSymlink(target, filepath.Join(s.ctx.Root, "current")); err != nil {
		return err
	}
	rel := strings.TrimPrefix(s.ctx.ReleaseDir, s.ctx.Root+string(filepath.Separator))
	log.Ok("current -> " + rel + "/" + s.directory())
	return nil
}

// HealthCheck 健康检查 (仅在 manifest 配置时)
func (s *StaticDeployer) HealthCheck() error {
	hc := s.ctx.Manifest.Healthcheck
	if hc == nil {
		log.Info("未配置健康检查，跳过")
		return nil
	}
	log.Step("正在健康检查 " + hc.URL + "...")
	if err := PollHealth(hc.URL, HealthcheckTimeoutSec(s.ctx.Manifest)); err != nil {
		return err
	}
	log.Ok("健康检查通过")
	return nil
}

// Rollback best-effort 切回上一版本
func (s *StaticDeployer) Rollback() error {
	log.Step("回滚: 恢复上一静态版本...")
	if s.ctx.PreviousVersion == "" {
		log.Info("没有可回滚的上一版本")
		return nil
	}
	prevDir := filepath.Join(s.ctx.Root, "releases", s.ctx.PreviousVersion, s.directory())
	if _, err := os.Stat(prevDir); err != nil {
		log.Info("上一版本目录已不存在，保留当前版本")
		return nil
	}
	if err := AtomicSymlink(prevDir, filepath.Join(s.ctx.Root, "current")); err != nil {
		return err
	}
	log.Ok("已回滚到 " + s.ctx.PreviousVersion)
	return nil
}
