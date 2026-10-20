package analysis

import (
	"strings"
	"testing"

	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
)

func TestThinProjectBarsAndLocales(t *testing.T) {
	previous := i18n.Language()
	defer i18n.SetLanguage(previous)
	color := log.UseColor
	defer func() { log.UseColor = color }()
	log.UseColor = true
	projects := []ProjectAnalysis{{Name: "中文项目", Type: "static", TotalBytes: 100, PercentOfMax: 100}, {Name: "api", Type: "system", TotalBytes: 50, PercentOfMax: 50}, {Name: "web", Type: "docker", TotalBytes: 25, PercentOfMax: 25}}
	colors := projectColors(projects)
	bars := map[string]bool{}
	for _, p := range projects {
		bar := renderProjectBar(100, 20, colors[p.Name])
		if bars[bar] {
			t.Fatal("project colors repeated")
		}
		bars[bar] = true
		if strings.ContainsAny(bar, "%█░") {
			t.Fatalf("full-height or percentage bar: %q", bar)
		}
	}
	projects[0], projects[2] = projects[2], projects[0]
	reordered := projectColors(projects)
	for name, index := range colors {
		if reordered[name] != index {
			t.Fatal("color changed on sort")
		}
	}
	report := &AnalysisReport{Projects: projects, TotalProjects: 3, TotalBytes: 175, MaxBytes: 100, MaxProjectName: "中文项目"}
	for _, language := range []string{"en", "zh-CN"} {
		i18n.SetLanguage(language)
		for _, detail := range []bool{false, true} {
			rendered := stripANSI(RenderTerminal(report, detail))
			if strings.Contains(rendered, "%") {
				t.Fatal("percentage remains in terminal report")
			}
			if !strings.Contains(rendered, "中文项目") {
				t.Fatal("project name translated")
			}
			if language == "en" && !strings.Contains(rendered, "Project Analysis") {
				t.Fatal("English header missing")
			}
			if language == "zh-CN" && !strings.Contains(rendered, "项目分析") {
				t.Fatal("Chinese header missing")
			}
			// 项目表每个项目占一行，不再插入额外空行。
			start := strings.Index(rendered, "api")
			end := strings.Index(rendered[start:], "Storage summary")
			if language == "en" && !detail && end > 0 && strings.Contains(rendered[start:start+end], "\n\n2") {
				t.Fatal("extra project spacing remains")
			}
		}
	}
}
