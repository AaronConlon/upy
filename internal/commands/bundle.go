// uply bundle <文件> [--force] [--root <dir>]: 直接部署本地 zip, 不访问 GitHub
package commands

import (
	"fmt"

	"github.com/AaronConlon/uply/internal/deploy"
	"github.com/AaronConlon/uply/internal/log"
	"github.com/AaronConlon/uply/internal/ui"
)

// BundleArgs bundle 命令参数
type BundleArgs struct {
	File  string
	Force bool
	Root  string
}

// Bundle 执行 bundle 命令
func Bundle(args BundleArgs) error {
	if args.File == "" {
		return fmt.Errorf("bundle 命令需要一个 <文件> 参数: uply bundle <文件> [--force] [--root <目录>]")
	}
	ui.PrintProjectBanner(args.Root, ui.BannerOptions{ZipPath: args.File})

	log.Info("正在部署本地 bundle " + log.Path(args.File) + forceSuffix(args.Force))
	res, err := deploy.DeployBundle(args.File, deploy.Options{Force: args.Force, Root: args.Root})
	if err != nil {
		return err
	}
	if res.Skipped {
		log.Info("当前已是目标版本，跳过部署（--force 可强制重新部署）。")
	}
	return nil
}
