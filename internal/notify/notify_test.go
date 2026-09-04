package notify

import (
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
