// upy clean [项目名称] [--bundles] [--old-releases] [--all] [--purge] [--unregister] [-y] [--root <dir>]
package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AaronConlon/upy/internal/cleaner"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/ui"
)

// CleanArgs clean 命令参数
type CleanArgs struct {
	ProjectName string
	Root        string
	Bundles     bool
	OldReleases bool
	All         bool
	Purge       bool
	Unregister  bool
	Yes         bool
}

type targetItem struct {
	Name  string
	Root  string
	IsAll bool
	IsCwd bool
}

// Clean 执行 clean 命令
func Clean(args CleanArgs) error {
	// 1. 定位目标项目
	targets, isBatch, err := resolveCleanTargets(args)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		log.Info("未找到任何可清理的项目。")
		return nil
	}

	// 2. 批量清理分支
	if isBatch {
		return cleanAllProjects(targets, args)
	}

	// 3. 单项目清理
	target := targets[0]
	return cleanSingleProject(target, args)
}

func resolveCleanTargets(args CleanArgs) ([]targetItem, bool, error) {
	// 显式指定 --root
	if args.Root != "" {
		abs, err := filepath.Abs(args.Root)
		if err != nil {
			return nil, false, err
		}
		name := filepath.Base(abs)
		if cfg, err := config.ReadProjectConfig(abs); err == nil {
			name = cfg.Name
		}
		return []targetItem{{Name: name, Root: abs}}, false, nil
	}

	// 显式指定项目名称
	if args.ProjectName != "" {
		projects, err := config.ListLocalProjects()
		if err != nil {
			return nil, false, err
		}
		for _, p := range projects {
			if strings.EqualFold(p.Name, args.ProjectName) {
				return []targetItem{{Name: p.Name, Root: p.Root}}, false, nil
			}
		}
		// 检查当前目录是否刚好是该项目
		cwd, _ := os.Getwd()
		if cfg, err := config.ReadProjectConfig(cwd); err == nil && strings.EqualFold(cfg.Name, args.ProjectName) {
			return []targetItem{{Name: cfg.Name, Root: cwd}}, false, nil
		}
		return nil, false, fmt.Errorf("找不到已登记的项目 %q。可在项目目录下运行，或直接运行 upy clean 交互选择", args.ProjectName)
	}

	// 交互模式：收集可用项目
	cwd, _ := os.Getwd()
	cwdIsProject := false
	cwdName := ""
	if cfg, err := config.ReadProjectConfig(cwd); err == nil {
		cwdIsProject = true
		cwdName = cfg.Name
	}

	registered, _ := config.ListLocalProjects()

	// 构建选项列表
	var options []ui.SelectOption
	var items []targetItem

	if cwdIsProject {
		options = append(options, ui.SelectOption{
			Label: fmt.Sprintf("[当前目录项目] %s", cwdName),
			Hint:  log.Path(cwd),
		})
		items = append(items, targetItem{Name: cwdName, Root: cwd, IsCwd: true})
	}

	for _, p := range registered {
		if cwdIsProject && p.Root == cwd {
			continue // 避免与当前目录重复
		}
		options = append(options, ui.SelectOption{
			Label: p.Name,
			Hint:  log.Path(p.Root),
		})
		items = append(items, targetItem{Name: p.Name, Root: p.Root})
	}

	if len(items) == 0 {
		return nil, false, fmt.Errorf("未找到任何项目。请在包含 deploy.yaml 的项目目录下执行，或先通过 upy 部署一个项目")
	}

	// 若有多个项目，提供批量清理选项 (CLEAN-005: 提示文案与指定参数保持一致)
	if len(items) > 1 {
		batchHint := fmt.Sprintf("批量对 %d 个项目执行常规瘦身", len(items))
		if args.Purge {
			batchHint = fmt.Sprintf("⚠️ 批量彻底清空 %d 个项目的部署产物", len(items))
		} else if args.Unregister {
			batchHint = fmt.Sprintf("批量注销 %d 个项目登记（不删物理文件）", len(items))
		} else if args.Bundles && !args.OldReleases && !args.All {
			batchHint = fmt.Sprintf("批量对 %d 个项目清理 Bundle 缓存", len(items))
		} else if args.OldReleases && !args.Bundles && !args.All {
			batchHint = fmt.Sprintf("批量对 %d 个项目清理历史版本与临时目录", len(items))
		}

		options = append(options, ui.SelectOption{
			Label: "[全部已登记项目]",
			Hint:  batchHint,
		})
		items = append(items, targetItem{IsAll: true})
	}

	// 如果只有当前项目且无其他登记项目，且非强制交互，直接选定当前项目
	if len(items) == 1 && items[0].IsCwd && hasCleanFlags(args) {
		return []targetItem{items[0]}, false, nil
	}

	idx, err := ui.Select("请选择要清理的项目", options)
	if err != nil {
		return nil, false, err
	}

	selected := items[idx]
	if selected.IsAll {
		// 返回所有项目进行批量处理
		var allProjects []targetItem
		for _, it := range items {
			if !it.IsAll {
				allProjects = append(allProjects, it)
			}
		}
		return allProjects, true, nil
	}

	return []targetItem{selected}, false, nil
}

func hasCleanFlags(args CleanArgs) bool {
	return args.Bundles || args.OldReleases || args.All || args.Purge || args.Unregister
}

func cleanSingleProject(target targetItem, args CleanArgs) error {
	log.Step(fmt.Sprintf("正在分析项目 %s（%s）数据占用...", log.Sky(target.Name), log.Path(target.Root)))

	stats, err := cleaner.AnalyzeProject(target.Root)
	if err != nil {
		return fmt.Errorf("分析项目数据失败: %w", err)
	}

	// 打印占用分析结果
	printProjectStats(stats)

	// 非交互模式（指定了具体标志）
	if hasCleanFlags(args) {
		return executeFlagsClean(target, stats, args)
	}

	// 交互式菜单选择
	return interactiveCleanMenu(target, stats, args)
}

func printProjectStats(stats *cleaner.ProjectStats) {
	fmt.Printf("  📦 Bundle 压缩包缓存:   %s (%d 个文件)\n",
		log.Cyan(cleaner.FormatBytes(stats.BundleBytes)), stats.BundleFiles)

	oldVerDesc := "无"
	if len(stats.OldReleases) > 0 {
		oldVerDesc = fmt.Sprintf("%d 个旧版本: %s", len(stats.OldReleases), strings.Join(stats.OldReleases, ", "))
	}
	fmt.Printf("  📂 历史旧版本目录:     %s (%s)\n",
		log.Cyan(cleaner.FormatBytes(stats.OldReleaseBytes)), oldVerDesc)

	tmpDesc := "无"
	if len(stats.TmpDirs) > 0 {
		tmpDesc = fmt.Sprintf("%d 个临时目录", len(stats.TmpDirs))
	}
	fmt.Printf("  🗑️  部署中断残留目录:   %s (%s)\n",
		log.Cyan(cleaner.FormatBytes(stats.TmpBytes)), tmpDesc)

	// CLEAN-001 & CLEAN-002: 精准展示活跃版本、冲突及未知状态
	if stats.VersionConflict {
		fmt.Printf("  ⚠️  版本冲突: state.json 记录为 %s，但 current 软链指向 %s (%s)\n",
			log.Yellow(stats.CurrentVersion), log.Yellow(stats.SymlinkVersion), log.Yellow("两者均受保护"))
	} else if stats.ActiveVersionUnknown {
		fmt.Printf("  ⚠️  活跃版本未知: 发现 %d 个版本目录，但无法判定活跃版本 (%s)\n",
			len(stats.UnidentifiedReleases), log.Yellow("为防误删已全部保护"))
	} else if len(stats.ProtectedVersions) > 0 {
		fmt.Printf("  🛡️  当前运行活跃版本:   %s (%s)\n",
			log.Green(strings.Join(stats.ProtectedVersions, ", ")), log.Green("受保护"))
	}
	fmt.Println()
}

func interactiveCleanMenu(target targetItem, stats *cleaner.ProjectStats, args CleanArgs) error {
	var options []ui.SelectOption
	var actions []string

	totalReleasable := stats.TotalReleasableBytes()

	// 选项 1: 推荐瘦身
	recomLabel := "⚡ 推荐瘦身 (清理 Bundle 缓存 + 旧版本 + 残留临时目录)"
	if totalReleasable > 0 {
		recomLabel += fmt.Sprintf(" [%s]", cleaner.FormatBytes(totalReleasable))
	}
	options = append(options, ui.SelectOption{
		Label: recomLabel,
		Hint:  "保留当前生效版本，释放历史下载包与过期版本占用的磁盘",
	})
	actions = append(actions, "slim")

	// 选项 2: 仅清理 bundles
	bundleLabel := "📦 仅清理 Bundle 下载缓存"
	if stats.BundleBytes > 0 {
		bundleLabel += fmt.Sprintf(" [%s]", cleaner.FormatBytes(stats.BundleBytes))
	}
	options = append(options, ui.SelectOption{
		Label: bundleLabel,
		Hint:  fmt.Sprintf("删除 bundles/ 下的 %d 个 zip 压缩包", stats.BundleFiles),
	})
	actions = append(actions, "bundles")

	// 选项 3: 仅清理旧版本与临时目录
	releasesLabel := "📂 仅清理历史旧版本与残留临时目录"
	relBytes := stats.OldReleaseBytes + stats.TmpBytes
	if relBytes > 0 {
		releasesLabel += fmt.Sprintf(" [%s]", cleaner.FormatBytes(relBytes))
	}
	options = append(options, ui.SelectOption{
		Label: releasesLabel,
		Hint:  "清理非当前运行的历史版本目录及中断残留的临时目录",
	})
	actions = append(actions, "releases")

	// 选项 4: 注销项目登记
	options = append(options, ui.SelectOption{
		Label: "📑 从本地项目列表中注销此项目 (不删物理文件)",
		Hint:  "移除在全局 upy 命令中的登记，后续仍可切入项目目录部署",
	})
	actions = append(actions, "unregister")

	// 选项 5: 彻底清空
	options = append(options, ui.SelectOption{
		Label: "⚠️  彻底清空此项目所有部署产物 (高风险)",
		Hint:  "删除 releases/、bundles/、current 软链与状态，还原为未部署状态",
	})
	actions = append(actions, "purge")

	idx, err := ui.Select("请选择清理操作", options)
	if err != nil {
		return err
	}

	action := actions[idx]

	switch action {
	case "slim":
		return performSlimClean(target.Root)
	case "bundles":
		return performBundlesClean(target.Root)
	case "releases":
		return performReleasesClean(target.Root)
	case "unregister":
		return performUnregister(target.Name, target.Root)
	case "purge":
		if !args.Yes {
			ok, err := ui.Confirm(fmt.Sprintf("⚠️  确定要彻底清空项目 %s 的全部部署文件吗？此操作不可逆！", target.Name), false)
			if err != nil || !ok {
				log.Info("已取消彻底清空操作。")
				return nil
			}
		}
		return performPurgeClean(target.Name, target.Root)
	}

	return nil
}

func executeFlagsClean(target targetItem, stats *cleaner.ProjectStats, args CleanArgs) error {
	var totalFreed int64

	if args.Unregister {
		return performUnregister(target.Name, target.Root)
	}

	if args.Purge {
		if !args.Yes {
			ok, err := ui.Confirm(fmt.Sprintf("⚠️  确定要彻底清空项目 %s 的全部部署文件吗？此操作不可逆！", target.Name), false)
			if err != nil || !ok {
				log.Info("已取消彻底清空操作。")
				return nil
			}
		}
		return performPurgeClean(target.Name, target.Root)
	}

	if args.All || (args.Bundles && args.OldReleases) {
		return performSlimClean(target.Root)
	}

	var errs []error

	if args.Bundles {
		freed, count, err := cleaner.CleanBundles(target.Root)
		if err != nil {
			log.Warn("清理 Bundle 缓存出错: " + err.Error())
			errs = append(errs, err)
		} else {
			totalFreed += freed
			log.Ok(fmt.Sprintf("已删除 %d 个 Bundle 缓存压缩包（释放 %s）", count, cleaner.FormatBytes(freed)))
		}
	}

	if args.OldReleases {
		freed, cleaned, err := cleaner.CleanOldReleases(target.Root)
		if err != nil {
			log.Warn("清理历史版本出错: " + err.Error())
			errs = append(errs, err)
		} else {
			totalFreed += freed
			for _, v := range cleaned {
				log.Ok("已清理历史版本 " + v)
			}
		}

		tFreed, tCount, tErr := cleaner.CleanTmpDeployDirs(target.Root)
		if tErr != nil {
			log.Warn("清理残留临时目录出错: " + tErr.Error())
			errs = append(errs, tErr)
		} else {
			totalFreed += tFreed
			if tCount > 0 {
				log.Ok(fmt.Sprintf("已清理 %d 个残留临时目录", tCount))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("清理过程中发生错误: %w", errors.Join(errs...))
	}

	log.Ok(fmt.Sprintf("清理完成！共释放磁盘空间: %s", log.Green(log.Bold(cleaner.FormatBytes(totalFreed)))))
	return nil
}

func performSlimClean(root string) error {
	var totalFreed int64
	var errs []error

	log.Step("正在清理 Bundle 下载缓存...")
	bFreed, bCount, err := cleaner.CleanBundles(root)
	if err != nil {
		log.Warn("清理 Bundle 缓存出错: " + err.Error())
		errs = append(errs, err)
	} else {
		totalFreed += bFreed
		if bCount > 0 {
			log.Ok(fmt.Sprintf("已删除 %d 个 Bundle 压缩包 (释放 %s)", bCount, cleaner.FormatBytes(bFreed)))
		} else {
			log.Info("无 Bundle 压缩包需要清理")
		}
	}

	log.Step("正在清理历史旧版本与残留临时文件...")
	rFreed, cleaned, err := cleaner.CleanOldReleases(root)
	if err != nil {
		log.Warn("清理历史版本出错: " + err.Error())
		errs = append(errs, err)
	} else {
		totalFreed += rFreed
		for _, v := range cleaned {
			log.Ok("已删除历史版本 releases/" + v)
		}
		if len(cleaned) == 0 {
			log.Info("无历史旧版本需要清理")
		}
	}

	tFreed, tCount, tErr := cleaner.CleanTmpDeployDirs(root)
	if tErr != nil {
		log.Warn("清理残留临时目录出错: " + tErr.Error())
		errs = append(errs, tErr)
	} else {
		totalFreed += tFreed
		if tCount > 0 {
			log.Ok(fmt.Sprintf("已清理 %d 个残留临时目录 (释放 %s)", tCount, cleaner.FormatBytes(tFreed)))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("瘦身清理过程中存在失败项目: %w", errors.Join(errs...))
	}

	log.Ok(fmt.Sprintf("✨ 瘦身完成！共释放磁盘空间: %s", log.Green(log.Bold(cleaner.FormatBytes(totalFreed)))))
	return nil
}

func performBundlesClean(root string) error {
	log.Step("正在清理 Bundle 下载缓存...")
	freed, count, err := cleaner.CleanBundles(root)
	if err != nil {
		return err
	}
	if count == 0 {
		log.Info("没有发现可清理的 Bundle 压缩包。")
		return nil
	}
	log.Ok(fmt.Sprintf("✨ 已清理 %d 个 Bundle 压缩包，共释放: %s", count, log.Green(log.Bold(cleaner.FormatBytes(freed)))))
	return nil
}

func performReleasesClean(root string) error {
	var totalFreed int64
	var errs []error

	log.Step("正在清理历史旧版本与残留临时目录...")
	freed, cleaned, err := cleaner.CleanOldReleases(root)
	if err != nil {
		log.Warn("清理历史版本出错: " + err.Error())
		errs = append(errs, err)
	} else {
		totalFreed += freed
		for _, v := range cleaned {
			log.Ok("已删除历史版本 releases/" + v)
		}
	}

	tFreed, tCount, tErr := cleaner.CleanTmpDeployDirs(root)
	if tErr != nil {
		log.Warn("清理残留临时目录出错: " + tErr.Error())
		errs = append(errs, tErr)
	} else {
		totalFreed += tFreed
		if tCount > 0 {
			log.Ok(fmt.Sprintf("已清理 %d 个残留临时目录", tCount))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("清理历史版本过程中发生错误: %w", errors.Join(errs...))
	}

	if len(cleaned) == 0 && tCount == 0 {
		log.Info("没有发现可清理的历史版本或临时目录。")
		return nil
	}
	log.Ok(fmt.Sprintf("✨ 清理完成！共释放: %s", log.Green(log.Bold(cleaner.FormatBytes(totalFreed)))))
	return nil
}

func performUnregister(name, root string) error {
	log.Step(fmt.Sprintf("正在从本地项目登记表中注销 %s...", log.Sky(name)))
	if err := config.UnregisterLocalProject(name, root); err != nil {
		return fmt.Errorf("注销项目失败: %w", err)
	}
	log.Ok(fmt.Sprintf("已从本地项目登记表中移除 %s（物理文件已保留）", log.Green(name)))
	return nil
}

func performPurgeClean(name, root string) error {
	log.Step(fmt.Sprintf("正在清空项目 %s 所有部署产物...", log.Sky(name)))
	freed, err := cleaner.PurgeProject(root)
	if err != nil {
		return fmt.Errorf("清空项目产物未完全成功: %w", err)
	}
	if err := config.UnregisterLocalProject(name, root); err != nil {
		return fmt.Errorf("已清空项目产物，但从本地登记列表中注销项目失败: %w", err)
	}
	log.Ok(fmt.Sprintf("✨ 项目 %s 已彻底清空并注销，共释放磁盘空间: %s", log.Green(name), log.Green(log.Bold(cleaner.FormatBytes(freed)))))
	return nil
}

// cleanAllProjects 批量清理 (CLEAN-004 & CLEAN-005: 遵循显式参数，传播错误，准确累计)
func cleanAllProjects(targets []targetItem, args CleanArgs) error {
	if !args.Yes {
		var confirmMsg string
		if args.Purge {
			confirmMsg = fmt.Sprintf("⚠️ 确定要彻底清空全部 %d 个项目的部署产物吗？此操作不可逆！", len(targets))
		} else if args.Unregister {
			confirmMsg = fmt.Sprintf("确定要注销全部 %d 个项目登记吗？（物理文件将保留）", len(targets))
		} else if args.Bundles && !args.OldReleases && !args.All {
			confirmMsg = fmt.Sprintf("确定要清理全部 %d 个项目的 Bundle 缓存吗？", len(targets))
		} else if args.OldReleases && !args.Bundles && !args.All {
			confirmMsg = fmt.Sprintf("确定要清理全部 %d 个项目的历史版本与临时目录吗？", len(targets))
		} else {
			confirmMsg = fmt.Sprintf("确定要对全部 %d 个项目执行常规瘦身（清理缓存与旧版本）吗？", len(targets))
		}

		ok, err := ui.Confirm(confirmMsg, !args.Purge)
		if err != nil || !ok {
			log.Info("已取消批量清理。")
			return nil
		}
	}

	var errs []error
	for i, t := range targets {
		fmt.Printf("\n[%d/%d] 正在处理项目 %s (%s)...\n", i+1, len(targets), log.Sky(t.Name), log.Path(t.Root))

		var targetErr error
		if args.Unregister {
			targetErr = performUnregister(t.Name, t.Root)
		} else if args.Purge {
			targetErr = performPurgeClean(t.Name, t.Root)
		} else if args.Bundles && !args.OldReleases && !args.All {
			targetErr = performBundlesClean(t.Root)
		} else if args.OldReleases && !args.Bundles && !args.All {
			targetErr = performReleasesClean(t.Root)
		} else {
			// 常规瘦身
			targetErr = performSlimClean(t.Root)
		}

		if targetErr != nil {
			log.Warn(fmt.Sprintf("%s 处理出错: %v", t.Name, targetErr))
			errs = append(errs, fmt.Errorf("%s: %w", t.Name, targetErr))
		}
	}

	fmt.Println()
	if len(errs) > 0 {
		return fmt.Errorf("批量处理完成，但有 %d 个项目出现错误: %w", len(errs), errors.Join(errs...))
	}

	log.Ok(fmt.Sprintf("🎉 全部 %d 个项目处理完成！", len(targets)))
	return nil
}
