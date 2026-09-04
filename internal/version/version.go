// Package version 保存构建时注入的版本号
// 通过 ldflags 注入: -X github.com/AaronConlon/uply/internal/version.Value=v0.1.11
package version

import "strings"

// Value 当前版本 (构建时注入, 可带或不带 v 前缀)
var Value = "dev"

// String 返回 "v<版本>", 保证单 v 前缀
func String() string {
	return "v" + strings.TrimPrefix(Value, "v")
}
