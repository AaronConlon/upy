package github

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func sampleRelease() *GHRelease {
	return &GHRelease{
		Tag: "v0.5.1",
		Assets: []GHAsset{
			{ID: 1, Name: "bundle.zip"},
			{ID: 2, Name: "test-deploy-website-v0.5.1-20260815.zip"},
		},
	}
}

// 精确匹配仍优先 (默认配置 asset: bundle.zip)
func TestFindAssetExact(t *testing.T) {
	a, err := FindAsset(sampleRelease(), "bundle.zip", "test-deploy-website", "test-deploy-website")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 1 {
		t.Fatalf("期望精确匹配 bundle.zip, 得到 %s", a.Name)
	}
}

// 配置名不存在时, 模糊匹配 "项目名-版本tag-日期.zip"
func TestFindAssetFuzzy(t *testing.T) {
	a, err := FindAsset(sampleRelease(), "whatever.zip", "test-deploy-website", "test-deploy-website")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 2 {
		t.Fatalf("期望模糊匹配版本化资产, 得到 %s", a.Name)
	}
}

// 项目名不匹配 → 找不到
func TestFindAssetNoMatch(t *testing.T) {
	_, err := FindAsset(sampleRelease(), "whatever.zip", "other-project", "other-project")
	if err == nil {
		t.Fatal("期望报错")
	}
}

// 无项目名线索时仅精确匹配 (自更新场景)
func TestFindAssetExactOnly(t *testing.T) {
	a, err := FindAsset(sampleRelease(), "bundle.zip", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != 1 {
		t.Fatalf("期望精确匹配, 得到 %s", a.Name)
	}
}

func TestDownloadAssetSuccess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DEPLOY_GITHUB_TOKEN", "")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dummy content"))
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "downloaded.bin")
	asset := &GHAsset{Name: "test.bin", URL: ts.URL, Size: int64(len("dummy content"))}
	if err := DownloadAsset("owner/repo", asset, dest); err != nil {
		t.Fatalf("下载期望成功，但返回错误: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("无法读取目标文件: %v", err)
	}
	if string(data) != "dummy content" {
		t.Fatalf("内容不符合预期: %q", string(data))
	}
}

func TestDownloadAssetIncompleteCleansUp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DEPLOY_GITHUB_TOKEN", "")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("short")) // 只有 5 字节，但 asset.Size 声明为 100
	}))
	defer ts.Close()

	destDir := t.TempDir()
	dest := filepath.Join(destDir, "corrupted.bin")
	asset := &GHAsset{Name: "corrupted.bin", URL: ts.URL, Size: 100}

	err := DownloadAsset("owner/repo", asset, dest)
	if err == nil {
		t.Fatal("期望因字节数不足报错，但未报错")
	}

	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("下载失败后目标文件不应存在，但发现目标文件仍存在: %v", statErr)
	}

	// 确认没有任何临时文件残留
	entries, _ := os.ReadDir(destDir)
	if len(entries) != 0 {
		t.Fatalf("下载失败后目录应干净，但发现残留条目: %v", entries)
	}
}

