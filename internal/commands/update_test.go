package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceExecutableSameDir(t *testing.T) {
	dir := t.TempDir()

	dst := filepath.Join(dir, "app")
	if err := os.WriteFile(dst, []byte("old-binary-content"), 0o755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(dir, ".app-new")
	if err := os.WriteFile(src, []byte("new-binary-content"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceExecutable(src, dst); err != nil {
		t.Fatalf("replaceExecutable 失败: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "new-binary-content" {
		t.Fatalf("内容未更新或读取失败: data=%s, err=%v", string(data), err)
	}

	info, err := os.Stat(dst)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("权限异常: perm=%o, err=%v", info.Mode().Perm(), err)
	}
}

func TestReplaceExecutableDifferentDir(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	dst := filepath.Join(dirB, "app")
	if err := os.WriteFile(dst, []byte("old-version"), 0o755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(dirA, "downloaded-app")
	if err := os.WriteFile(src, []byte("new-version"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceExecutable(src, dst); err != nil {
		t.Fatalf("replaceExecutable 失败: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "new-version" {
		t.Fatalf("内容未更新或读取失败: data=%s, err=%v", string(data), err)
	}

	info, err := os.Stat(dst)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("权限异常: perm=%o, err=%v", info.Mode().Perm(), err)
	}
}
