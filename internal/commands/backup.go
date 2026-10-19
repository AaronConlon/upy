// upy backup [项目名称] [--root <dir>] [--script <file>] [--list] [--prune <N>] [--all] [-y]
package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AaronConlon/upy/internal/backup"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/notify"
	"github.com/AaronConlon/upy/internal/ui"
)

// BackupArgs backup 命令参数
type BackupArgs struct {
	ProjectName string
	Root        string
	Script      string
	List        bool
	Prune       int
	All         bool
	Yes         bool
}

type backupTarget struct {
	Name  string
	Root  string
	IsAll bool
	IsCwd bool
}

// Backup 执行 backup 命令
func Backup(args BackupArgs) error {
	targets, isBatch, err := resolveBackupTargets(args)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		log.Info("未找到任何项目。")
		return nil
	}

	// 1. --list 列表模式
	if args.List {
		for _, target := range targets {
			if err := listProjectBackups(target); err != nil {
				return err
			}
		}
		return nil
	}

	// 2. --prune 手动轮转模式
	if args.Prune > 0 {
		for _, target := range targets {
			if err := pruneProjectBackups(target, args.Prune); err != nil {
				return err
			}
		}
		return nil
	}

	// 3. 批量备份模式
	if isBatch || args.All {
		return backupAllProjects(targets, args)
	}

	// 4. 单项目备份
	return backupSingleProject(targets[0], args)
}

func resolveBackupTargets(args BackupArgs) ([]backupTarget, bool, error) {
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
		return []backupTarget{{Name: name, Root: abs}}, false, nil
	}

	// 显式指定项目名称
	if args.ProjectName != "" {
		projects, err := config.ListLocalProjects()
		if err != nil {
			return nil, false, err
		}
		for _, p := range projects {
			if strings.EqualFold(p.Name, args.ProjectName) {
				return []backupTarget{{Name: p.Name, Root: p.Root}}, false, nil
			}
		}
		// 检查当前目录是否刚好是该项目
		cwd, _ := os.Getwd()
		if cfg, err := config.ReadProjectConfig(cwd); err == nil && strings.EqualFold(cfg.Name, args.ProjectName) {
			return []backupTarget{{Name: cfg.Name, Root: cwd}}, false, nil
		}
		return nil, false, fmt.Errorf("找不到已登记的项目 %q。可在项目目录下运行，或直接运行 upy backup 交互选择", args.ProjectName)
	}

	// 显式指定 --all
	if args.All {
		registered, err := config.ListLocalProjects()
		if err != nil {
			return nil, false, err
		}
		var list []backupTarget
		for _, p := range registered {
			list = append(list, backupTarget{Name: p.Name, Root: p.Root})
		}
		if len(list) == 0 {
			// 兜底当前目录
			cwd, _ := os.Getwd()
			if cfg, err := config.ReadProjectConfig(cwd); err == nil {
				list = append(list, backupTarget{Name: cfg.Name, Root: cwd, IsCwd: true})
			}
		}
		if len(list) == 0 {
			return nil, false, fmt.Errorf("未找到任何已登记项目，无法执行批量备份")
		}
		return list, true, nil
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

	var options []ui.SelectOption
	var items []backupTarget

	if cwdIsProject {
		options = append(options, ui.SelectOption{
			Label: fmt.Sprintf("[当前目录项目] %s", cwdName),
			Hint:  log.Path(cwd),
		})
		items = append(items, backupTarget{Name: cwdName, Root: cwd, IsCwd: true})
	}

	for _, p := range registered {
		if cwdIsProject && p.Root == cwd {
			continue
		}
		options = append(options, ui.SelectOption{
			Label: p.Name,
			Hint:  log.Path(p.Root),
		})
		items = append(items, backupTarget{Name: p.Name, Root: p.Root})
	}

	if len(items) == 0 {
		return nil, false, fmt.Errorf("未找到任何项目。请在包含 deploy.yaml 的项目目录下执行，或先通过 upy 部署一个项目")
	}

	if len(items) > 1 {
		options = append(options, ui.SelectOption{
			Label: "[全部已登记项目]",
			Hint:  fmt.Sprintf("批量对 %d 个项目依次执行约定备份", len(items)),
		})
		items = append(items, backupTarget{Name: "全部项目", IsAll: true})
	}

	idx, err := ui.Select("请选择要备份的项目:", options)
	if err != nil {
		return nil, false, err
	}

	selected := items[idx]
	if selected.IsAll {
		var allTargets []backupTarget
		for _, it := range items {
			if !it.IsAll {
				allTargets = append(allTargets, it)
			}
		}
		return allTargets, true, nil
	}

	return []backupTarget{selected}, false, nil
}

func listProjectBackups(target backupTarget) error {
	backupDir := filepath.Join(target.Root, backup.DefaultBackupDir)
	if cfg, err := config.ReadProjectConfig(target.Root); err == nil && cfg.Backup != nil && cfg.Backup.Dir != "" {
		if filepath.IsAbs(cfg.Backup.Dir) {
			backupDir = cfg.Backup.Dir
		} else {
			backupDir = filepath.Join(target.Root, cfg.Backup.Dir)
		}
	}

	files, err := backup.ScanBackups(backupDir)
	if err != nil {
		return err
	}

	fmt.Printf("\n项目 %s 历史备份列表 (%s):\n\n", log.Sky(target.Name), log.Path(backupDir))
	if len(files) == 0 {
		fmt.Println("  (当前暂无备份产物)")
		return nil
	}

	var totalSize int64
	fmt.Printf("  %-48s %-12s %s\n", "文件名称", "文件体积", "生成时间")
	fmt.Println("  " + strings.Repeat("-", 76))
	for _, f := range files {
		totalSize += f.Size
		fmt.Printf("  %-48s %-12s %s\n", f.Name, humanSize(f.Size), f.ModTime.Format("2006-01-02 15:04:05"))
	}
	fmt.Println("  " + strings.Repeat("-", 76))
	fmt.Printf("  累计 %d 个备份文件，总占用空间: %s\n\n", len(files), humanSize(totalSize))
	return nil
}

func pruneProjectBackups(target backupTarget, keep int) error {
	backupDir := filepath.Join(target.Root, backup.DefaultBackupDir)
	if cfg, err := config.ReadProjectConfig(target.Root); err == nil && cfg.Backup != nil && cfg.Backup.Dir != "" {
		if filepath.IsAbs(cfg.Backup.Dir) {
			backupDir = cfg.Backup.Dir
		} else {
			backupDir = filepath.Join(target.Root, cfg.Backup.Dir)
		}
	}

	deleted, err := backup.PruneBackups(backupDir, keep)
	if err != nil {
		return err
	}

	if len(deleted) == 0 {
		log.Info(fmt.Sprintf("项目 %s 备份数量未超过 %d 份，无需清理。", target.Name, keep))
		return nil
	}

	log.Ok(fmt.Sprintf("项目 %s 成功清理 %d 份陈旧备份 (保留最近 %d 份):", target.Name, len(deleted), keep))
	for _, f := range deleted {
		fmt.Printf("  - %s\n", f)
	}
	return nil
}

func backupSingleProject(target backupTarget, args BackupArgs) error {
	log.Step(fmt.Sprintf("正在对项目 %s 执行约定备份...", log.Sky(target.Name)))

	opts := backup.Options{
		Root:         target.Root,
		CustomScript: args.Script,
		Retention:    args.Prune,
	}

	ctx := context.Background()
	result, err := backup.Run(ctx, opts)
	if err != nil {
		notify.Send(notify.Event{
			Kind:    notify.KindBackupFailure,
			Project: target.Name,
			Root:    target.Root,
			Error:   err.Error(),
		})
		return err
	}

	log.Ok(fmt.Sprintf("项目 %s 备份执行完成！", target.Name))
	if len(result.CreatedFiles) > 0 {
		for _, f := range result.CreatedFiles {
			fmt.Printf("  📦 归档产物: %s (%s)\n", log.Sky(f.Name), humanSize(f.Size))
		}
	} else {
		fmt.Printf("  📦 备份目录: %s (脚本未输出新增文件)\n", log.Path(result.BackupDir))
	}

	if len(result.PrunedFiles) > 0 {
		fmt.Printf("  🛡️  自动轮转: 清理了 %d 份历史备份\n", len(result.PrunedFiles))
	}

	fmt.Printf("  ⏱  总耗时:   %s\n", ui.FormatDuration(result.Duration))

	notify.Send(notify.Event{
		Kind:    notify.KindBackupSuccess,
		Project: target.Name,
		Root:    target.Root,
		Version: summarizeFiles(result.CreatedFiles),
		Elapsed: ui.FormatDuration(result.Duration),
	})

	return nil
}

func backupAllProjects(targets []backupTarget, args BackupArgs) error {
	fmt.Printf("\n🚀 准备对全部 %d 个项目依次执行约定备份...\n\n", len(targets))

	var successCount int
	var failCount int
	var totalBytes int64

	for i, target := range targets {
		fmt.Printf("[%d/%d] 正在备份项目 %s (%s)...\n", i+1, len(targets), log.Sky(target.Name), log.Path(target.Root))
		opts := backup.Options{
			Root:         target.Root,
			CustomScript: args.Script,
			Retention:    args.Prune,
		}

		ctx := context.Background()
		res, err := backup.Run(ctx, opts)
		if err != nil {
			log.Warn(fmt.Sprintf("项目 %s 备份失败: %v", target.Name, err))
			failCount++
			notify.Send(notify.Event{
				Kind:    notify.KindBackupFailure,
				Project: target.Name,
				Root:    target.Root,
				Error:   err.Error(),
			})
			continue
		}

		successCount++
		totalBytes += res.TotalBytes
		var outName string
		if len(res.CreatedFiles) > 0 {
			outName = res.CreatedFiles[0].Name
		} else {
			outName = "完成"
		}
		log.Ok(fmt.Sprintf("项目 %s 备份成功 (%s, 耗时 %s)", target.Name, outName, ui.FormatDuration(res.Duration)))

		notify.Send(notify.Event{
			Kind:    notify.KindBackupSuccess,
			Project: target.Name,
			Root:    target.Root,
			Version: summarizeFiles(res.CreatedFiles),
			Elapsed: ui.FormatDuration(res.Duration),
		})
	}

	fmt.Printf("\n🎉 批量备份结束：成功 %d 个，失败 %d 个，累计生成产物 %s\n\n",
		successCount, failCount, humanSize(totalBytes))
	return nil
}

func summarizeFiles(files []backup.BackupFile) string {
	if len(files) == 0 {
		return "备份成功"
	}
	var names []string
	for _, f := range files {
		names = append(names, fmt.Sprintf("%s (%s)", f.Name, humanSize(f.Size)))
	}
	return strings.Join(names, ", ")
}
