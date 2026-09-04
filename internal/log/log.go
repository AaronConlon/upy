// Package log 提供步骤化日志输出 + token 脱敏 + 颜色辅助（中文 CLI）
package log

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var secrets []string

// RegisterSecret 注册需要脱敏的敏感值（GitHub token）
func RegisterSecret(v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return
	}
	for _, s := range secrets {
		if s == v {
			return
		}
	}
	secrets = append(secrets, v)
}

// Redact 将已知敏感值替换为 ***
func Redact(s string) string {
	out := s
	for _, sec := range secrets {
		if sec == "" {
			continue
		}
		out = strings.ReplaceAll(out, sec, "***")
	}
	return out
}

const (
	dim     = "\x1b[2m"
	green   = "\x1b[32m"
	cyan    = "\x1b[36m"
	bright  = "\x1b[96m"
	bold    = "\x1b[1m"
	red     = "\x1b[31m"
	yellow  = "\x1b[33m"
	magenta = "\x1b[35m"
	reset   = "\x1b[0m"
)

// UseColor 是否输出 ANSI 颜色（仅 TTY）
var UseColor = term.IsTerminal(int(os.Stderr.Fd()))

func paint(code, s string) string {
	if !UseColor {
		return s
	}
	return code + s + reset
}

// Green / Cyan / Sky / Dim / Bold / Yellow / Red / Magenta 颜色辅助
func Green(s string) string   { return paint(green, s) }
func Cyan(s string) string    { return paint(cyan, s) }
func Sky(s string) string     { return paint(bright, s) }
func Dim(s string) string     { return paint(dim, s) }
func Bold(s string) string    { return paint(bold, s) }
func Yellow(s string) string  { return paint(yellow, s) }
func Red(s string) string     { return paint(red, s) }
func Magenta(s string) string { return paint(magenta, s) }

// Path 突出路径: 天蓝加粗
func Path(s string) string { return paint(bright+bold, s) }

func emit(s string) {
	fmt.Fprintln(os.Stderr, Redact(s))
}

func Step(msg string)     { emit("  " + paint(dim, "->") + " " + msg) }
func Progress(msg string) { emit("  " + paint(cyan, "...") + " " + msg) }
func Ok(msg string)       { emit("  " + paint(green, "✓") + " " + msg) }
func Info(msg string)     { emit("  " + paint(dim, "-") + " " + msg) }
func Warn(msg string)     { emit("  " + paint(yellow, "!") + " " + msg) }
func Fail(msg string)     { emit("  " + paint(red, "错误: ") + msg) }
