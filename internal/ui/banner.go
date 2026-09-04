// 项目信息横幅: release/bundle 开始时打印 项目名/目录/类型/当前版本 (彩色 + emoji)
package ui

import (
	"os"
	"strings"

	"github.com/AaronConlon/uply/internal/config"
	"github.com/AaronConlon/uply/internal/log"
	"github.com/AaronConlon/uply/internal/state"
)

// BannerOptions 横幅选项
type BannerOptions struct {
	Type    string // 部署类型描述, 如 "Docker 容器 (container)"
	ZipPath string // bundle 模式: 传入 zip 路径推断类型
}

// DescribeType 描述部署类型
func DescribeType(m *config.Manifest) string {
	if m.Type == "static" {
		return "静态站点 (static)"
	}
	if m.Type == "docker" {
		mode := "container"
		if m.Docker != nil && m.Docker.Mode == "compose" {
			mode = "compose"
		}
		label := "Docker 容器"
		if mode == "compose" {
			label = "Docker Compose"
		}
		return label + " (" + mode + ")"
	}
	return m.Type
}

// PrintProjectBanner 打印项目横幅
func PrintProjectBanner(root string, opts BannerOptions) {
	name := "未知项目"
	if cfg, err := config.ReadProjectConfig(root); err == nil && cfg.Name != "" {
		name = cfg.Name
	}

	bannerType := opts.Type
	if bannerType == "" && opts.ZipPath != "" {
		if t := manifestTypeFromZip(opts.ZipPath); t != "" {
			bannerType = t
		}
	}

	current := "未部署"
	if st, err := state.Read(root); err == nil && st.CurrentVersion != "" {
		current = st.CurrentVersion
	}

	line := log.Cyan(strings.Repeat("━", 44))
	os.Stderr.WriteString(line + "\n")
	os.Stderr.WriteString("  📦 项目: " + log.Green(log.Bold(name)) + "\n")
	os.Stderr.WriteString("  📁 目录: " + log.Path(root) + "\n")
	if bannerType != "" {
		os.Stderr.WriteString("  🧩 类型: " + log.Sky(bannerType) + "\n")
	}
	if current == "未部署" {
		os.Stderr.WriteString("  🏷 当前版本: " + log.Yellow(current) + "\n")
	} else {
		os.Stderr.WriteString("  🏷 当前版本: " + log.Green(current) + "\n")
	}
	os.Stderr.WriteString(line + "\n")
}
