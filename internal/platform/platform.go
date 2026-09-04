// 当前运行平台描述 + manifest.target 兼容性判断
package platform

import (
	"runtime"
	"strings"
)

// CurrentOS 返回当前系统: darwin | linux
func CurrentOS() string {
	if runtime.GOOS == "darwin" {
		return "darwin"
	}
	return "linux"
}

// CurrentArch 返回当前架构: x64 | arm64
func CurrentArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "x64"
}

// NormalizeArch 归一化架构名
func NormalizeArch(a string) string {
	switch strings.ToLower(a) {
	case "x64", "amd64":
		return "x64"
	case "arm64", "aarch64":
		return "arm64"
	default:
		return strings.ToLower(a)
	}
}

// IsCompatible 判断 manifest.target 与当前平台是否兼容 (target 为空视为兼容)
func IsCompatible(targetOS, targetArch string) bool {
	if targetOS != "" && targetOS != CurrentOS() {
		return false
	}
	if targetArch != "" && NormalizeArch(targetArch) != CurrentArch() {
		return false
	}
	return true
}
