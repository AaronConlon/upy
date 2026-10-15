package deploy

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestMoveDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "target")

	// 创建源目录中的层级结构
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := moveDir(src, dst); err != nil {
		t.Fatalf("moveDir 失败: %v", err)
	}

	// 确认目标结构完整
	aData, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil || string(aData) != "hello" {
		t.Fatalf("读取 a.txt 失败: %v, 内容: %s", err, string(aData))
	}
	bData, err := os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil || string(bData) != "world" {
		t.Fatalf("读取 sub/b.txt 失败: %v, 内容: %s", err, string(bData))
	}

	// 源目录应已被清理
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("源目录应该已被删除")
	}
}

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "target")

	if err := os.MkdirAll(filepath.Join(src, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "dir", "f.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir 失败: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "dir", "f.txt"))
	if err != nil || string(data) != "content" {
		t.Fatalf("copyDir 内容校验失败: %v, data: %s", err, string(data))
	}
}

func TestDeployBundleCreatesTmpUnderReleasesDir(t *testing.T) {
	root := t.TempDir()
	bundlePath := filepath.Join(t.TempDir(), "bundle.zip")

	// 构造一个 static 类型的测试 bundle
	f, err := os.Create(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("deploy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	manifest := "name: demo\nversion: 1.0.0\ntype: static\n"
	if _, err := w.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	wDist, err := zw.Create("dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wDist.Write([]byte("<h1>hello</h1>")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	res, err := DeployBundle(bundlePath, Options{Root: root, Force: true})
	if err != nil {
		t.Fatalf("DeployBundle 失败: %v", err)
	}
	if res.Version != "1.0.0" {
		t.Fatalf("期望版本 1.0.0, 得到 %s", res.Version)
	}

	releaseDir := filepath.Join(root, "releases", "1.0.0")
	if _, err := os.Stat(releaseDir); err != nil {
		t.Fatalf("版本目录未创建: %v", err)
	}

	// 确认 releasesDir 下没有任何遗留的 .tmp-deploy- 临时目录
	entries, _ := os.ReadDir(filepath.Join(root, "releases"))
	for _, e := range entries {
		if e.Name() != "1.0.0" {
			t.Fatalf("releases 目录下存在未清理的多余条目: %s", e.Name())
		}
	}
}
