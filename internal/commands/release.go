// upy release [list|latest|<版本>] [--force] [--root <dir>]
package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/bundle"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/deploy"
	"github.com/AaronConlon/upy/internal/github"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/ui"
)

// ReleaseArgs release 命令参数
type ReleaseArgs struct {
	Target    string
	Force     bool
	Root      string
	StartedAt time.Time
}

// Release 执行 release 命令
func Release(args ReleaseArgs) error {
	startedAt := args.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
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
		log.Step(i18n.T("正在列出 ") + log.Sky(cfg.Release.Repository) + i18n.T(" 的版本..."))
		rels, err := github.ListReleases(cfg.Release.Repository)
		if err != nil {
			return err
		}
		if len(rels) == 0 {
			return fmt.Errorf(i18n.T("仓库 %s 没有任何已发布版本。"), cfg.Release.Repository)
		}
		opts := make([]ui.SelectOption, len(rels))
		for i, r := range rels {
			opts[i] = ui.SelectOption{Label: r.Tag, Hint: r.Name}
		}
		idx, err := ui.Select(i18n.T("请选择要部署的版本"), opts)
		if err != nil {
			return err
		}
		release = &rels[idx]
	case args.Target == "latest":
		log.Step(i18n.T("正在获取 ") + log.Sky(cfg.Release.Repository) + i18n.T(" 的最新版本..."))
		release, err = github.GetLatest(cfg.Release.Repository)
		if err != nil {
			return err
		}
	default:
		log.Step(i18n.T("正在获取 ") + log.Sky(cfg.Release.Repository) + i18n.T(" 的 ") + log.Sky(args.Target) + i18n.T(" 版本..."))
		release, err = github.GetByTag(cfg.Release.Repository, args.Target)
		if err != nil {
			return err
		}
	}
	log.Ok(i18n.T("已选择版本 ") + log.Green(release.Tag))

	// 资产名不固定 (如 "项目名-版本tag-日期.zip"): 先精确匹配, 未命中则按 项目名+版本号 模糊匹配
	repoShort := ""
	if parts := strings.Split(cfg.Release.Repository, "/"); len(parts) == 2 {
		repoShort = parts[1]
	}
	asset, err := github.FindAsset(release, cfg.Release.Asset, cfg.Name, repoShort)
	if err != nil {
		return err
	}

	// bundle 缓存保留到 <root>/bundles/:
	// - 资产名本身含版本号 (如 test-deploy-website-v0.5.2-20260815.zip) → 沿用资产原始名
	// - 资产名固定不含版本 (如 bundle.zip) → 加上 <tag>- 前缀, 避免不同版本互相覆盖复用
	bundlesDir := filepath.Join(args.Root, "bundles")
	os.MkdirAll(bundlesDir, 0o755)
	_ = os.Chmod(bundlesDir, 0o755)
	cacheName := asset.Name
	if !assetNameHasTag(asset.Name, release.Tag) {
		cacheName = release.Tag + "-" + asset.Name
	}
	localZip := filepath.Join(bundlesDir, cacheName)

	zipPath := localZip
	reuseCache := checkBundleCache(localZip, release.Tag, args.Force)

	if !reuseCache {
		_ = os.Remove(localZip)
		log.Step(i18n.T("⬇️  正在下载资产 ") + asset.Name + "（" + humanSize(asset.Size) + "）...")
		if err := github.DownloadAsset(cfg.Release.Repository, asset, localZip); err != nil {
			_ = os.Remove(localZip)
			return err
		}
		if v := bundle.ManifestVersionFromZip(localZip); v == "" {
			_ = os.Remove(localZip)
			return i18n.Error("下载完成但 bundle 格式无效或缺少 upy.yaml 清单文件，已清理损坏文件")
		}
		log.Ok(i18n.T("已保存到 ") + log.Path(localZip))
	}

	log.Info(fmt.Sprintf(i18n.T("开始部署 %s v%s%s"), cfg.Name, stripV(release.Tag), forceSuffix(args.Force)))
	_, err = deploy.DeployBundle(zipPath, deploy.Options{
		Force:     args.Force,
		Root:      args.Root,
		StartedAt: startedAt,
	})
	if err != nil {
		return err
	}
	// 只在 release 完整成功后登记；下载、解压或部署失败时不会污染本地项目列表。
	if err := config.SaveLocalProject(cfg.Name, args.Root); err != nil {
		log.Warn(i18n.T("部署成功，但无法记录本地项目: ") + err.Error())
	}
	return nil
}

func listReleaseHistory(repo string) error {
	log.Step(i18n.T("正在列出 ") + log.Sky(repo) + i18n.T(" 的版本..."))
	rels, err := github.ListReleases(repo)
	if err != nil {
		return err
	}
	if len(rels) == 0 {
		log.Info(i18n.T("仓库 ") + repo + i18n.T(" 没有任何已发布版本。"))
		return nil
	}
	fmt.Println(i18n.T("\n发布历史:"))
	for i, r := range rels {
		assets := i18n.T("无资产")
		if len(r.Assets) > 0 {
			parts := make([]string, 0, len(r.Assets))
			for _, a := range r.Assets {
				parts = append(parts, a.Name+"（"+humanSize(a.Size)+"）")
			}
			assets = joinStr(parts, "，")
		}
		flags := ""
		if i == 0 {
			flags = i18n.T("  [最新]")
		}
		if r.Prerelease {
			flags += i18n.T("  [预发布]")
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

// assetNameHasTag 判断资产文件名是否已包含版本语义 (大小写不敏感, 兼容带/不带 v 前缀)
func assetNameHasTag(assetName, tag string) bool {
	n := strings.ToLower(assetName)
	if strings.Contains(n, strings.ToLower(tag)) {
		return true
	}
	return strings.Contains(n, strings.TrimPrefix(strings.ToLower(tag), "v"))
}

// sameVersion 比较 manifest 版本与 release tag 是否一致 (大小写与 v 前缀不敏感)
func sameVersion(manifestVersion, tag string) bool {
	mv := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(manifestVersion)), "v")
	tv := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(tag)), "v")
	return mv == tv
}

// checkBundleCache 检查本地 bundle 缓存是否可直接复用；若损坏或版本不一致则清理
func checkBundleCache(localZip, targetTag string, force bool) bool {
	if _, err := os.Stat(localZip); err != nil {
		return false
	}
	if force {
		log.Info(i18n.T("♻️  --force 已指定，重新下载覆盖本地 bundle"))
		return false
	}
	// 复用前校验: zip 必须有效且 manifest 的 version 必须与目标版本一致, 否则清理并重新下载
	v := bundle.ManifestVersionFromZip(localZip)
	if v == "" {
		log.Warn(i18n.T("⚠️  本地缓存 bundle 损坏或无效，清理后重新下载"))
		_ = os.Remove(localZip)
		return false
	}
	if !sameVersion(v, targetTag) {
		log.Info(fmt.Sprintf(i18n.T("⚠️  本地缓存 bundle 的版本是 %s，与目标版本 %s 不一致，重新下载"), v, targetTag))
		_ = os.Remove(localZip)
		return false
	}
	log.Info(i18n.T("📦 本地已存在 bundle，直接复用 ") + log.Path(localZip) + i18n.T("（--force 可强制重新下载）"))
	return true
}
