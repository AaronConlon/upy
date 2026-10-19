// upy analysis [项目名称] [--detail] [--json] [--sort <size|time|name>] [--root <dir>]
package commands

import (
	"fmt"
	"strings"

	"github.com/AaronConlon/upy/internal/analysis"
)

// AnalysisArgs analysis 命令执行参数
type AnalysisArgs struct {
	ProjectName string
	Root        string
	Detail      bool
	JSON        bool
	Sort        string
}

// Analysis 执行 analysis 命令
func Analysis(args AnalysisArgs) error {
	projects, err := analysis.CollectAll(args.Root)
	if err != nil {
		return fmt.Errorf("收集项目信息失败: %w", err)
	}

	targetName := strings.TrimSpace(args.ProjectName)
	if targetName != "" {
		var matched []analysis.ProjectAnalysis
		for _, p := range projects {
			if p.Name == targetName {
				matched = append(matched, p)
			}
		}

		if len(matched) == 0 {
			// 尝试将 targetName 视为本地路径探测
			pa := analysis.CollectProject(targetName, targetName)
			if !pa.IsMissingRoot {
				matched = append(matched, pa)
			}
		}

		if len(matched) == 0 {
			return fmt.Errorf("未找到项目: %s（可在项目目录运行 upy deploy 完成登记，或使用 upy analysis 查看全量列表）", targetName)
		}
		projects = matched
		// 单项目分析时，默认开启详细子目录分解
		args.Detail = true
	}

	report := analysis.Analyze(projects, args.Sort)

	if args.JSON {
		jsonStr, err := analysis.RenderJSON(report)
		if err != nil {
			return fmt.Errorf("序列化分析报告失败: %w", err)
		}
		fmt.Println(jsonStr)
		return nil
	}

	termStr := analysis.RenderTerminal(report, args.Detail)
	fmt.Print(termStr)
	return nil
}
