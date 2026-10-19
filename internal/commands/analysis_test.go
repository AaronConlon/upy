package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AaronConlon/upy/internal/state"
)

func TestAnalysisCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 创建 deploy.yaml
	deployYaml := `name: test-analysis-app
release:
  provider: github
  repository: AaronConlon/test-analysis-app
`
	if err := os.WriteFile(filepath.Join(tmpDir, "deploy.yaml"), []byte(deployYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. 创建 releases 与 state.json
	vDir := filepath.Join(tmpDir, "releases", "v1.0.0")
	if err := os.MkdirAll(vDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vDir, "app.bin"), make([]byte, 5000), 0o644); err != nil {
		t.Fatal(err)
	}

	st := state.State{
		CurrentVersion: "v1.0.0",
		DeployedAt:     time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		Type:           "system",
		UnitName:       "upy-test-analysis-app.service",
		Status:         "active",
	}
	stData, _ := json.Marshal(st)
	_ = os.WriteFile(filepath.Join(tmpDir, "state.json"), stData, 0o644)

	// 3. 测试标准执行
	err := Analysis(AnalysisArgs{
		Root: tmpDir,
	})
	if err != nil {
		t.Fatalf("expected Analysis to succeed, got: %v", err)
	}

	// 4. 测试 --detail
	err = Analysis(AnalysisArgs{
		Root:   tmpDir,
		Detail: true,
	})
	if err != nil {
		t.Fatalf("expected Analysis --detail to succeed, got: %v", err)
	}

	// 5. 测试 --json
	err = Analysis(AnalysisArgs{
		Root: tmpDir,
		JSON: true,
	})
	if err != nil {
		t.Fatalf("expected Analysis --json to succeed, got: %v", err)
	}

	// 6. 测试指定存在的项目
	err = Analysis(AnalysisArgs{
		Root:        tmpDir,
		ProjectName: "test-analysis-app",
	})
	if err != nil {
		t.Fatalf("expected Analysis for specific project to succeed, got: %v", err)
	}

	// 7. 测试指定不存在的项目
	err = Analysis(AnalysisArgs{
		Root:        tmpDir,
		ProjectName: "non-existent-xyz-app",
	})
	if err == nil {
		t.Fatalf("expected error for non-existent project, but got nil")
	}

	// 8. 测试不同排序方式
	for _, sortOpt := range []string{"size", "time", "name"} {
		err = Analysis(AnalysisArgs{
			Root: tmpDir,
			Sort: sortOpt,
		})
		if err != nil {
			t.Fatalf("expected Analysis --sort %s to succeed, got: %v", sortOpt, err)
		}
	}
}
