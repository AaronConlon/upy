// Package backup 实现了约定优先的项目数据备份引擎
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/projectlock"
	"github.com/AaronConlon/upy/internal/state"
)

// DefaultBackupDir 默认备份产物目录名
const DefaultBackupDir = "backups"

// DefaultTimeout 默认备份超时时间
const DefaultTimeout = 15 * time.Minute

// DefaultRetention 默认自动保留的最新备份份数
const DefaultRetention = 7

var (
	// ErrNoScript 未找到可执行备份脚本
	ErrNoScript = errors.New("未找到备份脚本（请在项目根目录下创建 backup.sh、scripts/backup.sh，或在 deploy.yaml 中配置 backup.script）")
)

// BackupFile 描述一个备份产物文件
type BackupFile struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
}

// FindScript 在项目根目录下探测备份脚本
func FindScript(root, customScript string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(customScript) != "" {
		p := customScript
		if !filepath.IsAbs(p) {
			p = filepath.Join(absRoot, p)
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("指定的自定义备份脚本不存在: %s", customScript)
	}

	candidates := []string{
		filepath.Join(absRoot, "backup.sh"),
		filepath.Join(absRoot, "scripts", "backup.sh"),
		filepath.Join(absRoot, "bin", "backup.sh"),
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}

	return "", ErrNoScript
}

// ScanBackups 扫描备份目录下的普通文件，按修改时间降序排序（最新在前）
func ScanBackups(backupDir string) ([]BackupFile, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}

	var files []BackupFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 忽略隐藏文件与临时文件
		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, BackupFile{
			Name:    name,
			Path:    filepath.Join(backupDir, name),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// PruneBackups 轮转清理旧备份，保留最近的 keepCount 份文件
func PruneBackups(backupDir string, keepCount int) ([]string, error) {
	if keepCount <= 0 {
		return nil, nil
	}

	files, err := ScanBackups(backupDir)
	if err != nil {
		return nil, err
	}

	if len(files) <= keepCount {
		return nil, nil
	}

	toDelete := files[keepCount:]
	var deleted []string

	for _, f := range toDelete {
		if err := os.Remove(f.Path); err == nil {
			deleted = append(deleted, f.Name)
		} else if !os.IsNotExist(err) {
			return deleted, fmt.Errorf("删除陈旧备份文件 %s 失败: %w", f.Name, err)
		}
	}

	return deleted, nil
}

// Options 执行备份的选项
type Options struct {
	Root         string
	CustomScript string
	BackupDir    string
	Retention    int
	Timeout      time.Duration
	Stdout       io.Writer
	Stderr       io.Writer
}

// Result 备份执行结果
type Result struct {
	ProjectName  string
	ScriptPath   string
	BackupDir    string
	CreatedFiles []BackupFile
	PrunedFiles  []string
	TotalBytes   int64
	Duration     time.Duration
}

// Run 执行一次项目备份
func Run(ctx context.Context, opts Options) (*Result, error) {
	absRoot, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("无效的项目路径: %w", err)
	}

	// 1. 获取跨进程排他锁
	lock, err := projectlock.Acquire(absRoot, "备份中")
	if err != nil {
		return nil, fmt.Errorf("获取项目锁失败: %w", err)
	}
	defer lock.Release()

	// 2. 读取项目与版本上下文
	projectName := filepath.Base(absRoot)
	deployType := ""
	currentVersion := ""

	// 尝试解析 deploy.yaml 获取名称及配置
	cfg, cfgErr := config.ReadProjectConfig(absRoot)
	if cfgErr == nil && cfg.Name != "" {
		projectName = cfg.Name
		if cfg.Backup != nil {
			if opts.CustomScript == "" && cfg.Backup.Script != "" {
				opts.CustomScript = cfg.Backup.Script
			}
			if opts.BackupDir == "" && cfg.Backup.Dir != "" {
				opts.BackupDir = cfg.Backup.Dir
			}
			if opts.Retention <= 0 && cfg.Backup.Retention > 0 {
				opts.Retention = cfg.Backup.Retention
			}
			if opts.Timeout <= 0 && cfg.Backup.Timeout != "" {
				if d, parseErr := time.ParseDuration(cfg.Backup.Timeout); parseErr == nil {
					opts.Timeout = d
				}
			}
		}
	}

	// 尝试读取当前运行状态与版本
	st, stErr := state.Read(absRoot)
	if stErr == nil && st != nil {
		currentVersion = st.CurrentVersion
		deployType = st.Type
	}

	// 3. 确定备份脚本路径
	scriptPath, err := FindScript(absRoot, opts.CustomScript)
	if err != nil {
		return nil, err
	}

	// 4. 确定并创建备份产物目录
	backupDir := opts.BackupDir
	if strings.TrimSpace(backupDir) == "" {
		backupDir = DefaultBackupDir
	}
	if !filepath.IsAbs(backupDir) {
		backupDir = filepath.Join(absRoot, backupDir)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建备份输出目录失败: %w", err)
	}

	// 5. 记录执行前的备份目录快照
	beforeFiles, _ := ScanBackups(backupDir)
	beforeMap := make(map[string]int64, len(beforeFiles))
	for _, f := range beforeFiles {
		beforeMap[f.Name] = f.Size
	}

	// 6. 确保脚本具有执行权限 (非 Windows)
	_ = os.Chmod(scriptPath, 0o755)

	// 7. 组装超时与子进程环境变量
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	timestamp := time.Now().Format("20060102-150405")
	env := os.Environ()
	env = append(env,
		fmt.Sprintf("UPY_PROJECT_NAME=%s", projectName),
		fmt.Sprintf("UPY_PROJECT_ROOT=%s", absRoot),
		fmt.Sprintf("UPY_BACKUP_DIR=%s", backupDir),
		fmt.Sprintf("UPY_CURRENT_VERSION=%s", currentVersion),
		fmt.Sprintf("UPY_TIMESTAMP=%s", timestamp),
		fmt.Sprintf("UPY_DEPLOY_TYPE=%s", deployType),
	)

	cmd := exec.CommandContext(runCtx, scriptPath)
	cmd.Dir = absRoot
	cmd.Env = env
	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)

	if runErr != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("备份脚本执行超时 (超过 %s)", timeout)
		}
		return nil, fmt.Errorf("备份脚本执行失败: %w", runErr)
	}

	// 8. 扫描生成的新增或更新文件
	afterFiles, _ := ScanBackups(backupDir)
	var created []BackupFile
	var totalBytes int64

	for _, f := range afterFiles {
		oldSize, existed := beforeMap[f.Name]
		if !existed || f.Size != oldSize {
			created = append(created, f)
			totalBytes += f.Size
		}
	}

	// 9. 自动轮转旧备份
	retention := opts.Retention
	if retention <= 0 && cfg != nil && cfg.Backup != nil && cfg.Backup.Retention > 0 {
		retention = cfg.Backup.Retention
	} else if retention <= 0 {
		retention = DefaultRetention
	}

	var pruned []string
	if retention > 0 {
		pruned, _ = PruneBackups(backupDir, retention)
	}

	return &Result{
		ProjectName:  projectName,
		ScriptPath:   scriptPath,
		BackupDir:    backupDir,
		CreatedFiles: created,
		PrunedFiles:  pruned,
		TotalBytes:   totalBytes,
		Duration:     duration,
	}, nil
}
