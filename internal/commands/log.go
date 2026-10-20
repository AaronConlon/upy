package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/server"
	"github.com/AaronConlon/upy/internal/ui"
	"golang.org/x/term"
)

type LogArgs struct {
	Target string // "server", "upy", or project name (or empty)
	Follow bool   // -f, --follow
	Tail   int    // -n, --lines, --tail (default 50)
	Clear  bool   // -c, --clear
	List   bool   // --list
	Server bool   // -s, --server
}

func Log(args LogArgs) error {
	logsDir := filepath.Join(config.UserDataDir(), "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	// 1. --list 模式: 列出所有可用日志
	if args.List {
		return listAllLogs(logsDir)
	}

	target := strings.TrimSpace(args.Target)
	if args.Server {
		target = "server"
	}

	// 2. 若未指定 target，尝试交互式选择或默认展示
	if target == "" {
		resolved, ok, err := selectLogTarget(logsDir)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		target = resolved
	}

	// 3. 定位日志文件路径
	logPath, label, err := resolveLogFilePath(logsDir, target)
	if err != nil {
		return err
	}

	// 4. -c, --clear 清空日志
	if args.Clear {
		if err := os.WriteFile(logPath, []byte{}, 0o644); err != nil {
			return fmt.Errorf(i18n.T("清空日志文件失败: %w"), err)
		}
		log.Ok(i18n.T("已成功清空日志: ") + log.Sky(label) + " (" + log.Path(logPath) + ")")
		return nil
	}

	// 5. 检查文件是否存在
	if _, err := os.Stat(logPath); err != nil {
		if os.IsNotExist(err) {
			log.Info(i18n.T("日志文件尚未生成: ") + log.Path(logPath))
			if !args.Follow {
				return nil
			}
		} else {
			return err
		}
	}

	// 6. 展示历史末尾 N 行
	tailLines := args.Tail
	if tailLines <= 0 {
		tailLines = 50
	}

	log.Step(fmt.Sprintf(i18n.T("查看日志 %s (末尾 %d 行): %s"), log.Sky(label), tailLines, log.Path(logPath)))

	lines, lastOffset, _ := readLastNLines(logPath, tailLines)
	for _, l := range lines {
		fmt.Println(l)
	}

	// 7. -f, --follow 实时追踪
	if args.Follow {
		return followLog(logPath, lastOffset)
	}

	return nil
}

func resolveLogFilePath(logsDir, target string) (path string, label string, err error) {
	lower := strings.ToLower(target)
	switch lower {
	case "server":
		return server.ServerLogPath(), i18n.T("server 服务日志"), nil
	case "upy", "cli", "self":
		return filepath.Join(logsDir, "upy.log"), i18n.T("upy 运行日志"), nil
	}

	// 尝试作为项目目录处理
	projDir := filepath.Join(logsDir, target)
	if fi, err := os.Stat(projDir); err == nil && fi.IsDir() {
		entries, err := os.ReadDir(projDir)
		if err == nil {
			var logFiles []string
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
					logFiles = append(logFiles, filepath.Join(projDir, e.Name()))
				}
			}
			if len(logFiles) > 0 {
				// 按时间降序，选最新的一份
				sort.Slice(logFiles, func(i, j int) bool {
					infoI, _ := os.Stat(logFiles[i])
					infoJ, _ := os.Stat(logFiles[j])
					if infoI == nil || infoJ == nil {
						return false
					}
					return infoI.ModTime().After(infoJ.ModTime())
				})
				latest := logFiles[0]
				return latest, fmt.Sprintf(i18n.T("项目 %s 部署日志 (%s)"), target, filepath.Base(latest)), nil
			}
		}
	}

	// 尝试直接作为文件名匹配
	direct := filepath.Join(logsDir, target)
	if _, err := os.Stat(direct); err == nil {
		return direct, target, nil
	}
	if !strings.HasSuffix(direct, ".log") {
		directWithExt := direct + ".log"
		if _, err := os.Stat(directWithExt); err == nil {
			return directWithExt, target + ".log", nil
		}
	}

	return filepath.Join(logsDir, target+".log"), target, nil
}

func selectLogTarget(logsDir string) (string, bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// 非 TTY 环境默认返回 upy 运行日志
		return "upy", true, nil
	}

	options := []ui.SelectOption{
		{Label: i18n.T("server 服务运行日志"), Hint: i18n.T("server.log (API/Webhook/请求访问)")},
		{Label: i18n.T("upy 自身执行日志"), Hint: i18n.T("upy.log (CLI 命令与执行流水)")},
	}

	// 扫描已有项目日志
	entries, err := os.ReadDir(logsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				projName := e.Name()
				options = append(options, ui.SelectOption{
					Label: fmt.Sprintf(i18n.T("项目 %s 部署日志"), projName),
					Hint:  fmt.Sprintf(i18n.T("%s/ 最新任务"), projName),
				})
			}
		}
	}

	idx, err := ui.Select(i18n.T("请选择要查看的日志"), options)
	if err != nil {
		return "", false, err
	}

	switch idx {
	case 0:
		return "server", true, nil
	case 1:
		return "upy", true, nil
	default:
		projName := strings.TrimPrefix(options[idx].Label, i18n.T("项目 "))
		projName = strings.TrimSuffix(projName, i18n.T(" 部署日志"))
		return projName, true, nil
	}
}

func listAllLogs(logsDir string) error {
	log.Step(i18n.T("日志目录: ") + log.Path(logsDir))
	fmt.Println()

	type fileItem struct {
		Name     string
		Category string
		Size     string
		ModTime  string
	}

	var items []fileItem

	// 1. 检查 server.log
	serverPath := server.ServerLogPath()
	if fi, err := os.Stat(serverPath); err == nil {
		items = append(items, fileItem{
			Name:     "server.log",
			Category: i18n.T("服务运行"),
			Size:     log.HumanSize(fi.Size()),
			ModTime:  formatRelTime(fi.ModTime()),
		})
	}

	// 2. 检查 upy.log
	upyPath := filepath.Join(logsDir, "upy.log")
	if fi, err := os.Stat(upyPath); err == nil {
		items = append(items, fileItem{
			Name:     "upy.log",
			Category: i18n.T("CLI 运行"),
			Size:     log.HumanSize(fi.Size()),
			ModTime:  formatRelTime(fi.ModTime()),
		})
	}

	// 3. 检查各个项目目录
	entries, err := os.ReadDir(logsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			projName := e.Name()
			subEntries, _ := os.ReadDir(filepath.Join(logsDir, projName))
			for _, se := range subEntries {
				if !se.IsDir() && strings.HasSuffix(se.Name(), ".log") {
					sfi, _ := se.Info()
					sz := "-"
					mt := "-"
					if sfi != nil {
						sz = log.HumanSize(sfi.Size())
						mt = formatRelTime(sfi.ModTime())
					}
					items = append(items, fileItem{
						Name:     filepath.Join(projName, se.Name()),
						Category: i18n.T("项目部署"),
						Size:     sz,
						ModTime:  mt,
					})
				}
			}
		}
	}

	if len(items) == 0 {
		log.Info(i18n.T("当前暂无任何日志文件。"))
		return nil
	}

	fmt.Printf("  %-30s  %-10s  %-10s  %s\n", i18n.T("日志文件"), i18n.T("类别"), i18n.T("大小"), i18n.T("最后修改"))
	fmt.Println("  " + strings.Repeat("─", 65))
	for _, it := range items {
		fmt.Printf("  %-30s  %-10s  %-10s  %s\n", it.Name, it.Category, it.Size, it.ModTime)
	}
	fmt.Println()
	return nil
}

func readLastNLines(filePath string, n int) ([]string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	fileSize := stat.Size()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, fileSize, nil
}

func followLog(filePath string, startOffset int64) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info(i18n.T("正在实时追踪日志... (按 Ctrl+C 退出)"))
	offset := startOffset

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			log.Info(i18n.T("已退出实时日志追踪"))
			return nil
		case <-ticker.C:
			fi, err := os.Stat(filePath)
			if err != nil {
				continue
			}
			curSize := fi.Size()
			if curSize < offset {
				// 文件被清空或截断
				offset = 0
			}
			if curSize > offset {
				f, err := os.Open(filePath)
				if err != nil {
					continue
				}
				_, _ = f.Seek(offset, io.SeekStart)
				scanner := bufio.NewScanner(f)
				for scanner.Scan() {
					fmt.Println(scanner.Text())
				}
				offset = curSize
				_ = f.Close()
			}
		}
	}
}

func formatRelTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return i18n.T("刚刚")
	case d < time.Hour:
		return fmt.Sprintf(i18n.T("%d分钟前"), int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf(i18n.T("%d小时前"), int(d.Hours()))
	default:
		return t.Format("2006-01-02")
	}
}
