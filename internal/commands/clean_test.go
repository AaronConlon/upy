package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/state"
)

func setupCleanTestProject(t *testing.T, name string) string {
	root := t.TempDir()

	// deploy.yaml
	_ = os.WriteFile(filepath.Join(root, "deploy.yaml"), []byte("name: "+name+"\nrelease:\n  provider: github\n  repository: test/"+name+"\n"), 0o644)

	// bundles/
	bundlesDir := filepath.Join(root, "bundles")
	_ = os.MkdirAll(bundlesDir, 0o755)
	_ = os.WriteFile(filepath.Join(bundlesDir, "bundle-v1.zip"), []byte("bundle-v1-bytes"), 0o644)
	_ = os.WriteFile(filepath.Join(bundlesDir, "bundle-v2.zip"), []byte("bundle-v2-bytes"), 0o644)

	// releases/
	releasesDir := filepath.Join(root, "releases")
	v1Dir := filepath.Join(releasesDir, "v1", "dist")
	_ = os.MkdirAll(v1Dir, 0o755)
	_ = os.WriteFile(filepath.Join(v1Dir, "index.html"), []byte("v1"), 0o644)

	v2Dir := filepath.Join(releasesDir, "v2", "dist")
	_ = os.MkdirAll(v2Dir, 0o755)
	_ = os.WriteFile(filepath.Join(v2Dir, "index.html"), []byte("v2"), 0o644)

	tmpDir := filepath.Join(releasesDir, ".tmp-deploy-999")
	_ = os.MkdirAll(tmpDir, 0o755)
	_ = os.WriteFile(filepath.Join(tmpDir, "tmp.bin"), []byte("tmp"), 0o644)

	// current -> releases/v2/dist
	_ = os.Symlink(filepath.Join(releasesDir, "v2", "dist"), filepath.Join(root, "current"))

	// state.json
	_ = state.WriteAtomic(root, &state.State{CurrentVersion: "v2"})

	return root
}

func TestCleanWithFlagsBundles(t *testing.T) {
	root := setupCleanTestProject(t, "proj-bundles")

	err := Clean(CleanArgs{
		Root:    root,
		Bundles: true,
		Yes:     true,
	})
	if err != nil {
		t.Fatalf("Clean bundles 失败: %v", err)
	}

	entries, _ := os.ReadDir(filepath.Join(root, "bundles"))
	if len(entries) != 0 {
		t.Errorf("bundles 目录应为空, 现有: %d 个文件", len(entries))
	}

	// 确认 release 目录保留
	if _, err := os.Stat(filepath.Join(root, "releases", "v2")); err != nil {
		t.Errorf("当前版本 v2 不应被清理")
	}
}

func TestCleanWithFlagsOldReleases(t *testing.T) {
	root := setupCleanTestProject(t, "proj-releases")

	err := Clean(CleanArgs{
		Root:        root,
		OldReleases: true,
		Yes:         true,
	})
	if err != nil {
		t.Fatalf("Clean old-releases 失败: %v", err)
	}

	// 确认旧版本 v1 被删除
	if _, err := os.Stat(filepath.Join(root, "releases", "v1")); !os.IsNotExist(err) {
		t.Errorf("旧版本 v1 应被删除")
	}

	// 确认临时目录被删除
	if _, err := os.Stat(filepath.Join(root, "releases", ".tmp-deploy-999")); !os.IsNotExist(err) {
		t.Errorf("临时目录 .tmp-deploy-999 应被删除")
	}

	// 确认当前生效版本 v2 依然存在且安全
	if _, err := os.Stat(filepath.Join(root, "releases", "v2")); err != nil {
		t.Fatalf("当前活跃版本 v2 被错误删除了！")
	}
}

func TestCleanWithFlagsAll(t *testing.T) {
	root := setupCleanTestProject(t, "proj-all")

	err := Clean(CleanArgs{
		Root: root,
		All:  true,
		Yes:  true,
	})
	if err != nil {
		t.Fatalf("Clean all 失败: %v", err)
	}

	// bundles 清空
	bEntries, _ := os.ReadDir(filepath.Join(root, "bundles"))
	if len(bEntries) != 0 {
		t.Errorf("bundles 目录应为空")
	}

	// releases 仅剩 v2
	rEntries, _ := os.ReadDir(filepath.Join(root, "releases"))
	if len(rEntries) != 1 || rEntries[0].Name() != "v2" {
		t.Errorf("releases 应该只保留当前版本 v2, 得到: %v", rEntries)
	}
}

func TestCleanWithFlagsPurge(t *testing.T) {
	root := setupCleanTestProject(t, "proj-purge")

	err := Clean(CleanArgs{
		Root:  root,
		Purge: true,
		Yes:   true,
	})
	if err != nil {
		t.Fatalf("Clean purge 失败: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "releases")); !os.IsNotExist(err) {
		t.Errorf("releases 目录应被彻底清空")
	}
	if _, err := os.Stat(filepath.Join(root, "bundles")); !os.IsNotExist(err) {
		t.Errorf("bundles 目录应被彻底清空")
	}
	if _, err := os.Stat(filepath.Join(root, "current")); !os.IsNotExist(err) {
		t.Errorf("current 软链应被彻底清空")
	}
	// deploy.yaml 应该保留
	if _, err := os.Stat(filepath.Join(root, "deploy.yaml")); err != nil {
		t.Errorf("deploy.yaml 配置应保留")
	}
}

func TestCleanWithUnregister(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("UPY_CONFIG", path)

	root := setupCleanTestProject(t, "proj-unreg")
	_ = config.SaveLocalProject("proj-unreg", root)

	err := Clean(CleanArgs{
		Root:       root,
		Unregister: true,
		Yes:        true,
	})
	if err != nil {
		t.Fatalf("Clean unregister 失败: %v", err)
	}

	projects, _ := config.ListLocalProjects()
	if len(projects) != 0 {
		t.Fatalf("项目注销后列表应为空, 现有: %+v", projects)
	}
}

// TestClean005_BatchCleanRespectsFlags: 批量清理时尊重显式参数 (--bundles 仅清理 bundle，不清理 releases)
func TestClean005_BatchCleanRespectsFlags(t *testing.T) {
	rootA := setupCleanTestProject(t, "batch-a")
	rootB := setupCleanTestProject(t, "batch-b")

	targets := []targetItem{
		{Name: "batch-a", Root: rootA},
		{Name: "batch-b", Root: rootB},
	}

	// 传入 --bundles，批量清理应只清理 bundles，绝不触碰 releases
	err := cleanAllProjects(targets, CleanArgs{
		Bundles: true,
		Yes:     true,
	})
	if err != nil {
		t.Fatalf("cleanAllProjects 失败: %v", err)
	}

	for _, root := range []string{rootA, rootB} {
		// bundles 应被清空
		bEntries, _ := os.ReadDir(filepath.Join(root, "bundles"))
		if len(bEntries) != 0 {
			t.Errorf("项目 %s 的 bundles 目录应为空", root)
		}

		// 关键断言: releases/v1 历史版本绝对不能被删除！
		v1Dir := filepath.Join(root, "releases", "v1")
		if _, err := os.Stat(v1Dir); os.IsNotExist(err) {
			t.Fatalf("项目 %s 的 releases/v1 被错误删除了！--bundles 参数被批量操作忽略扩大了范围", root)
		}
	}
}

// TestClean005_BatchUnregisterDoesNotDeleteFiles: 批量注销仅注销项目，不删除物理文件
func TestClean005_BatchUnregisterDoesNotDeleteFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("UPY_CONFIG", path)
	t.Setenv("UPLY_CONFIG", "")

	rootA := setupCleanTestProject(t, "batch-unreg-a")
	rootB := setupCleanTestProject(t, "batch-unreg-b")

	_ = config.SaveLocalProject("batch-unreg-a", rootA)
	_ = config.SaveLocalProject("batch-unreg-b", rootB)

	targets := []targetItem{
		{Name: "batch-unreg-a", Root: rootA},
		{Name: "batch-unreg-b", Root: rootB},
	}

	err := cleanAllProjects(targets, CleanArgs{
		Unregister: true,
		Yes:        true,
	})
	if err != nil {
		t.Fatalf("cleanAllProjects unregister 失败: %v", err)
	}

	// 登记表应已清空
	projects, _ := config.ListLocalProjects()
	if len(projects) != 0 {
		t.Fatalf("项目登记表应为空，现有: %+v", projects)
	}

	// 物理文件全部保留
	for _, root := range []string{rootA, rootB} {
		if _, err := os.Stat(filepath.Join(root, "bundles")); err != nil {
			t.Errorf("项目 %s 的 bundles 物理目录应保留", root)
		}
		if _, err := os.Stat(filepath.Join(root, "releases", "v2")); err != nil {
			t.Errorf("项目 %s 的 releases/v2 物理目录应保留", root)
		}
	}
}
