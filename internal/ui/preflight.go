package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AaronConlon/upy/internal/bundle"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/platform"
)

// PreflightItem 预检条目
type PreflightItem struct {
	Label   string
	Value   string
	Status  string // "ok", "warn", "info"
	Warning string
}

// PreflightReport 部署前业务预检报告
type PreflightReport struct {
	ProjectName    string
	TargetVersion  string
	PackageVersion string
	IsMismatch     bool
	Items          []PreflightItem
	Warnings       []string
}

// CheckPreflight 执行业务校验与元数据一致性核对
func CheckPreflight(extractedDir, projectRoot string, m *config.Manifest, pcfg *config.ProjectConfig) *PreflightReport {
	rep := &PreflightReport{
		ProjectName:   m.Name,
		TargetVersion: m.Version,
	}

	// 1. 项目名称核对
	nameStatus := "ok"
	nameNote := ""
	if pcfg != nil && pcfg.Name != "" && pcfg.Name != m.Name {
		nameStatus = "warn"
		nameNote = fmt.Sprintf(i18n.T("deploy.yaml 配置为: %s"), pcfg.Name)
		rep.Warnings = append(rep.Warnings, fmt.Sprintf(i18n.T("项目名称核对提示: 清单名称为 [%s]，但部署配置名称为 [%s]"), m.Name, pcfg.Name))
	}
	rep.Items = append(rep.Items, PreflightItem{
		Label:   i18n.T("项目名称"),
		Value:   m.Name,
		Status:  nameStatus,
		Warning: nameNote,
	})

	// 2. 目标版本与 package.json 交叉核对 (核心业务校验)
	pkgVer := bundle.ReadPackageJSONVersion(extractedDir)
	rep.PackageVersion = pkgVer
	verStatus := "ok"
	verNote := ""
	if pkgVer != "" {
		normManifest := strings.TrimPrefix(strings.TrimSpace(m.Version), "v")
		normPkg := strings.TrimPrefix(strings.TrimSpace(pkgVer), "v")
		if normManifest != normPkg {
			rep.IsMismatch = true
			verStatus = "warn"
			verNote = fmt.Sprintf("⚠️ (package.json: %s · %s)", pkgVer, i18n.T("版本不一致"))
			rep.Warnings = append(rep.Warnings, fmt.Sprintf(i18n.T("版本核对提示: 清单版本为 [%s]，但内部 package.json 声明版本为 [%s]！"), m.Version, pkgVer))
		} else {
			verNote = fmt.Sprintf("(package.json: %s · %s)", pkgVer, i18n.T("一致"))
		}
	}
	rep.Items = append(rep.Items, PreflightItem{
		Label:   i18n.T("目标版本"),
		Value:   m.Version,
		Status:  verStatus,
		Warning: verNote,
	})

	// 3. 部署形态类型
	rep.Items = append(rep.Items, PreflightItem{
		Label:  i18n.T("部署类型"),
		Value:  DescribeType(m),
		Status: "ok",
	})

	// 4. 平台兼容性
	targetOS, targetArch := "", ""
	if m.Target != nil {
		targetOS = m.Target.OS
		targetArch = m.Target.Arch
	}
	platStatus := "ok"
	platVal := fmt.Sprintf("%s/%s", platform.CurrentOS(), platform.CurrentArch())
	platNote := ""
	if targetOS != "" || targetArch != "" {
		isCompat := platform.IsCompatible(targetOS, targetArch)
		if !isCompat {
			platStatus = "warn"
			platNote = fmt.Sprintf(i18n.T("目标声明: %s/%s · 与主机不兼容"), targetOS, targetArch)
		} else {
			platNote = fmt.Sprintf(i18n.T("目标声明: %s/%s · 兼容"), targetOS, targetArch)
		}
	}
	rep.Items = append(rep.Items, PreflightItem{
		Label:   i18n.T("主机架构"),
		Value:   platVal,
		Status:  platStatus,
		Warning: platNote,
	})

	// 5. 持久化与环境变量文件检测
	envStatus := "info"
	envVal := i18n.T("使用默认环境")
	envPaths := []string{
		filepath.Join(projectRoot, "shared", ".env"),
		filepath.Join(projectRoot, "shared", "service.env"),
		filepath.Join(projectRoot, "storage", ".env"),
		filepath.Join(extractedDir, ".env"),
	}
	var detected []string
	for _, ep := range envPaths {
		if _, err := os.Stat(ep); err == nil {
			rel, _ := filepath.Rel(projectRoot, ep)
			if rel == "" || strings.HasPrefix(rel, "..") {
				rel = filepath.Base(ep)
			}
			detected = append(detected, rel)
		}
	}
	if len(detected) > 0 {
		envStatus = "ok"
		envVal = strings.Join(detected, ", ") + " " + i18n.T("(已就绪)")
	}
	rep.Items = append(rep.Items, PreflightItem{
		Label:  i18n.T("环境配置"),
		Value:  envVal,
		Status: envStatus,
	})

	// 6. 健康检查配置
	if m.Healthcheck != nil && strings.TrimSpace(m.Healthcheck.URL) != "" {
		rep.Items = append(rep.Items, PreflightItem{
			Label:  i18n.T("健康探针"),
			Value:  fmt.Sprintf("%s (timeout: %v)", m.Healthcheck.URL, m.Healthcheck.Timeout),
			Status: "ok",
		})
	}

	return rep
}

// PrintPreflightCard 输出 ANSI 业务预检卡片
func PrintPreflightCard(rep *PreflightReport) {
	width := 56
	title := i18n.T("📋 业务预检与配置校验")
	topBorder := log.Cyan("┌─ ") + log.Bold(title) + " " + log.Cyan(strings.Repeat("─", max(2, width-8-len([]rune(title))*2))) + log.Cyan("┐")
	bottomBorder := log.Cyan("└" + strings.Repeat("─", width) + "┘")

	os.Stderr.WriteString("\n" + topBorder + "\n")
	for _, it := range rep.Items {
		icon := log.Green("✓")
		if it.Status == "warn" {
			icon = log.Yellow("!")
		} else if it.Status == "info" {
			icon = log.Sky("ℹ")
		}

		line := fmt.Sprintf("  %s %s: %s", icon, log.Dim(it.Label), log.Bold(it.Value))
		if it.Warning != "" {
			if it.Status == "warn" {
				line += " " + log.Yellow(it.Warning)
			} else {
				line += " " + log.Dim(it.Warning)
			}
		}
		os.Stderr.WriteString(log.Cyan("│") + line + "\n")
	}
	os.Stderr.WriteString(bottomBorder + "\n\n")
}

// PrintStep 打印标准化阶段火箭指示符 (ANSI 特效)
func PrintStep(step, total int, title string) {
	tag := log.Cyan(log.Bold(fmt.Sprintf("🚀 [%d/%d]", step, total)))
	os.Stderr.WriteString(fmt.Sprintf("\n%s %s\n", tag, log.Bold(title)))
}

// PrintSuccessBanner 打印部署成功 ANSI 庆祝横幅
func PrintSuccessBanner(name, version, elapsed string) {
	line := log.Green(strings.Repeat("━", 50))
	msg := fmt.Sprintf(i18n.T("  🎉 部署成功: %s %s (耗时: %s)"), log.Green(log.Bold(name)), log.Bold(version), log.Green(elapsed))
	os.Stderr.WriteString("\n" + line + "\n" + msg + "\n" + line + "\n")
}

// PrintRollbackAlert 打印触发自动回滚的高能警示
func PrintRollbackAlert(targetVersion, reason string) {
	alertTag := log.Red(log.Bold("🚨 " + i18n.T("正在触发自动回滚")))
	os.Stderr.WriteString(fmt.Sprintf("\n%s: %s [%s]...\n", alertTag, i18n.T("恢复至上一稳定版本"), log.Green(log.Bold(targetVersion))))
	if reason != "" {
		os.Stderr.WriteString(fmt.Sprintf("   %s: %s\n\n", log.Dim(i18n.T("原因")), log.Yellow(reason)))
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
