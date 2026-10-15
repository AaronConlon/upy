package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestDownloadAssetFallback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DEPLOY_GITHUB_TOKEN", "mock_token")

	// 模拟 public fallback 服务器
	publicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fallback success content"))
	}))
	defer publicServer.Close()

	// 模拟 API 服务器返回 500
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer apiServer.Close()

	origAPIBase := apiBase
	apiBase = apiServer.URL
	defer func() { apiBase = origAPIBase }()

	dest := filepath.Join(t.TempDir(), "fallback.bin")
	asset := &GHAsset{
		ID:   123,
		Name: "fallback.bin",
		URL:  publicServer.URL,
		Size: int64(len("fallback success content")),
	}

	if err := DownloadAsset("owner/repo", asset, dest); err != nil {
		t.Fatalf("API 失败后期望自动降级公共直链成功，但返回错误: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("无法读取目标文件: %v", err)
	}
	if string(data) != "fallback success content" {
		t.Fatalf("文件内容不匹配: %q", string(data))
	}
}

func TestEnvProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("ALL_PROXY", "socks5://127.0.0.1:1080")

	// 外部请求应走 ALL_PROXY
	req, _ := http.NewRequest(http.MethodGet, "https://github.com/foo/bar", nil)
	proxyURL, err := envProxy(req)
	if err != nil {
		t.Fatalf("envProxy 失败: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "socks5://127.0.0.1:1080" {
		t.Fatalf("期望解析到 ALL_PROXY socks5://127.0.0.1:1080, 实际为 %v", proxyURL)
	}

	// 本地 loopback 请求应绕过代理
	reqLocal, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/test", nil)
	proxyLocal, err := envProxy(reqLocal)
	if err != nil {
		t.Fatalf("envProxy loopback 失败: %v", err)
	}
	if proxyLocal != nil {
		t.Fatalf("loopback 请求不应走代理，实际为: %v", proxyLocal)
	}

	// NO_PROXY 匹配的目标应绕过代理
	t.Setenv("NO_PROXY", "example.com,internal.net")
	reqNoProxy, _ := http.NewRequest(http.MethodGet, "https://api.example.com/test", nil)
	proxyNoProxy, err := envProxy(reqNoProxy)
	if err != nil {
		t.Fatalf("envProxy NO_PROXY 失败: %v", err)
	}
	if proxyNoProxy != nil {
		t.Fatalf("NO_PROXY 匹配请求不应走代理，实际为: %v", proxyNoProxy)
	}
}

func TestDownloadAssetResumeOnDisconnect(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DEPLOY_GITHUB_TOKEN", "")

	fullContent := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	attempts := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		rangeHeader := r.Header.Get("Range")
		if attempts == 1 {
			// 第一次连接：发送前 10 个字节后强制断开连接，模拟网络中断
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Length", strconv.Itoa(len(fullContent)))
			w.WriteHeader(http.StatusOK)
			w.Write(fullContent[:10])
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
			return
		}

		// 第二次连接：断点续传，验证从 byte 10 开始请求
		if strings.HasPrefix(rangeHeader, "bytes=10-") {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 10-%d/%d", len(fullContent)-1, len(fullContent)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(fullContent[10:])
			return
		}

		// 默认完整返回
		w.WriteHeader(http.StatusOK)
		w.Write(fullContent)
	}))
	defer ts.Close()

	dest := filepath.Join(t.TempDir(), "resumed.bin")
	asset := &GHAsset{Name: "resumed.bin", URL: ts.URL, Size: int64(len(fullContent))}
	if err := DownloadAsset("owner/repo", asset, dest); err != nil {
		t.Fatalf("断点续传期望成功，但返回错误: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("无法读取目标文件: %v", err)
	}
	if string(data) != string(fullContent) {
		t.Fatalf("内容不符合预期: 得到 %q, 期望 %q", string(data), string(fullContent))
	}
	if attempts < 2 {
		t.Fatalf("期望发生重试与续传，实际请求次数: %d", attempts)
	}
}



