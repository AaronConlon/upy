package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackupCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// 部署清单
	deployYaml := `name: test-app
release:
  provider: github
  repository: AaronConlon/test-app
backup:
  dir: my-backups
`
	if err := os.WriteFile(filepath.Join(tmpDir, "deploy.yaml"), []byte(deployYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	// 备份脚本
	script := `#!/bin/sh
set -e
mkdir -p "$UPY_BACKUP_DIR"
echo "hello world" > "$UPY_BACKUP_DIR/data-$UPY_TIMESTAMP.dump"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "backup.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1. 执行备份
	err := Backup(BackupArgs{
		Root: tmpDir,
	})
	if err != nil {
		t.Fatalf("expected backup to succeed, got: %v", err)
	}

	// 2. 检查 --list
	err = Backup(BackupArgs{
		Root: tmpDir,
		List: true,
	})
	if err != nil {
		t.Fatalf("expected backup --list to succeed, got: %v", err)
	}

	// 3. 检查 --prune
	err = Backup(BackupArgs{
		Root:  tmpDir,
		Prune: 1,
	})
	if err != nil {
		t.Fatalf("expected backup --prune to succeed, got: %v", err)
	}
}
