// Package log 提供步骤化日志输出 + token 脱敏 + 颜色辅助（中文 CLI）
package log

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/term"
)

var (
	secretsMu sync.RWMutex
	secrets   []string

	outMu     sync.RWMutex
	outWriter io.Writer = os.Stderr
)

// SetOutput 设置日志输出目标 (默认为 os.Stderr)
func SetOutput(w io.Writer) {
	outMu.Lock()
	defer outMu.Unlock()
	if w == nil {
		outWriter = os.Stderr
		return
	}
	outWriter = w
}

// ResetOutput 恢复默认输出目标为 os.Stderr
func ResetOutput() {
	SetOutput(os.Stderr)
}

// RegisterSecret 注册需要脱敏的敏感值（GitHub token）
func RegisterSecret(v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return
	}
	secretsMu.Lock()
	defer secretsMu.Unlock()
	for _, s := range secrets {
		if s == v {
			return
		}
	}
	secrets = append(secrets, v)
}

// Redact 将已知敏感值替换为 ***
func Redact(s string) string {
	secretsMu.RLock()
	defer secretsMu.RUnlock()
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
	outMu.RLock()
	w := outWriter
	outMu.RUnlock()
	fmt.Fprintln(w, Redact(s))
}

func Step(msg string)     { emit("  " + paint(dim, "->") + " " + msg) }
func Progress(msg string) { emit("  " + paint(cyan, "...") + " " + msg) }
func Ok(msg string)       { emit("  " + paint(green, "✓") + " " + msg) }
func Info(msg string)     { emit("  " + paint(dim, "-") + " " + msg) }
func Warn(msg string)     { emit("  " + paint(yellow, "!") + " " + msg) }
func Fail(msg string)     { emit("  " + paint(red, "错误: ") + msg) }

// HumanSize 将字节大小格式化为易读的字符串 (如 1.2MB, 512KB)
func HumanSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return strconv.FormatInt(bytes, 10) + "B"
	case bytes < 1024*1024:
		return strconv.FormatFloat(float64(bytes)/1024, 'f', 1, 64) + "KB"
	default:
		return strconv.FormatFloat(float64(bytes)/1024/1024, 'f', 1, 64) + "MB"
	}
}
