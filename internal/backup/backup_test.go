package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindScript(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 无脚本
	_, err := FindScript(tmpDir, "")
	if err != ErrNoScript {
		t.Fatalf("expected ErrNoScript, got: %v", err)
	}

	// 2. 项目根目录 backup.sh
	rootScript := filepath.Join(tmpDir, "backup.sh")
	if err := os.WriteFile(rootScript, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err := FindScript(tmpDir, "")
	if err != nil || found != rootScript {
		t.Fatalf("expected %s, got: %s (err: %v)", rootScript, found, err)
	}

	// 3. 自定义脚本
	custom := filepath.Join(tmpDir, "my-dump.sh")
	if err := os.WriteFile(custom, []byte("#!/bin/sh\necho custom\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	found, err = FindScript(tmpDir, "my-dump.sh")
	if err != nil || found != custom {
		t.Fatalf("expected %s, got: %s (err: %v)", custom, found, err)
	}

	// 4. 自定义脚本不存在
	_, err = FindScript(tmpDir, "not-exist.sh")
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("expected not exist error, got: %v", err)
	}
}

func TestScanAndPruneBackups(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	// 创建 4 个文件，时间从早到晚
	f1 := filepath.Join(backupDir, "backup-1.tar.gz")
	f2 := filepath.Join(backupDir, "backup-2.tar.gz")
	f3 := filepath.Join(backupDir, "backup-3.tar.gz")
	f4 := filepath.Join(backupDir, "backup-4.tar.gz")

	_ = os.WriteFile(f1, []byte("data1"), 0o644)
	_ = os.Chtimes(f1, now.Add(-4*time.Hour), now.Add(-4*time.Hour))

	_ = os.WriteFile(f2, []byte("data2"), 0o644)
	_ = os.Chtimes(f2, now.Add(-3*time.Hour), now.Add(-3*time.Hour))

	_ = os.WriteFile(f3, []byte("data3"), 0o644)
	_ = os.Chtimes(f3, now.Add(-2*time.Hour), now.Add(-2*time.Hour))

	_ = os.WriteFile(f4, []byte("data4"), 0o644)
	_ = os.Chtimes(f4, now.Add(-1*time.Hour), now.Add(-1*time.Hour))

	// 扫描验证
	files, err := ScanBackups(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d", len(files))
	}
	// 最新应该是 backup-4
	if files[0].Name != "backup-4.tar.gz" {
		t.Fatalf("expected backup-4.tar.gz to be first, got: %s", files[0].Name)
	}

	// 轮转保留最近 2 份，删除 2 份
	deleted, err := PruneBackups(backupDir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted files, got: %v", deleted)
	}

	remaining, _ := ScanBackups(backupDir)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining files, got %d", len(remaining))
	}
	if remaining[0].Name != "backup-4.tar.gz" || remaining[1].Name != "backup-3.tar.gz" {
		t.Fatalf("unexpected remaining files: %v", remaining)
	}
}

func TestRunSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "backup.sh")
	scriptContent := `#!/bin/sh
set -e
mkdir -p "$UPY_BACKUP_DIR"
echo "data from $UPY_PROJECT_NAME" > "$UPY_BACKUP_DIR/dump-$UPY_PROJECT_NAME-$UPY_TIMESTAMP.sql"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	res, err := Run(ctx, Options{
		Root: tmpDir,
	})
	if err != nil {
		t.Fatalf("expected run to succeed, got: %v", err)
	}

	if len(res.CreatedFiles) != 1 {
		t.Fatalf("expected 1 created file, got: %d", len(res.CreatedFiles))
	}
	if !strings.HasPrefix(res.CreatedFiles[0].Name, "dump-") {
		t.Fatalf("unexpected created file name: %s", res.CreatedFiles[0].Name)
	}
}

func TestRunScriptFailure(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "backup.sh")
	scriptContent := `#!/bin/sh
echo "error occurred" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err := Run(ctx, Options{
		Root: tmpDir,
	})
	if err == nil {
		t.Fatal("expected run error, got nil")
	}
}
