// 交互选择器: TTY 用 survey 库 (专业终端控件), 非 TTY 回退为编号选择
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"
)

// SelectOption 选项
type SelectOption struct {
	Label string
	Hint  string
	Badge string // 例如: "(当前运行 / ACTIVE)", "(上一版本 / ROLLBACK)"
	Warn  string // 例如: "⚠️ (package: 1.0.3)"
}

// Select 交互选择 (返回选中项索引)
func Select(message string, options []SelectOption) (int, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) {
		return surveySelect(message, options)
	}
	return numberedSelect(message, options)
}

// formatOptionLine 将选项格式化为带高亮徽章与警示的单行文本
func formatOptionLine(o SelectOption) string {
	var parts []string
	parts = append(parts, o.Label)
	if o.Warn != "" {
		parts = append(parts, log.Yellow(o.Warn))
	}
	if o.Badge != "" {
		parts = append(parts, log.Green(o.Badge))
	}
	return strings.Join(parts, " ")
}

// surveySelect 使用 survey 库渲染方向键选择
func surveySelect(message string, options []SelectOption) (int, error) {
	labels := make([]string, len(options))
	lookup := make(map[string]int, len(options))
	for i, o := range options {
		line := formatOptionLine(o)
		labels[i] = line
		lookup[line] = i
	}

	prompt := &survey.Select{
		Message:  message,
		Options:  labels,
		VimMode:  true,
		PageSize: 12,
		Help:     i18n.T("↑/↓ 或 j/k 选择，回车确认，输入可搜索过滤"),
		Description: func(value string, index int) string {
			if index >= 0 && index < len(options) {
				return options[index].Hint
			}
			if idx, ok := lookup[value]; ok {
				return options[idx].Hint
			}
			return ""
		},
	}

	opts := []survey.AskOpt{
		survey.WithIcons(func(icons *survey.IconSet) {
			icons.SelectFocus = survey.Icon{Text: "❯", Format: "cyan+b"}
			icons.Question = survey.Icon{Text: "📋", Format: "cyan"}
		}),
	}

	var picked string
	if err := survey.AskOne(prompt, &picked, opts...); err != nil {
		return 0, i18n.Error("已取消")
	}

	idx, ok := lookup[picked]
	if !ok {
		// 备用兜底循环
		for i, l := range labels {
			if l == picked {
				idx = i
				ok = true
				break
			}
		}
	}
	if !ok {
		return 0, i18n.Error("无效选择")
	}

	chosen := options[idx]
	if chosen.Warn != "" {
		log.Warn(chosen.Warn)
	}

	return idx, nil
}

func numberedSelect(message string, options []SelectOption) (int, error) {
	fmt.Fprintln(os.Stderr, log.Cyan("📋 ")+message)
	for i, o := range options {
		line := formatOptionLine(o)
		hint := ""
		if o.Hint != "" {
			hint = log.Dim(" - " + o.Hint)
		}
		fmt.Fprintf(os.Stderr, "  [%d] %s%s\n", i+1, line, hint)
	}
	fmt.Fprint(os.Stderr, i18n.T("输入序号: "))
	var n int
	if _, err := fmt.Scanln(&n); err != nil {
		return 0, i18n.Error("无效选择")
	}
	if n < 1 || n > len(options) {
		return 0, fmt.Errorf(i18n.T("无效选择: %d"), n)
	}

	chosen := options[n-1]
	if chosen.Warn != "" {
		log.Warn(chosen.Warn)
	}

	return n - 1, nil
}

// Confirm 提示用户确认操作 (y/N)
func Confirm(message string, defaultVal bool) (bool, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) {
		var confirmed bool
		prompt := &survey.Confirm{
			Message: message,
			Default: defaultVal,
		}
		if err := survey.AskOne(prompt, &confirmed); err != nil {
			return false, i18n.Error("已取消")
		}
		return confirmed, nil
	}

	// 非 TTY 环境回退
	defStr := "y/N"
	if defaultVal {
		defStr = "Y/n"
	}
	fmt.Fprintf(os.Stderr, "%s [%s]: ", message, defStr)
	var input string
	_, _ = fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return defaultVal, nil
	}
	return input == "y" || input == "yes", nil
}
