// cleaner 项目数据清理器: 扫描与清理 bundles 缓存、旧版本目录、残留临时文件及完全重置
package cleaner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/cmdrun"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/deployer"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/projectlock"
	"github.com/AaronConlon/upy/internal/state"
)

var (
	// ErrUnknownActiveVersion 当存在版本目录但无法从状态或软链识别活跃版本时返回
	ErrUnknownActiveVersion = i18n.Error("无法确定项目活跃版本（state.json 与 current 软链均无效），拒绝清理历史版本；如需彻底清空请使用 --purge")
)

// ProjectStats 项目可清理资源统计信息
type ProjectStats struct {
	CurrentVersion       string   // 状态记录或解析出的首选当前版本
	SymlinkVersion       string   // current 符号链接指向的版本
	ProtectedVersions    []string // 受保护不被删除的全部活跃版本列表 (同时保护 state 与 symlink 指向版本)
	VersionConflict      bool     // state 与 current 软链版本是否存在分歧
	ActiveVersionUnknown bool     // 是否存在版本目录但无法判定任何活跃版本
	UnidentifiedReleases []string // 活跃版本未知时无法确认为旧版本的版本目录列表

	BundleFiles     int      // bundle 缓存文件数
	BundleBytes     int64    // bundle 缓存大小 (字节)
	OldReleases     []string // 可清理的非活跃旧版本列表
	OldReleaseBytes int64    // 旧版本占用大小 (字节)
	TmpDirs         []string // 残留临时目录列表
	TmpBytes        int64    // 残留临时目录大小 (字节)
}

// TotalReleasableBytes 常规瘦身可释放的总字节数 (bundles + 旧版本 + 残留临时目录)
func (s *ProjectStats) TotalReleasableBytes() int64 {
	return s.BundleBytes + s.OldReleaseBytes + s.TmpBytes
}

// HasReleasable 是否有任何可清理的数据
func (s *ProjectStats) HasReleasable() bool {
	return s.BundleFiles > 0 || len(s.OldReleases) > 0 || len(s.TmpDirs) > 0
}

// AnalyzeProject 分析项目目录下的各类数据占用
func AnalyzeProject(root string) (*ProjectStats, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	stats := &ProjectStats{}

	// 1. 获取并核对活跃版本 (CLEAN-001: 分别解析 state.json 与 current 软链; SYS-004: 保护 PreviousVersion)
	var protected []string
	st, err := state.Read(absRoot)
	if err == nil && st != nil {
		if strings.TrimSpace(st.CurrentVersion) != "" {
			cur := strings.TrimSpace(st.CurrentVersion)
			stats.CurrentVersion = cur
			protected = append(protected, cur)
		}
		if strings.TrimSpace(st.PreviousVersion) != "" {
			prev := strings.TrimSpace(st.PreviousVersion)
			if !slices.Contains(protected, prev) {
				protected = append(protected, prev)
			}
		}
	}

	symlinkVer := resolveCurrentVersionFromSymlink(absRoot)
	if symlinkVer != "" {
		stats.SymlinkVersion = symlinkVer
		if !slices.Contains(protected, symlinkVer) {
			protected = append(protected, symlinkVer)
		}
		if stats.CurrentVersion == "" {
			stats.CurrentVersion = symlinkVer
		}
	}

	// SYS-022: 检查系统服务 Unit 文件当前实际指向的版本（防止部署被强杀中断后，Unit 指向新版本而 state 仍为旧版本导致误删）
	unitVer := resolveVersionFromUnit(absRoot, st)
	if unitVer != "" && !slices.Contains(protected, unitVer) {
		protected = append(protected, unitVer)
	}

	if stats.CurrentVersion != "" && stats.SymlinkVersion != "" && stats.CurrentVersion != stats.SymlinkVersion {
		stats.VersionConflict = true
	}
	stats.ProtectedVersions = protected

	// 2. 统计 bundles/
	bundlesDir := filepath.Join(absRoot, "bundles")
	if entries, err := os.ReadDir(bundlesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
				if info, err := e.Info(); err == nil {
					stats.BundleFiles++
					stats.BundleBytes += info.Size()
				}
			}
		}
	}

	// 3. 统计 releases/
	releasesDir := filepath.Join(absRoot, "releases")
	var normalReleases []string
	if entries, err := os.ReadDir(releasesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dirName := e.Name()
			dirPath := filepath.Join(releasesDir, dirName)
			if strings.HasPrefix(dirName, ".tmp-deploy-") {
				stats.TmpDirs = append(stats.TmpDirs, dirName)
				size, _ := dirSize(dirPath)
				stats.TmpBytes += size
				continue
			}
			normalReleases = append(normalReleases, dirName)
		}
	}

	// 4. CLEAN-002: 活跃版本未知保护
	// 如果存在普通版本目录，但无法从 state.json 或 current 软链识别任何受保护的活跃版本，
	// 拒绝将任何版本加入 OldReleases，避免普通清理误删全部 release
	if len(normalReleases) > 0 && len(protected) == 0 {
		stats.ActiveVersionUnknown = true
		stats.UnidentifiedReleases = normalReleases
	} else {
		for _, dirName := range normalReleases {
			// 严格排除所有受保护版本 (包含 state 记录与 current 软链指向)
			if slices.Contains(protected, dirName) {
				continue
			}
			stats.OldReleases = append(stats.OldReleases, dirName)
			dirPath := filepath.Join(releasesDir, dirName)
			size, _ := dirSize(dirPath)
			stats.OldReleaseBytes += size
		}
	}

	return stats, nil
}

// CleanBundles 清理 bundles/ 目录下的所有 zip 压缩包
func CleanBundles(root string) (int64, int, error) {
	lock, err := projectlock.Acquire(root, i18n.T("清理 Bundle 缓存"))
	if err != nil {
		return 0, 0, err
	}
	defer lock.Release()

	bundlesDir := filepath.Join(root, "bundles")
	entries, err := os.ReadDir(bundlesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	var freedBytes int64
	var count int
	var errs []error
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		filePath := filepath.Join(bundlesDir, e.Name())
		info, err := e.Info()
		if err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("读取 bundle 文件信息失败 %s: %w"), e.Name(), err))
			continue
		}
		if removeErr := os.Remove(filePath); removeErr != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除 bundle 缓存失败 %s: %w"), e.Name(), removeErr))
		} else {
			freedBytes += info.Size()
			count++
		}
	}
	return freedBytes, count, errors.Join(errs...)
}

// PruneBundles 按保留策略清理 bundles/ 目录下的 zip 压缩包，保留修改时间最新的 keepCount 个 (0~100)
// 返回释放的字节数、已删除的文件名列表、错误
func PruneBundles(root string, keepCount int) (int64, []string, error) {
	if keepCount < 0 {
		keepCount = 0
	}
	lock, err := projectlock.Acquire(root, i18n.T("清理 Bundle 缓存"))
	if err != nil {
		return 0, nil, err
	}
	defer lock.Release()

	bundlesDir := filepath.Join(root, "bundles")
	entries, err := os.ReadDir(bundlesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, err
	}

	type bundleItem struct {
		name    string
		path    string
		size    int64
		modTime time.Time
	}

	var items []bundleItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, bundleItem{
			name:    e.Name(),
			path:    filepath.Join(bundlesDir, e.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
	}

	if len(items) <= keepCount {
		return 0, nil, nil
	}

	// 按修改时间降序排序 (最新的排前面)；如果修改时间相同，按文件名降序排序
	sort.Slice(items, func(i, j int) bool {
		if items[i].modTime.Equal(items[j].modTime) {
			return items[i].name > items[j].name
		}
		return items[i].modTime.After(items[j].modTime)
	})

	var freedBytes int64
	var deleted []string
	var errs []error

	for i := keepCount; i < len(items); i++ {
		item := items[i]
		if err := os.Remove(item.path); err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除 bundle 缓存失败 %s: %w"), item.name, err))
		} else {
			freedBytes += item.size
			deleted = append(deleted, item.name)
		}
	}

	return freedBytes, deleted, errors.Join(errs...)
}

// CleanOldReleases 清理 releases/ 下所有非活跃历史版本目录
func CleanOldReleases(root string) (int64, []string, error) {
	lock, err := projectlock.Acquire(root, i18n.T("清理历史旧版本"))
	if err != nil {
		return 0, nil, err
	}
	defer lock.Release()

	stats, err := AnalyzeProject(root)
	if err != nil {
		return 0, nil, err
	}
	// CLEAN-002: 活跃版本未知时禁止清理
	if stats.ActiveVersionUnknown {
		return 0, nil, ErrUnknownActiveVersion
	}

	releasesDir := filepath.Join(root, "releases")
	var freedBytes int64
	var cleaned []string
	var errs []error

	for _, v := range stats.OldReleases {
		targetDir := filepath.Join(releasesDir, v)
		size, _ := dirSize(targetDir)
		if err := os.RemoveAll(targetDir); err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除历史版本 %s 失败: %w"), v, err))
		} else {
			freedBytes += size
			cleaned = append(cleaned, v)
		}
	}

	return freedBytes, cleaned, errors.Join(errs...)
}

// CleanTmpDeployDirs 清理 releases/ 下残留的 .tmp-deploy-* 临时目录
func CleanTmpDeployDirs(root string) (int64, int, error) {
	lock, err := projectlock.Acquire(root, i18n.T("清理残留临时目录"))
	if err != nil {
		return 0, 0, err
	}
	defer lock.Release()

	releasesDir := filepath.Join(root, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	var freedBytes int64
	var count int
	var errs []error
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), ".tmp-deploy-") {
			continue
		}
		dirPath := filepath.Join(releasesDir, e.Name())
		size, _ := dirSize(dirPath)
		if err := os.RemoveAll(dirPath); err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除临时目录 %s 失败: %w"), e.Name(), err))
		} else {
			freedBytes += size
			count++
		}
	}
	return freedBytes, count, errors.Join(errs...)
}

// PurgeProject 全量清理项目部署产物 (删除 releases/、bundles/、current 软链、state.json)
func PurgeProject(root string) (int64, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return 0, err
	}

	lock, err := projectlock.Acquire(absRoot, i18n.T("彻底清空项目部署产物"))
	if err != nil {
		return 0, err
	}
	defer lock.Release()

	var totalFreed int64
	var errs []error

	// 0. 卸载可能纳管的 systemd unit (SYS-004)
	unitDir := os.Getenv("UPY_SYSTEMD_DIR")
	if unitDir == "" {
		unitDir = "/etc/systemd/system"
	}
	unitName := ""
	if st, err := state.Read(absRoot); err == nil && st != nil && st.UnitName != "" {
		unitName = st.UnitName
	}
	if unitName == "" {
		if cfg, err := config.ReadProjectConfig(absRoot); err == nil && cfg != nil {
			if cfg.Service != nil && cfg.Service.Unit != "" {
				unitName = cfg.Service.Unit
			} else if cfg.Name != "" {
				unitName = fmt.Sprintf("upy-%s.service", deployer.Slug(cfg.Name))
			}
		}
	}
	if unitName != "" {
		unitPath := filepath.Join(unitDir, unitName)
		if content, err := os.ReadFile(unitPath); err == nil {
			text := string(content)
			if strings.Contains(text, "managed by upy") || strings.Contains(text, absRoot) {
				_ = cmdrun.Run("systemctl", []string{"stop", unitName}, cmdrun.Options{AllowNonZero: true})
				_ = cmdrun.Run("systemctl", []string{"disable", unitName}, cmdrun.Options{AllowNonZero: true})
				if err := os.Remove(unitPath); err != nil {
					errs = append(errs, fmt.Errorf(i18n.T("删除 unit 文件 %s 失败: %w"), unitPath, err))
				}
				_ = cmdrun.Run("systemctl", []string{"daemon-reload"}, cmdrun.Options{})
				_ = cmdrun.Run("systemctl", []string{"reset-failed", unitName}, cmdrun.Options{AllowNonZero: true})
			}
		}
	}
	_ = os.Remove(filepath.Join(absRoot, ".deploy.unit.bak"))

	// 1. releases/
	releasesDir := filepath.Join(absRoot, "releases")
	if size, _ := dirSize(releasesDir); size > 0 {
		if err := os.RemoveAll(releasesDir); err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除 releases 目录失败: %w"), err))
		} else {
			totalFreed += size
		}
	}

	// 2. bundles/
	bundlesDir := filepath.Join(absRoot, "bundles")
	if size, _ := dirSize(bundlesDir); size > 0 {
		if err := os.RemoveAll(bundlesDir); err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除 bundles 目录失败: %w"), err))
		} else {
			totalFreed += size
		}
	}

	// 3. current 软链
	currentLink := filepath.Join(absRoot, "current")
	if _, err := os.Lstat(currentLink); err == nil {
		if err := os.Remove(currentLink); err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除 current 软链失败: %w"), err))
		}
	}

	// 4. state.json
	stateFile := filepath.Join(absRoot, "state.json")
	if info, err := os.Stat(stateFile); err == nil {
		size := info.Size()
		if err := os.Remove(stateFile); err != nil {
			errs = append(errs, fmt.Errorf(i18n.T("删除 state.json 失败: %w"), err))
		} else {
			totalFreed += size
		}
	}

	return totalFreed, errors.Join(errs...)
}

// FormatBytes 将字节数格式化为人类可读的字符串 (如 45.2 MB)
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

// resolveCurrentVersionFromSymlink 从 current 符号链接解析出规范的当前版本名。
// 基于规范化后的目标路径与项目 releases/ 目录层级关系，杜绝单纯的字符串截取
func resolveCurrentVersionFromSymlink(root string) string {
	linkPath := filepath.Join(root, "current")
	target, err := os.Readlink(linkPath)
	if err != nil {
		return ""
	}

	absTarget := target
	if !filepath.IsAbs(absTarget) {
		absTarget = filepath.Join(root, target)
	}
	absTarget = filepath.Clean(absTarget)
	absReleases := filepath.Clean(filepath.Join(root, "releases"))

	rel, err := filepath.Rel(absReleases, absTarget)
	if err != nil {
		return ""
	}

	relSlash := filepath.ToSlash(rel)
	// 如果不在 releases 目录下，或者为上一级引用，则不认为是有效版本目录
	if strings.HasPrefix(relSlash, "../") || relSlash == ".." || relSlash == "." {
		return ""
	}

	parts := strings.Split(relSlash, "/")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

// dirSize 计算指定目录递归大小
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法读取的文件
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// resolveVersionFromUnit 检查系统服务 Unit 文件当前实际指向的版本（防止部署被强杀中断后，Unit 指向新版本而 state 仍为旧版本导致误删 - SYS-22）
func resolveVersionFromUnit(root string, st *state.State) string {
	unitDir := os.Getenv("UPY_SYSTEMD_DIR")
	if unitDir == "" {
		unitDir = "/etc/systemd/system"
	}
	unitName := ""
	if st != nil && st.UnitName != "" {
		unitName = st.UnitName
	}
	if unitName == "" {
		if cfg, err := config.ReadProjectConfig(root); err == nil && cfg != nil {
			if cfg.Service != nil && cfg.Service.Unit != "" {
				unitName = cfg.Service.Unit
			} else if cfg.Name != "" {
				unitName = fmt.Sprintf("upy-%s.service", deployer.Slug(cfg.Name))
			}
		}
	}
	if unitName == "" {
		return ""
	}

	unitPath := filepath.Join(unitDir, unitName)
	content, err := os.ReadFile(unitPath)
	if err != nil {
		return ""
	}
	text := string(content)

	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	// 校验 Unit 归属
	if !strings.Contains(text, "managed by upy") && !strings.Contains(text, absRoot) {
		return ""
	}

	releasesPrefix := filepath.Join(absRoot, "releases")
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "WorkingDirectory=") {
			wd := strings.TrimSpace(strings.TrimPrefix(trimmed, "WorkingDirectory="))
			if rel, err := filepath.Rel(releasesPrefix, wd); err == nil {
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if len(parts) > 0 && parts[0] != "" && parts[0] != "." && !strings.HasPrefix(parts[0], "..") {
					return parts[0]
				}
			}
		}
		if strings.HasPrefix(trimmed, "ExecStart=") {
			exec := strings.TrimSpace(strings.TrimPrefix(trimmed, "ExecStart="))
			fields := strings.Fields(exec)
			if len(fields) > 0 {
				target := strings.Trim(fields[0], `"`)
				if rel, err := filepath.Rel(releasesPrefix, target); err == nil {
					parts := strings.Split(filepath.ToSlash(rel), "/")
					if len(parts) > 0 && parts[0] != "" && parts[0] != "." && !strings.HasPrefix(parts[0], "..") {
						return parts[0]
					}
				}
			}
		}
	}
	return ""
}
