// upy bundle <文件> [--force] [--root <dir>]: 直接部署本地 zip, 不访问 GitHub
package commands

import (
	"time"

	"github.com/AaronConlon/upy/internal/deploy"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/ui"
)

// BundleArgs bundle 命令参数
type BundleArgs struct {
	File      string
	Force     bool
	Root      string
	StartedAt time.Time
}

// Bundle 执行 bundle 命令
func Bundle(args BundleArgs) error {
	startedAt := args.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	if args.File == "" {
		return i18n.Error("bundle 命令需要一个 <文件> 参数: upy bundle <文件> [--force] [--root <目录>]")
	}
	ui.PrintProjectBanner(args.Root, ui.BannerOptions{ZipPath: args.File})

	log.Info(i18n.T("正在部署本地 bundle ") + log.Path(args.File) + forceSuffix(args.Force))
	_, err := deploy.DeployBundle(args.File, deploy.Options{
		Force:     args.Force,
		Root:      args.Root,
		StartedAt: startedAt,
	})
	if err != nil {
		return err
	}
	return nil
}
