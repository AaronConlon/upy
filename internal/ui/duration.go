package ui

import (
	"fmt"
	"time"
)

// FormatDuration 格式化耗时展示:
// - 小于 60s: 以秒展示并保留1位小数, 如 0.5s, 12.3s
// - 超过 60s (且小于 60m): 视为分, 如 1m, 1m15s
// - 超过 60m: 以时开头, 如 1h, 1h2m, 1h15m30s
func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	sec := d.Seconds()
	if sec < 60 {
		return fmt.Sprintf("%.1fs", sec)
	}
	totalSec := int(d.Round(time.Second).Seconds())
	if totalSec < 3600 {
		m := totalSec / 60
		s := totalSec % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%ds", m, s)
	}
	h := totalSec / 3600
	remSec := totalSec % 3600
	m := remSec / 60
	s := remSec % 60
	if s == 0 && m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	if s == 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dh%dm%ds", h, m, s)
}
