// 公共辅助
package commands

import (
	"strconv"
	"strings"
)

const selfRepo = "AaronConlon/upy"

func stripV(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func forceSuffix(force bool) string {
	if force {
		return "（--force）"
	}
	return ""
}

func humanSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return strconv.FormatInt(bytes, 10) + "B"
	case bytes < 1024*1024:
		return strconv.FormatFloat(float64(bytes)/1024, 'f', 1, 64) + "KB"
	default:
		return strconv.FormatFloat(float64(bytes)/1024/1024, 'f', 1, 64) + "MB"
	}
}
