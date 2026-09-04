// uply release [list|latest|<版本>] [--force] [--root <dir>]
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AaronConlon/uply/internal/config"
	"github.com/AaronConlon/uply/internal/deploy"
	"github.com/AaronConlon/uply/internal/github"
	"github.com/AaronConlon/uply/internal/log"
	"github.com/AaronConlon/uply/internal/ui"
)

// ReleaseArgs release 命令参数
type ReleaseArgs struct {
	Target string
	Force  bool
	Root   string
}

// Release 执行 release 命令
func Release(args ReleaseArgs) error {
	ui.PrintProjectBanner(args.Root, ui.BannerOptions{})

	cfg, err := config.ReadProjectConfig(args.Root)
	if err != nil {
		return err
	}

	if args.Target == "list" {
		return listReleaseHistory(cfg.Release.Repository)
	}

	var release *github.GHRelease
	switch {
	case args.Target == "":
		log.Step("正在列出 " + log.Sky(cfg.Release.Repository) + " 的版本...")
		rels, err := github.ListReleases(cfg.Release.Repository)
		if err != nil {
			return err
		}
		if len(rels) == 0 {
			return fmt.Errorf("仓库 %s 没有任何已发布版本。", cfg.Release.Repository)
		}
		opts := make([]ui.SelectOption, len(rels))
		for i, r := range rels {
			opts[i] = ui.SelectOption{Label: r.Tag, Hint: r.Name}
		}
		idx, err := ui.Select("请选择要部署的版本", opts)
		if err != nil {
			return err
		}
		release = &rels[idx]
	case args.Target == "latest":
		log.Step("正在获取 " + log.Sky(cfg.Release.Repository) + " 的最新版本...")
		release, err = github.GetLatest(cfg.Release.Repository)
		if err != nil {
			return err
		}
	default:
		log.Step("正在获取 " + log.Sky(cfg.Release.Repository) + " 的 " + log.Sky(args.Target) + " 版本...")
		release, err = github.GetByTag(cfg.Release.Repository, args.Target)
		if err != nil {
			return err
		}
	}
	log.Ok("已选择版本 " + log.Green(release.Tag))

	// 资产名不固定 (如 "项目名-版本tag-日期.zip"): 先精确匹配, 未命中则按 项目名+版本号 模糊匹配
	repoShort := ""
	if parts := strings.Split(cfg.Release.Repository, "/"); len(parts) == 2 {
		repoShort = parts[1]
	}
	asset, err := github.FindAsset(release, cfg.Release.Asset, cfg.Name, repoShort)
	if err != nil {
		return err
	}

	// bundle 下载保留到 <root>/bundles/<资产原始名>, 已存在则复用 (除非 --force)
	// 文件名保留下载资产的完整语义, 如 test-deploy-website-v0.5.2-20260815.zip
	bundlesDir := filepath.Join(args.Root, "bundles")
	os.MkdirAll(bundlesDir, 0o755)
	localZip := filepath.Join(bundlesDir, asset.Name)

	zipPath := localZip
	if _, err := os.Stat(localZip); err == nil && !args.Force {
		log.Info("📦 本地已存在 bundle，直接复用 " + log.Path(localZip) + "（--force 可强制重新下载）")
	} else {
		if _, err := os.Stat(localZip); err == nil && args.Force {
			log.Info("♻️  --force 已指定，重新下载覆盖本地 bundle")
		}
		log.Step("⬇️  正在下载资产 " + asset.Name + "（" + humanSize(asset.Size) + "）...")
		if err := github.DownloadAsset(cfg.Release.Repository, asset, localZip); err != nil {
			return err
		}
		log.Ok("已保存到 " + log.Path(localZip))
	}

	log.Info(fmt.Sprintf("开始部署 %s v%s%s", cfg.Name, stripV(release.Tag), forceSuffix(args.Force)))
	res, err := deploy.DeployBundle(zipPath, deploy.Options{Force: args.Force, Root: args.Root})
	if err != nil {
		return err
	}
	if res.Skipped {
		log.Info("当前已是目标版本，跳过部署（--force 可强制重新部署）。")
	}
	return nil
}

func listReleaseHistory(repo string) error {
	log.Step("正在列出 " + log.Sky(repo) + " 的版本...")
	rels, err := github.ListReleases(repo)
	if err != nil {
		return err
	}
	if len(rels) == 0 {
		log.Info("仓库 " + repo + " 没有任何已发布版本。")
		return nil
	}
	fmt.Println("\n发布历史:")
	for i, r := range rels {
		assets := "无资产"
		if len(r.Assets) > 0 {
			parts := make([]string, 0, len(r.Assets))
			for _, a := range r.Assets {
				parts = append(parts, a.Name+"（"+humanSize(a.Size)+"）")
			}
			assets = joinStr(parts, "，")
		}
		flags := ""
		if i == 0 {
			flags = "  [最新]"
		}
		if r.Prerelease {
			flags += "  [预发布]"
		}
		fmt.Printf("  %s%s\n", r.Tag, flags)
		fmt.Printf("    %s — %s\n", r.Name, assets)
	}
	return nil
}

func joinStr(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
