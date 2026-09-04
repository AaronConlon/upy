// 交互选择器: TTY 用 survey 库 (专业终端控件), 非 TTY 回退为编号选择
package ui

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"golang.org/x/term"
)

// SelectOption 选项
type SelectOption struct {
	Label string
	Hint  string
}

// Select 交互选择 (返回选中项索引)
func Select(message string, options []SelectOption) (int, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) {
		return surveySelect(message, options)
	}
	return numberedSelect(message, options)
}

// surveySelect 使用 survey 库渲染方向键选择
func surveySelect(message string, options []SelectOption) (int, error) {
	labels := make([]string, len(options))
	hints := make(map[string]string, len(options))
	for i, o := range options {
		labels[i] = o.Label
		if o.Hint != "" {
			hints[o.Label] = o.Hint
		}
	}

	prompt := &survey.Select{
		Message: message,
		Options: labels,
		Description: func(value string, _ int) string {
			return hints[value]
		},
	}

	var picked string
	if err := survey.AskOne(prompt, &picked); err != nil {
		return 0, fmt.Errorf("已取消")
	}
	for i, o := range options {
		if o.Label == picked {
			return i, nil
		}
	}
	return 0, fmt.Errorf("无效选择")
}

func numberedSelect(message string, options []SelectOption) (int, error) {
	fmt.Fprintln(os.Stderr, message)
	for i, o := range options {
		hint := ""
		if o.Hint != "" {
			hint = " - " + o.Hint
		}
		fmt.Fprintf(os.Stderr, "  [%d] %s%s\n", i+1, o.Label, hint)
	}
	fmt.Fprint(os.Stderr, "输入序号: ")
	var n int
	if _, err := fmt.Scanln(&n); err != nil {
		return 0, fmt.Errorf("无效选择")
	}
	if n < 1 || n > len(options) {
		return 0, fmt.Errorf("无效选择: %d", n)
	}
	return n - 1, nil
}
