package analysis

import (
	"testing"
	"time"
)

func TestAnalyzeBasicAndBenchmark(t *testing.T) {
	now := time.Now()
	t1 := now.Add(-3 * 24 * time.Hour) // 3 days ago
	t2 := now.Add(-10 * time.Minute)   // 10 minutes ago (latest)
	t3 := now.Add(-14 * 24 * time.Hour)

	projects := []ProjectAnalysis{
		{
			Name:       "proj-b",
			TotalBytes: 400,
			DeployedAt: &t2,
			Status:     "active",
		},
		{
			Name:       "proj-a",
			TotalBytes: 1000,
			DeployedAt: &t1,
			Status:     "active",
		},
		{
			Name:       "proj-c",
			TotalBytes: 100,
			DeployedAt: &t3,
			Status:     "active",
		},
	}

	report := Analyze(projects, "size")

	if report.TotalBytes != 1500 {
		t.Errorf("expected totalBytes 1500, got %d", report.TotalBytes)
	}
	if report.MaxBytes != 1000 {
		t.Errorf("expected maxBytes 1000, got %d", report.MaxBytes)
	}
	if report.MaxProjectName != "proj-a" {
		t.Errorf("expected maxProjectName proj-a, got %s", report.MaxProjectName)
	}
	if report.LatestDeploy == nil || report.LatestDeploy.Name != "proj-b" {
		t.Errorf("expected latest deploy proj-b, got %v", report.LatestDeploy)
	}

	// 检查默认按大小降序
	if len(report.Projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(report.Projects))
	}
	if report.Projects[0].Name != "proj-a" || report.Projects[0].PercentOfMax != 100.0 {
		t.Errorf("expected first project proj-a at 100.0%%, got %s (%.1f%%)", report.Projects[0].Name, report.Projects[0].PercentOfMax)
	}
	if report.Projects[1].Name != "proj-b" || report.Projects[1].PercentOfMax != 40.0 {
		t.Errorf("expected second project proj-b at 40.0%%, got %s (%.1f%%)", report.Projects[1].Name, report.Projects[1].PercentOfMax)
	}
	if report.Projects[2].Name != "proj-c" || report.Projects[2].PercentOfMax != 10.0 {
		t.Errorf("expected third project proj-c at 10.0%%, got %s (%.1f%%)", report.Projects[2].Name, report.Projects[2].PercentOfMax)
	}
}

func TestAnalyzeSorting(t *testing.T) {
	now := time.Now()
	t1 := now.Add(-3 * time.Hour)
	t2 := now.Add(-1 * time.Hour)

	projects := []ProjectAnalysis{
		{Name: "beta", TotalBytes: 500, DeployedAt: &t1},
		{Name: "alpha", TotalBytes: 100, DeployedAt: &t2},
	}

	// 1. Sort by time (latest first)
	repTime := Analyze(projects, "time")
	if repTime.Projects[0].Name != "alpha" {
		t.Errorf("expected alpha first for time sort, got %s", repTime.Projects[0].Name)
	}

	// 2. Sort by name
	repName := Analyze(projects, "name")
	if repName.Projects[0].Name != "alpha" || repName.Projects[1].Name != "beta" {
		t.Errorf("expected alpha then beta for name sort")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		t    *time.Time
		want string
	}{
		{nil, "-"},
		{func() *time.Time { t := now.Add(-20 * time.Second); return &t }(), "刚刚"},
		{func() *time.Time { t := now.Add(-15 * time.Minute); return &t }(), "15分钟前"},
		{func() *time.Time { t := now.Add(-5 * time.Hour); return &t }(), "5小时前"},
		{func() *time.Time { t := now.Add(-4 * 24 * time.Hour); return &t }(), "4天前"},
		{func() *time.Time { t := now.Add(-60 * 24 * time.Hour); return &t }(), now.Add(-60 * 24 * time.Hour).Format("2006-01-02")},
	}

	for _, c := range cases {
		got := FormatRelativeTime(c.t, now)
		if got != c.want {
			t.Errorf("FormatRelativeTime(%v) = %s, want %s", c.t, got, c.want)
		}
	}
}

func TestAnalyzeEmptyAndZero(t *testing.T) {
	repEmpty := Analyze(nil, "size")
	if repEmpty.TotalProjects != 0 || repEmpty.TotalBytes != 0 {
		t.Errorf("expected empty report, got %+v", repEmpty)
	}

	zeroProjects := []ProjectAnalysis{
		{Name: "zero1", TotalBytes: 0},
		{Name: "zero2", TotalBytes: 0},
	}
	repZero := Analyze(zeroProjects, "size")
	if repZero.MaxBytes != 0 {
		t.Errorf("expected maxBytes 0, got %d", repZero.MaxBytes)
	}
	if repZero.Projects[0].PercentOfMax != 0.0 {
		t.Errorf("expected percent 0.0, got %f", repZero.Projects[0].PercentOfMax)
	}
}
