package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AaronConlon/upy/internal/state"
)

func TestCollectorCalculateSizeNoSymlinkDoubleCount(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 创建真实文件
	targetFile := filepath.Join(tmpDir, "target.bin")
	data := make([]byte, 1024*10) // 10 KB
	if err := os.WriteFile(targetFile, data, 0o644); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	// 2. 创建指向该文件的软链
	linkPath := filepath.Join(tmpDir, "link.bin")
	if err := os.Symlink(targetFile, linkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	total := calculateSize(tmpDir)
	// targetFile 是 10240 字节，linkPath 是软链（大小为 targetFile 路径长度，几个到几十字节）
	// 如果跟随了软链，大小会是 20480 + 额外开销
	if total >= 20000 {
		t.Fatalf("expected calculateSize to NOT follow symlink, but got %d bytes", total)
	}
	if total < 10240 {
		t.Fatalf("expected at least 10240 bytes, got %d", total)
	}
}

func TestCollectProjectFullStructure(t *testing.T) {
	projDir := t.TempDir()

	// 1. 创建 releases/v1.0.0 (活跃) 与 releases/v0.9.0 (旧版)
	v1Dir := filepath.Join(projDir, "releases", "v1.0.0")
	v09Dir := filepath.Join(projDir, "releases", "v0.9.0")
	if err := os.MkdirAll(v1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(v09Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v1Dir, "app.bin"), make([]byte, 4000), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v09Dir, "app.bin"), make([]byte, 2000), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. 创建 current 软链接指向 v1.0.0
	currentLink := filepath.Join(projDir, "current")
	if err := os.Symlink(filepath.Join("releases", "v1.0.0"), currentLink); err != nil {
		t.Fatal(err)
	}

	// 3. 创建 bundles/ 与 backups/
	bundlesDir := filepath.Join(projDir, "bundles")
	backupsDir := filepath.Join(projDir, "backups")
	_ = os.MkdirAll(bundlesDir, 0o755)
	_ = os.MkdirAll(backupsDir, 0o755)
	_ = os.WriteFile(filepath.Join(bundlesDir, "bundle.zip"), make([]byte, 1500), 0o644)
	_ = os.WriteFile(filepath.Join(backupsDir, "backup.tar.gz"), make([]byte, 800), 0o644)

	// 4. 创建 state.json
	deployTime := "2026-10-19T10:00:00Z"
	st := state.State{
		CurrentVersion:  "v1.0.0",
		PreviousVersion: "v0.9.0",
		DeployedAt:      deployTime,
		Type:            "system",
		UnitName:        "upy-demo.service",
		Status:          "active",
	}
	stData, _ := json.Marshal(st)
	_ = os.WriteFile(filepath.Join(projDir, "state.json"), stData, 0o644)

	// 5. 执行收集
	pa := CollectProject("demo-project", projDir)

	if pa.Name != "demo-project" {
		t.Errorf("expected name demo-project, got %s", pa.Name)
	}
	if pa.Type != "system" {
		t.Errorf("expected type system, got %s", pa.Type)
	}
	if pa.CurrentVersion != "v1.0.0" {
		t.Errorf("expected currentVersion v1.0.0, got %s", pa.CurrentVersion)
	}
	if pa.DeployedAt == nil || pa.DeployedAt.Format(time.RFC3339) != deployTime {
		t.Errorf("expected deployedAt %s, got %v", deployTime, pa.DeployedAt)
	}
	if pa.ActiveBytes < 4000 {
		t.Errorf("expected activeBytes >= 4000, got %d", pa.ActiveBytes)
	}
	if pa.OldReleaseBytes < 2000 {
		t.Errorf("expected oldReleaseBytes >= 2000, got %d", pa.OldReleaseBytes)
	}
	if pa.BundlesBytes < 1500 {
		t.Errorf("expected bundlesBytes >= 1500, got %d", pa.BundlesBytes)
	}
	if pa.BackupsBytes < 800 {
		t.Errorf("expected backupsBytes >= 800, got %d", pa.BackupsBytes)
	}

	// 验证没有穿透 current 导致 ActiveBytes 被算入两次
	if pa.TotalBytes >= 12000 {
		t.Errorf("totalBytes %d indicates symlink double-counting occurred", pa.TotalBytes)
	}
}

func TestCollectProjectMissingDirectory(t *testing.T) {
	pa := CollectProject("missing-proj", "/path/to/non/existent/dir/xyz")
	if !pa.IsMissingRoot {
		t.Errorf("expected IsMissingRoot to be true")
	}
	if pa.Status != "missing" {
		t.Errorf("expected Status to be missing, got %s", pa.Status)
	}
	if pa.TotalBytes != 0 {
		t.Errorf("expected TotalBytes to be 0, got %d", pa.TotalBytes)
	}
}
