package server

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// RenderAnsiToHTML 将终端 ANSI 颜色转义字符解析为安全的带颜色的 HTML 标签
func RenderAnsiToHTML(raw string) string {
	escaped := html.EscapeString(raw)

	// 状态机替换
	openSpans := 0
	out := ansiRegex.ReplaceAllStringFunc(escaped, func(m string) string {
		sub := ansiRegex.FindStringSubmatch(m)
		if len(sub) < 2 || sub[1] == "0" || sub[1] == "" {
			res := strings.Repeat("</span>", openSpans)
			openSpans = 0
			return res
		}

		codes := strings.Split(sub[1], ";")
		var styles []string
		for _, c := range codes {
			switch c {
			case "1":
				styles = append(styles, "font-weight:bold")
			case "2":
				styles = append(styles, "opacity:0.65")
			case "31": // Red
				styles = append(styles, "color:#f85149")
			case "32": // Green
				styles = append(styles, "color:#3fb950")
			case "33": // Yellow
				styles = append(styles, "color:#d29922")
			case "34": // Blue
				styles = append(styles, "color:#58a6ff")
			case "35": // Magenta
				styles = append(styles, "color:#bc8cff")
			case "36", "96": // Cyan / Bright Cyan
				styles = append(styles, "color:#38bdf8")
			}
		}

		if len(styles) > 0 {
			openSpans++
			return fmt.Sprintf(`<span style="%s">`, strings.Join(styles, ";"))
		}
		return ""
	})

	if openSpans > 0 {
		out += strings.Repeat("</span>", openSpans)
	}
	return out
}
