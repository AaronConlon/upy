package notify

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatEventSuccess(t *testing.T) {
	title, body, group, level := formatEvent(Event{
		Kind:    KindSuccess,
		Project: "test-deploy-website",
		Version: "v0.5.2",
		Root:    "/opt/apps/site",
		Type:    "static",
		Elapsed: "3.2s",
	})
	if title != "Uply 部署成功 · test-deploy-website" {
		t.Fatalf("title=%s", title)
	}
	if group != "success" || level != "active" {
		t.Fatalf("group=%s level=%s", group, level)
	}
	for _, want := range []string{"项目: test-deploy-website", "版本: v0.5.2", "类型: 静态站点", "耗时: 3.2s"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}

func TestFormatEventFailureCompose(t *testing.T) {
	title, body, group, level := formatEvent(Event{
		Kind:    KindFailure,
		Project: "app",
		Version: "v1.2.0",
		Root:    "/opt/apps/app",
		Type:    "docker",
		Mode:    "compose",
		Error:   "docker compose build failed",
	})
	if title != "Uply 部署失败 · app" {
		t.Fatalf("title=%s", title)
	}
	if group != "failure" || level != "timeSensitive" {
		t.Fatalf("group=%s level=%s", group, level)
	}
	if !strings.Contains(body, "类型: Docker Compose 集成项目") {
		t.Fatalf("body=%s", body)
	}
	if !strings.Contains(body, "原因: docker compose build failed") {
		t.Fatalf("body=%s", body)
	}
}

func TestSendWithProjectAndUserBarks(t *testing.T) {
	var receivedURLs []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURLs = append(receivedURLs, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	defer ts.Close()

	// 准备用户配置: ~/.upy/config.yaml
	userConfigDir := t.TempDir()
	userConfigFile := filepath.Join(userConfigDir, "config.yaml")
	userYAML := fmt.Sprintf(`notify:
  bark:
    - serverUrl: %s
      token: token_global_1
    - serverUrl: %s
      token: token_common
`, ts.URL, ts.URL)
	_ = os.WriteFile(userConfigFile, []byte(userYAML), 0o644)
	t.Setenv("UPY_CONFIG", userConfigFile)

	// 准备项目配置: <projectRoot>/deploy.yaml
	projectDir := t.TempDir()
	projectDeployYAML := fmt.Sprintf(`name: test-demo
release:
  provider: github
  repository: owner/test-demo
notify:
  bark:
    - serverUrl: %s
      token: token_project_1
    - serverUrl: %s
      token: token_common
`, ts.URL, ts.URL)
	_ = os.WriteFile(filepath.Join(projectDir, "deploy.yaml"), []byte(projectDeployYAML), 0o644)

	// 发送通知
	Send(Event{
		Kind:    KindSuccess,
		Project: "test-demo",
		Version: "v1.0.0",
		Root:    projectDir,
		Type:    "static",
		Elapsed: "1.2s",
	})

	// 验证请求数量与去重: token_project_1, token_common, token_global_1 共 3 个不同的 token，token_common 仅发 1 次
	if len(receivedURLs) != 3 {
		t.Fatalf("期望收到 3 次推送 (去重后)，实际收到 %d: %+v", len(receivedURLs), receivedURLs)
	}

	expectedPaths := map[string]bool{
		"/token_project_1": true,
		"/token_common":    true,
		"/token_global_1":  true,
	}
	for _, p := range receivedURLs {
		if !expectedPaths[p] {
			t.Errorf("收到意外的推送路径: %s", p)
		}
	}
}

func TestSendProjectExplicitlyDisabled(t *testing.T) {
	var receivedCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	userConfigDir := t.TempDir()
	userConfigFile := filepath.Join(userConfigDir, "config.yaml")
	userYAML := fmt.Sprintf(`notify:
  bark:
    - serverUrl: %s
      token: global_key
`, ts.URL)
	_ = os.WriteFile(userConfigFile, []byte(userYAML), 0o644)
	t.Setenv("UPY_CONFIG", userConfigFile)

	projectDir := t.TempDir()
	projectDeployYAML := `name: disabled-demo
release:
  provider: github
  repository: owner/disabled-demo
notify:
  enabled: false
`
	_ = os.WriteFile(filepath.Join(projectDir, "deploy.yaml"), []byte(projectDeployYAML), 0o644)

	Send(Event{
		Kind:    KindSuccess,
		Project: "disabled-demo",
		Version: "v1.0.0",
		Root:    projectDir,
	})

	if receivedCount != 0 {
		t.Fatalf("项目已显式禁用通知，但收到了 %d 条推送", receivedCount)
	}
}
