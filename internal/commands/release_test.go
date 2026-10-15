package commands

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func createTestBundleZip(t *testing.T, dir, filename, version string) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create("deploy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	manifest := "name: test-app\nversion: " + version + "\ntype: static\n"
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheckBundleCacheNonexistent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bundle.zip")
	if checkBundleCache(p, "v1.0.0", false) {
		t.Fatal("不存在的文件不应复用")
	}
}

func TestCheckBundleCacheCorruptedCleansUp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bundle.zip")
	// 写入一个损坏的半截 zip 文件
	if err := os.WriteFile(p, []byte("PK\x03\x04corrupted-partial-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if checkBundleCache(p, "v1.0.0", false) {
		t.Fatal("损坏的 bundle 不应被复用")
	}

	// 损坏的 bundle 应被清理
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("损坏的 bundle 应该已被清理，但文件依然存在: %v", err)
	}
}

func TestCheckBundleCacheVersionMismatchCleansUp(t *testing.T) {
	dir := t.TempDir()
	p := createTestBundleZip(t, dir, "bundle.zip", "v1.0.0")

	if checkBundleCache(p, "v2.0.0", false) {
		t.Fatal("版本不一致时不应复用")
	}

	// 版本不一致的旧缓存应该被清理
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("版本不一致的旧 bundle 应该被清理，但依然存在: %v", err)
	}
}

func TestCheckBundleCacheValidMatch(t *testing.T) {
	dir := t.TempDir()
	p := createTestBundleZip(t, dir, "bundle.zip", "v1.0.0")

	if !checkBundleCache(p, "v1.0.0", false) {
		t.Fatal("合法且版本一致的 bundle 应成功复用")
	}

	// 合法的 bundle 应被保留
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("复用的 bundle 应该保留在磁盘上: %v", err)
	}
}

func TestCheckBundleCacheForce(t *testing.T) {
	dir := t.TempDir()
	p := createTestBundleZip(t, dir, "bundle.zip", "v1.0.0")

	if checkBundleCache(p, "v1.0.0", true) {
		t.Fatal("--force 时不应复用缓存")
	}
}
