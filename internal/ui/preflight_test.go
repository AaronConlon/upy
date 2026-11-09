package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AaronConlon/upy/internal/config"
)

func TestCheckPreflight_PackageJsonMismatch(t *testing.T) {
	tmp := t.TempDir()
	extractedDir := filepath.Join(tmp, "extracted")
	if err := os.MkdirAll(extractedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 写入与 manifest 版本不一致的 package.json
	pkgContent := `{"name": "demo-api", "version": "1.0.3"}`
	if err := os.WriteFile(filepath.Join(extractedDir, "package.json"), []byte(pkgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := &config.Manifest{
		Name:    "demo-api",
		Version: "v1.0.4",
		Type:    "docker",
		Docker:  &config.ManifestDocker{Mode: "compose"},
	}

	pcfg := &config.ProjectConfig{
		Name: "demo-api",
	}

	rep := CheckPreflight(extractedDir, tmp, manifest, pcfg)
	if !rep.IsMismatch {
		t.Errorf("期望检测到版本不一致 IsMismatch=true")
	}
	if rep.PackageVersion != "1.0.3" {
		t.Errorf("期望 packageVersion=1.0.3, got=%s", rep.PackageVersion)
	}
	if len(rep.Warnings) == 0 {
		t.Errorf("期望包含警告信息，实际为空")
	}

	// 验证 PrintPreflightCard 执行不出错
	PrintPreflightCard(rep)
	PrintStep(1, 5, "测试步骤")
	PrintSuccessBanner("demo-api", "v1.0.4", "1.2s")
	PrintRollbackAlert("v1.0.3", "测试回滚原因")
}

func TestCheckPreflight_MatchingPackageJson(t *testing.T) {
	tmp := t.TempDir()
	extractedDir := filepath.Join(tmp, "extracted")
	if err := os.MkdirAll(extractedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pkgContent := `{"name": "demo-api", "version": "1.0.4"}`
	if err := os.WriteFile(filepath.Join(extractedDir, "package.json"), []byte(pkgContent), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest := &config.Manifest{
		Name:    "demo-api",
		Version: "1.0.4",
		Type:    "static",
	}

	rep := CheckPreflight(extractedDir, tmp, manifest, nil)
	if rep.IsMismatch {
		t.Errorf("版本一致时不应标记 IsMismatch")
	}
	if len(rep.Warnings) != 0 {
		t.Errorf("版本一致时不应产生警告, got=%v", rep.Warnings)
	}
}

func TestFormatOptionLine(t *testing.T) {
	opt := SelectOption{
		Label: "v1.0.4",
		Warn:  "⚠️ (package: 1.0.3)",
		Badge: "(当前运行 / ACTIVE)",
	}
	line := formatOptionLine(opt)
	if !strings.Contains(line, "v1.0.4") || !strings.Contains(line, "1.0.3") || !strings.Contains(line, "ACTIVE") {
		t.Fatalf("formatOptionLine 未包含全部必要字段: %s", line)
	}
}
