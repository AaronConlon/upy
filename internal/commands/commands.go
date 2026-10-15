// 公共辅助
package commands

import (
	"strings"

	"github.com/AaronConlon/upy/internal/log"
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
	return log.HumanSize(bytes)
}
