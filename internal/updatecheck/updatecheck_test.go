package updatecheck

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
)

func TestStringWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"abc", 3},
		{"hello world", 11},
		{"中文", 4},
		{"🔔", 2},
		{"🔔 发现新版本", 13}, // 2 + 1 + 10 = 13
		{log.Bold("upy ") + log.Green("v1.0.0"), 10}, // upy (4) + v1.0.0 (6) = 10
	}
	for _, tc := range tests {
		got := stringWidth(tc.input)
		if got != tc.want {
			t.Errorf("stringWidth(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

func TestRenderPromptAlignment(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en"} {
		_ = i18n.SetLanguage(lang)
		prompt := RenderPrompt("v0.8.5", "v0.8.6")

		lines := strings.Split(strings.Trim(prompt, "\r\n"), "\n")
		if len(lines) < 4 {
			t.Fatalf("[%s] Prompt lines count = %d, want >= 4", lang, len(lines))
		}

		lineWidths := make([]int, len(lines))
		for i, line := range lines {
			lineWidths[i] = stringWidth(line)
		}

		expectedWidth := lineWidths[0]
		for i, w := range lineWidths {
			if w != expectedWidth {
				t.Errorf("[%s] line %d width = %d, want %d (content: %q)", lang, i, w, expectedWidth, lines[i])
			}
		}

		// 必须包含版本与命令信息
		clean := log.StripAnsi(prompt)
		if !strings.Contains(clean, "v0.8.5") || !strings.Contains(clean, "v0.8.6") || !strings.Contains(clean, "upy update") {
			t.Errorf("[%s] prompt missing expected keywords: %s", lang, clean)
		}
	}
}

func TestCheckerDisabled(t *testing.T) {
	f := false
	cfg := &config.UserConfig{
		Update: config.UpdateConfig{
			Check: &f,
		},
	}
	c := Start(cfg, "v0.8.5", "", "")
	if c.enabled {
		t.Fatal("Checker should be disabled when update.check is false")
	}

	var buf bytes.Buffer
	c.Notify(&buf)
	if buf.Len() > 0 {
		t.Fatalf("Disabled checker should not print anything, got: %s", buf.String())
	}
}

func TestCheckerWithFreshCacheNewerVersion(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "update_check.json")

	// 写入 1 小时前的缓存，新版本为 v0.9.0
	cData := CacheData{
		LastCheckedAt: time.Now().Add(-1 * time.Hour).Unix(),
		LatestVersion: "v0.9.0",
	}
	if err := saveCache(cacheFile, cData); err != nil {
		t.Fatal(err)
	}

	cfg := &config.UserConfig{}
	c := Start(cfg, "v0.8.5", "AaronConlon/upy", cacheFile)
	if !c.enabled {
		t.Fatal("Checker should be enabled")
	}
	if c.newVersion != "v0.9.0" {
		t.Fatalf("c.newVersion = %q, want v0.9.0", c.newVersion)
	}

	var buf bytes.Buffer
	c.Notify(&buf)

	output := buf.String()
	if !strings.Contains(output, "v0.8.5") || !strings.Contains(output, "v0.9.0") {
		t.Fatalf("Expected update notification in output, got: %s", output)
	}
}

func TestCheckerWithFreshCacheSameVersion(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "update_check.json")

	// 写入当前版本相同缓存
	cData := CacheData{
		LastCheckedAt: time.Now().Add(-1 * time.Hour).Unix(),
		LatestVersion: "v0.8.5",
	}
	if err := saveCache(cacheFile, cData); err != nil {
		t.Fatal(err)
	}

	cfg := &config.UserConfig{}
	c := Start(cfg, "v0.8.5", "AaronConlon/upy", cacheFile)

	var buf bytes.Buffer
	c.Notify(&buf)

	if buf.Len() > 0 {
		t.Fatalf("Same version should not output notification, got: %s", buf.String())
	}
}
