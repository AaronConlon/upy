package analysis

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/cleaner"
	"github.com/AaronConlon/upy/internal/log"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func stringWidth(s string) int {
	clean := stripANSI(s)
	w := 0
	for _, r := range clean {
		if r >= 0x1100 &&
			(r <= 0x115f ||
				r == 0x2329 || r == 0x232a ||
				(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
				(r >= 0xac00 && r <= 0xd7a3) ||
				(r >= 0xf900 && r <= 0xfaff) ||
				(r >= 0xfe10 && r <= 0xfe19) ||
				(r >= 0xfe30 && r <= 0xfe6f) ||
				(r >= 0xff00 && r <= 0xff60) ||
				(r >= 0xffe0 && r <= 0xffe6) ||
				(r >= 0x1f300 && r <= 0x1faff) || // Emojis and Symbols
				(r >= 0x20000 && r <= 0x2fffd) ||
				(r >= 0x30000 && r <= 0x3fffd)) {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func padRight(s string, width int) string {
	w := stringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// RenderBar 渲染基准条形图，以最大项目为 100%
func RenderBar(percent float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(math.Round((percent / 100.0) * float64(width)))
	if percent > 0 && filled == 0 {
		filled = 1 // 保留极微小刻度，提示有数据
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	barStr := strings.Repeat("█", filled) + strings.Repeat("░", empty)

	var coloredBar string
	switch {
	case percent >= 70.0:
		coloredBar = log.Cyan(barStr)
	case percent >= 30.0:
		coloredBar = log.Green(barStr)
	default:
		coloredBar = log.Dim(barStr)
	}

	return fmt.Sprintf("[%s] %5.1f%%", coloredBar, percent)
}

// SectionHeader 生成带统一宽度的装饰标题分割线
func SectionHeader(title string, totalWidth int) string {
	if totalWidth <= 0 {
		totalWidth = 72
	}
	w := stringWidth(title)
	remain := totalWidth - w - 4
	if remain < 4 {
		remain = 4
	}
	return log.Cyan("━ " + title + " " + strings.Repeat("━", remain))
}

// RenderTerminal 渲染漂亮的终端分析报告
func RenderTerminal(report *AnalysisReport, showDetail bool) string {
	var sb strings.Builder

	sb.WriteString(log.Bold("upy analysis — 客户端项目与磁盘分析") + "\n\n")

	// 1. 最近一次部署动态 Spotlight
	sb.WriteString(SectionHeader("最近一次部署动态", 72) + "\n")
	if report.LatestDeploy == nil {
		sb.WriteString("  💡 " + log.Dim("暂无部署记录（可在项目根目录运行 upy release 或 upy deploy 进行首次部署）") + "\n\n")
	} else {
		ld := report.LatestDeploy
		ver := ld.CurrentVersion
		if ver == "" {
			ver = "-"
		}
		sb.WriteString(fmt.Sprintf("  📦 项目: %s (%s)\n", log.Bold(ld.Name), log.Cyan(ver)))

		typeDesc := formatTypeDesc(ld.Type)
		if ld.UnitName != "" {
			typeDesc += " · " + ld.UnitName
		}
		sb.WriteString(fmt.Sprintf("  🧩 类型: %s\n", typeDesc))

		now := time.Now()
		relTime := FormatRelativeTime(ld.DeployedAt, now)
		timeDetail := "-"
		if ld.DeployedAt != nil {
			timeDetail = ld.DeployedAt.Local().Format("2006-01-02 15:04:05")
		}
		sb.WriteString(fmt.Sprintf("  ⏱  时间: %s (%s)\n", log.Bold(relTime), log.Dim(timeDetail)))

		statusDesc := formatStatusDesc(ld.Status)
		sb.WriteString(fmt.Sprintf("  🟢 状态: %s\n\n", statusDesc))
	}

	// 2. 项目分析 (状态 · 空间基准对比 · 类型 · 版本号)
	sb.WriteString(SectionHeader("项目分析 (状态 · 空间基准对比 · 类型 · 版本号)", 72) + "\n")
	if len(report.Projects) == 0 {
		sb.WriteString("  " + log.Dim("本机暂未发现通过 upy 纳管的项目。") + "\n\n")
	} else {
		// 计算项目名称的最大宽度
		nameColWidth := 16
		for _, p := range report.Projects {
			w := stringWidth(p.Name)
			if w > nameColWidth {
				nameColWidth = w
			}
		}
		if nameColWidth > 26 {
			nameColWidth = 26
		}

		headerLine := fmt.Sprintf("%s %s %s %s %s %s %s",
			padRight("#", 3),
			padRight("项目名称", nameColWidth+2),
			padRight("状态", 14),
			padRight("类型", 8),
			padRight("当前版本", 10),
			padRight("磁盘占用", 11),
			"基准对比 (Max)",
		)
		sb.WriteString(log.Dim(headerLine) + "\n")

		for i, p := range report.Projects {
			idxStr := padRight(fmt.Sprintf("%d", i+1), 3)

			displayName := p.Name
			if stringWidth(displayName) > nameColWidth {
				// 截断超长名称
				displayName = displayName[:nameColWidth-2] + ".."
			}
			nameStr := padRight(displayName, nameColWidth+2)
			statusStr := padRight(formatTableStatus(p.Status), 14)
			typeStr := padRight(p.Type, 8)

			verStr := p.CurrentVersion
			if verStr == "" {
				verStr = "-"
			}
			verStr = padRight(verStr, 10)

			sizeStr := cleaner.FormatBytes(p.TotalBytes)
			if p.IsMissingRoot {
				sizeStr = "目录丢失"
			}
			sizeStr = padRight(sizeStr, 11)

			barStr := RenderBar(p.PercentOfMax, 20)

			line := fmt.Sprintf("%s %s %s %s %s %s %s",
				idxStr,
				log.Bold(nameStr),
				statusStr,
				typeStr,
				log.Cyan(verStr),
				sizeStr,
				barStr,
			)
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	// 3. 细分明细视图 (--detail 模式)
	if showDetail && len(report.Projects) > 0 {
		sb.WriteString(SectionHeader("各项目内部存储分布明细 (--detail)", 72) + "\n")
		for i, p := range report.Projects {
			sb.WriteString(fmt.Sprintf("%d  %s (%s / %s) — %s [%.1f%%]\n",
				i+1,
				log.Bold(p.Name),
				p.Type,
				log.Cyan(p.CurrentVersion),
				cleaner.FormatBytes(p.TotalBytes),
				p.PercentOfMax,
			))

			// releases
			relNote := ""
			if p.OldReleaseBytes > 0 {
				relNote = fmt.Sprintf(" (活跃: %s / 旧版: %s %s)",
					cleaner.FormatBytes(p.ActiveBytes),
					cleaner.FormatBytes(p.OldReleaseBytes),
					log.Dim("[可使用 upy clean 释放]"),
				)
			} else if p.ActiveBytes > 0 {
				relNote = fmt.Sprintf(" (活跃: %s)", cleaner.FormatBytes(p.ActiveBytes))
			}
			sb.WriteString(fmt.Sprintf("   ├── releases/  %9s%s\n", cleaner.FormatBytes(p.ReleasesBytes), relNote))

			// bundles
			bundleNote := ""
			if p.BundlesBytes > 0 {
				bundleNote = " " + log.Dim("[Bundle 缓存，可清理]")
			}
			sb.WriteString(fmt.Sprintf("   ├── bundles/   %9s%s\n", cleaner.FormatBytes(p.BundlesBytes), bundleNote))

			// backups
			sb.WriteString(fmt.Sprintf("   ├── backups/   %9s\n", cleaner.FormatBytes(p.BackupsBytes)))

			// shared
			sb.WriteString(fmt.Sprintf("   ├── shared/    %9s\n", cleaner.FormatBytes(p.SharedBytes)))

			// other
			sb.WriteString(fmt.Sprintf("   └── other/     %9s\n\n", cleaner.FormatBytes(p.OtherBytes)))
		}
	}

	// 4. 存储汇总统计
	sb.WriteString(SectionHeader("存储汇总统计", 72) + "\n")
	sb.WriteString(fmt.Sprintf("  📊 纳管项目: %d 个\n", report.TotalProjects))

	avgBytes := int64(0)
	if report.TotalProjects > 0 {
		avgBytes = report.TotalBytes / int64(report.TotalProjects)
	}
	sb.WriteString(fmt.Sprintf("  💾 累计占用: %s (平均每个项目 %s)\n",
		log.Bold(cleaner.FormatBytes(report.TotalBytes)),
		cleaner.FormatBytes(avgBytes),
	))

	if report.TotalProjects > 0 && report.MaxBytes > 0 {
		maxPercent := float64(0)
		if report.TotalBytes > 0 {
			maxPercent = float64(report.MaxBytes) / float64(report.TotalBytes) * 100.0
		}
		sb.WriteString(fmt.Sprintf("  👑 空间首位: %s (%s, 占全机 upy 空间的 %.1f%%)\n",
			log.Bold(report.MaxProjectName),
			log.Cyan(cleaner.FormatBytes(report.MaxBytes)),
			maxPercent,
		))
	}

	if !showDetail {
		sb.WriteString("  💡 " + log.Dim("提示: 可使用 upy analysis --detail 查看各子目录详细空间分布") + "\n")
	}

	return sb.String()
}

// RenderJSON 将分析报告序列化为格式化 JSON 字符串
func RenderJSON(report *AnalysisReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func formatTypeDesc(t string) string {
	switch strings.ToLower(t) {
	case "system":
		return "Systemd 系统服务 (system)"
	case "docker":
		return "Docker 容器应用 (docker)"
	case "static":
		return "SPA 静态站点 (static)"
	default:
		return fmt.Sprintf("通用应用 (%s)", t)
	}
}

func formatStatusDesc(st string) string {
	switch strings.ToLower(st) {
	case "active":
		return log.Green("正常运行 (active)")
	case "degraded":
		return log.Yellow("降级运行 (degraded)")
	case "failed":
		return log.Red("异常退出 (failed)")
	case "missing":
		return log.Yellow("目录丢失 (missing)")
	case "undeployed":
		return log.Dim("尚未部署 (undeployed)")
	default:
		return st
	}
}

func formatTableStatus(st string) string {
	switch strings.ToLower(st) {
	case "active":
		return log.Green("🟢 active")
	case "degraded":
		return log.Yellow("🟡 degraded")
	case "failed":
		return log.Red("🔴 failed")
	case "missing":
		return log.Yellow("⚠️ missing")
	case "undeployed":
		return log.Dim("⚪ undeploy")
	default:
		return st
	}
}
