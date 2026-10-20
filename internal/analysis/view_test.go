package analysis

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStringWidthAndPadding(t *testing.T) {
	// ASCII string
	if w := stringWidth("hello"); w != 5 {
		t.Errorf("expected width 5 for hello, got %d", w)
	}

	// Chinese characters (each 2 columns)
	if w := stringWidth("项目名称"); w != 8 {
		t.Errorf("expected width 8 for '项目名称', got %d", w)
	}

	// Mixed
	if w := stringWidth("upy 部署"); w != 8 { // 3 + 1 + 4 = 8
		t.Errorf("expected width 8 for 'upy 部署', got %d", w)
	}

	// padRight
	padded := padRight("项目", 10)
	if stringWidth(padded) != 10 {
		t.Errorf("expected padded width 10, got %d", stringWidth(padded))
	}
}

func TestRenderBar(t *testing.T) {
	// 100%
	b100 := RenderBar(100.0, 20)
	if strings.Contains(b100, "%") || stripANSI(b100) != strings.Repeat("━", 20) {
		t.Errorf("unexpected 100%% bar: %s", b100)
	}

	// 0%
	b0 := RenderBar(0.0, 20)
	if strings.Contains(b0, "%") || stripANSI(b0) != strings.Repeat("─", 20) {
		t.Errorf("unexpected 0%% bar: %s", b0)
	}

	// Small positive percentage
	b1 := RenderBar(0.5, 20)
	if strings.Contains(b1, "%") || !strings.Contains(b1, "━") {
		t.Errorf("expected at least 1 filled block for tiny non-zero percent: %s", b1)
	}
}

func TestRenderTerminalAndDetail(t *testing.T) {
	now := time.Now()
	tDeploy := now.Add(-15 * time.Minute)

	report := &AnalysisReport{
		AnalyzedAt:     now,
		TotalProjects:  2,
		TotalBytes:     1500 * 1024 * 1024,
		MaxBytes:       1000 * 1024 * 1024,
		MaxProjectName: "app-large",
		LatestDeploy: &ProjectAnalysis{
			Name:           "app-small",
			CurrentVersion: "v1.2.0",
			Type:           "system",
			UnitName:       "upy-app-small.service",
			DeployedAt:     &tDeploy,
			Status:         "active",
		},
		Projects: []ProjectAnalysis{
			{
				Name:            "app-large",
				Type:            "docker",
				CurrentVersion:  "v2.0.0",
				TotalBytes:      1000 * 1024 * 1024,
				ReleasesBytes:   800 * 1024 * 1024,
				ActiveBytes:     300 * 1024 * 1024,
				OldReleaseBytes: 500 * 1024 * 1024,
				BundlesBytes:    150 * 1024 * 1024,
				BackupsBytes:    50 * 1024 * 1024,
				PercentOfMax:    100.0,
				PercentOfTotal:  66.7,
			},
			{
				Name:           "app-small",
				Type:           "system",
				CurrentVersion: "v1.2.0",
				TotalBytes:     500 * 1024 * 1024,
				PercentOfMax:   50.0,
				PercentOfTotal: 33.3,
				DeployedAt:     &tDeploy,
				Status:         "active",
			},
		},
	}

	// Standard view
	out := RenderTerminal(report, false)
	if !strings.Contains(out, "upy analysis") {
		t.Errorf("expected title in output")
	}
	if !strings.Contains(out, "app-small (v1.2.0)") {
		t.Errorf("expected spotlight project in output")
	}
	if strings.Contains(out, "%") || !strings.Contains(out, strings.Repeat("━", 20)) {
		t.Errorf("expected graphic-only comparison bars")
	}
	if !strings.Contains(out, "存储汇总统计") {
		t.Errorf("expected summary section in output")
	}

	// Detail view
	outDetail := RenderTerminal(report, true)
	if !strings.Contains(outDetail, "--detail") {
		t.Errorf("expected detail header in output")
	}
	if !strings.Contains(outDetail, "├── releases/") {
		t.Errorf("expected sub-tree breakdown in detail output")
	}
}

func TestRenderJSON(t *testing.T) {
	report := &AnalysisReport{
		TotalProjects:  1,
		TotalBytes:     1024,
		MaxBytes:       1024,
		MaxProjectName: "app",
		Projects: []ProjectAnalysis{
			{Name: "app", TotalBytes: 1024, PercentOfMax: 100.0},
		},
	}

	jsonStr, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("failed to render JSON: %v", err)
	}

	var parsed AnalysisReport
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("failed to parse generated JSON: %v", err)
	}
	if parsed.TotalProjects != 1 || parsed.MaxProjectName != "app" {
		t.Errorf("unmarshaled JSON mismatch: %+v", parsed)
	}
}
