package cleaner

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/AaronConlon/upy/internal/projectlock"
	"github.com/AaronConlon/upy/internal/state"
)

func setupTestProject(t *testing.T) string {
	root := t.TempDir()

	// 1. bundles/
	bundlesDir := filepath.Join(root, "bundles")
	if err := os.MkdirAll(bundlesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(bundlesDir, "app-v1.0.0.zip"), []byte(stringsRepeat("A", 1024)), 0o644) // 1KB
	_ = os.WriteFile(filepath.Join(bundlesDir, "app-v1.0.1.zip"), []byte(stringsRepeat("B", 2048)), 0o644) // 2KB
	_ = os.WriteFile(filepath.Join(bundlesDir, "notes.txt"), []byte("not-a-zip"), 0o644)

	// 2. releases/
	releasesDir := filepath.Join(root, "releases")
	// 旧版本 v1.0.0
	v1Dir := filepath.Join(releasesDir, "v1.0.0", "dist")
	_ = os.MkdirAll(v1Dir, 0o755)
	_ = os.WriteFile(filepath.Join(v1Dir, "index.html"), []byte("v1.0.0"), 0o644)

	// 当前版本 v1.0.1 (受保护)
	v2Dir := filepath.Join(releasesDir, "v1.0.1", "dist")
	_ = os.MkdirAll(v2Dir, 0o755)
	_ = os.WriteFile(filepath.Join(v2Dir, "index.html"), []byte("v1.0.1"), 0o644)

	// 临时残留目录 .tmp-deploy-123
	tmpDir := filepath.Join(releasesDir, ".tmp-deploy-123")
	_ = os.MkdirAll(tmpDir, 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "temp.bin"), []byte("temp"), 0o644)

	// 3. current 软链
	currentLink := filepath.Join(root, "current")
	_ = os.Symlink(filepath.Join(releasesDir, "v1.0.1", "dist"), currentLink)

	// 4. state.json
	_ = state.WriteAtomic(root, &state.State{CurrentVersion: "v1.0.1"})

	return root
}

func stringsRepeat(s string, count int) string {
	var b []byte
	for i := 0; i < count; i++ {
		b = append(b, s...)
	}
	return string(b)
}

func TestAnalyzeProject(t *testing.T) {
	root := setupTestProject(t)

	stats, err := AnalyzeProject(root)
	if err != nil {
		t.Fatalf("AnalyzeProject 失败: %v", err)
	}

	if stats.CurrentVersion != "v1.0.1" {
		t.Errorf("期望 CurrentVersion=v1.0.1, 得到: %s", stats.CurrentVersion)
	}
	if stats.BundleFiles != 2 {
		t.Errorf("期望 BundleFiles=2, 得到: %d", stats.BundleFiles)
	}
	if stats.BundleBytes != 3072 {
		t.Errorf("期望 BundleBytes=3072, 得到: %d", stats.BundleBytes)
	}
	if len(stats.OldReleases) != 1 || stats.OldReleases[0] != "v1.0.0" {
		t.Errorf("期望 OldReleases=[v1.0.0], 得到: %v", stats.OldReleases)
	}
	if len(stats.TmpDirs) != 1 || stats.TmpDirs[0] != ".tmp-deploy-123" {
		t.Errorf("期望 TmpDirs=[.tmp-deploy-123], 得到: %v", stats.TmpDirs)
	}
	if !stats.HasReleasable() {
		t.Errorf("期望 HasReleasable=true")
	}
}

func TestCleanBundles(t *testing.T) {
	root := setupTestProject(t)

	freed, count, err := CleanBundles(root)
	if err != nil {
		t.Fatalf("CleanBundles 失败: %v", err)
	}
	if count != 2 {
		t.Errorf("期望清理 2 个文件, 得到: %d", count)
	}
	if freed != 3072 {
		t.Errorf("期望释放 3072 字节, 得到: %d", freed)
	}

	// 确认非 zip 文件保留
	txtPath := filepath.Join(root, "bundles", "notes.txt")
	if _, err := os.Stat(txtPath); err != nil {
		t.Errorf("notes.txt 不应被删除")
	}
}

func TestCleanOldReleasesProtectsCurrent(t *testing.T) {
	root := setupTestProject(t)

	freed, cleaned, err := CleanOldReleases(root)
	if err != nil {
		t.Fatalf("CleanOldReleases 失败: %v", err)
	}
	if len(cleaned) != 1 || cleaned[0] != "v1.0.0" {
		t.Errorf("期望清理 v1.0.0, 得到: %v", cleaned)
	}
	if freed == 0 {
		t.Errorf("期望释放空间 > 0")
	}

	// 核心断言: 当前生效版本 v1.0.1 绝不能被删除
	currentDir := filepath.Join(root, "releases", "v1.0.1")
	if _, err := os.Stat(currentDir); err != nil {
		t.Fatalf("当前活跃版本 v1.0.1 被错误删除了！")
	}

	// 确认旧版本已删除
	oldDir := filepath.Join(root, "releases", "v1.0.0")
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("旧版本 v1.0.0 应该已被删除")
	}
}

// TestClean001_ConflictProtection: state 记录 v1，但 current 软链指向 v2，验证两者均不被删除
func TestClean001_ConflictProtection(t *testing.T) {
	root := t.TempDir()
	releasesDir := filepath.Join(root, "releases")
	_ = os.MkdirAll(filepath.Join(releasesDir, "v0", "dist"), 0o755)
	_ = os.MkdirAll(filepath.Join(releasesDir, "v1", "dist"), 0o755)
	_ = os.MkdirAll(filepath.Join(releasesDir, "v2", "dist"), 0o755)

	// state 记录为 v1
	_ = state.WriteAtomic(root, &state.State{CurrentVersion: "v1"})
	// current 指向 v2/dist
	_ = os.Symlink(filepath.Join(releasesDir, "v2", "dist"), filepath.Join(root, "current"))

	stats, err := AnalyzeProject(root)
	if err != nil {
		t.Fatalf("AnalyzeProject 失败: %v", err)
	}

	if !stats.VersionConflict {
		t.Errorf("期望检测到版本分歧 VersionConflict=true")
	}
	// OldReleases 应该只有 v0，v1 和 v2 必须都被保护
	if len(stats.OldReleases) != 1 || stats.OldReleases[0] != "v0" {
		t.Fatalf("期望 OldReleases 仅包含 [v0], 实际得到: %v", stats.OldReleases)
	}

	freed, cleaned, err := CleanOldReleases(root)
	if err != nil {
		t.Fatalf("CleanOldReleases 失败: %v", err)
	}
	if len(cleaned) != 1 || cleaned[0] != "v0" {
		t.Errorf("期望仅清理 [v0], 实际得到: %v (freed=%d)", cleaned, freed)
	}

	// 断言 v1 和 v2 都保留
	if _, err := os.Stat(filepath.Join(releasesDir, "v1")); err != nil {
		t.Errorf("v1 (state版本) 不应被删除")
	}
	if _, err := os.Stat(filepath.Join(releasesDir, "v2")); err != nil {
		t.Errorf("v2 (current软链版本) 不应被删除")
	}
}

// TestClean002_UnknownActiveVersionProtection: 无法确定活跃版本时拒绝清理全部 release
func TestClean002_UnknownActiveVersionProtection(t *testing.T) {
	root := t.TempDir()
	releasesDir := filepath.Join(root, "releases")
	_ = os.MkdirAll(filepath.Join(releasesDir, "v1"), 0o755)
	_ = os.MkdirAll(filepath.Join(releasesDir, "v2"), 0o755)
	_ = os.MkdirAll(filepath.Join(releasesDir, "v3"), 0o755)

	// 无 state.json，且无 current 软链
	stats, err := AnalyzeProject(root)
	if err != nil {
		t.Fatalf("AnalyzeProject 失败: %v", err)
	}
	if !stats.ActiveVersionUnknown {
		t.Errorf("期望 ActiveVersionUnknown=true")
	}
	if len(stats.OldReleases) != 0 {
		t.Fatalf("活跃版本未知时，OldReleases 必须为空，防止误删全部 release！实际得到: %v", stats.OldReleases)
	}

	// 执行 CleanOldReleases 必须返回 ErrUnknownActiveVersion 错误，拒绝执行
	_, _, err = CleanOldReleases(root)
	if err == nil || !errors.Is(err, ErrUnknownActiveVersion) {
		t.Fatalf("期望返回 ErrUnknownActiveVersion, 得到: %v", err)
	}

	// 断言所有版本目录均未被删除
	for _, ver := range []string{"v1", "v2", "v3"} {
		if _, err := os.Stat(filepath.Join(releasesDir, ver)); err != nil {
			t.Errorf("版本 %s 不应被删除", ver)
		}
	}
}

// TestClean003_ConcurrentLocking: 模拟部署持有锁时，清理操作被明确拒绝
func TestClean003_ConcurrentLocking(t *testing.T) {
	root := t.TempDir()
	releasesDir := filepath.Join(root, "releases")
	_ = os.MkdirAll(filepath.Join(releasesDir, "v1"), 0o755)
	_ = state.WriteAtomic(root, &state.State{CurrentVersion: "v1"})

	// 模拟部署进程持有锁
	lockPath := filepath.Join(root, projectlock.LockFileName)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// 使用底层文件锁锁定
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("action: 部署中\npid: 99999\n")
	_ = f.Sync()

	// 此时尝试执行清理，由于无法获取锁，应返回明确错误
	_, _, err = CleanBundles(root)
	if err == nil {
		t.Fatal("期望 CleanBundles 失败（由于锁被占用），但成功了")
	}

	_, _, err = CleanOldReleases(root)
	if err == nil {
		t.Fatal("期望 CleanOldReleases 失败（由于锁被占用），但成功了")
	}
}

// TestClean004_DeletionErrorPropagation: 删除失败时返回错误，且只累计成功删除的文件大小
func TestClean004_DeletionErrorPropagation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 用户会无视目录写权限，跳过只读目录删除测试")
	}

	root := t.TempDir()
	bundlesDir := filepath.Join(root, "bundles")
	_ = os.MkdirAll(bundlesDir, 0o755)

	zip1 := filepath.Join(bundlesDir, "app-1.zip")
	_ = os.WriteFile(zip1, []byte("test-data-1"), 0o644)

	// 将 bundles 目录设为只读 (0555)，使其内部文件无法删除
	_ = os.Chmod(bundlesDir, 0o555)
	defer func() {
		_ = os.Chmod(bundlesDir, 0o755)
	}()

	freed, count, err := CleanBundles(root)
	if err == nil {
		t.Fatal("在只读目录下删除文件，期望返回错误，但 err 为 nil")
	}
	if freed != 0 {
		t.Fatalf("删除失败的文件不应累计入 freedBytes，期望 0，得到 %d", freed)
	}
	if count != 0 {
		t.Fatalf("删除失败的文件不应累计入 count，期望 0，得到 %d", count)
	}
}

func TestCleanTmpDeployDirs(t *testing.T) {
	root := setupTestProject(t)

	freed, count, err := CleanTmpDeployDirs(root)
	if err != nil {
		t.Fatalf("CleanTmpDeployDirs 失败: %v", err)
	}
	if count != 1 {
		t.Errorf("期望清理 1 个临时目录, 得到: %d", count)
	}
	if freed == 0 {
		t.Errorf("期望释放空间 > 0")
	}

	tmpPath := filepath.Join(root, "releases", ".tmp-deploy-123")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("临时目录 .tmp-deploy-123 应被删除")
	}
}

func TestPurgeProject(t *testing.T) {
	root := setupTestProject(t)

	freed, err := PurgeProject(root)
	if err != nil {
		t.Fatalf("PurgeProject 失败: %v", err)
	}
	if freed == 0 {
		t.Errorf("期望释放空间 > 0")
	}

	if _, err := os.Stat(filepath.Join(root, "releases")); !os.IsNotExist(err) {
		t.Errorf("releases 目录应被彻底删除")
	}
	if _, err := os.Stat(filepath.Join(root, "bundles")); !os.IsNotExist(err) {
		t.Errorf("bundles 目录应被彻底删除")
	}
	if _, err := os.Stat(filepath.Join(root, "current")); !os.IsNotExist(err) {
		t.Errorf("current 软链应被彻底删除")
	}
	if _, err := os.Stat(filepath.Join(root, "state.json")); !os.IsNotExist(err) {
		t.Errorf("state.json 应被彻底删除")
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024 * 45, "45.0 MB"},
		{1024 * 1024 * 1024 * 2, "2.0 GB"},
	}
	for _, c := range cases {
		got := FormatBytes(c.bytes)
		if got != c.want {
			t.Errorf("FormatBytes(%d) = %s, want %s", c.bytes, got, c.want)
		}
	}
}
