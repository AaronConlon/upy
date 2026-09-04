// 极简 semver 比较: 只支持 X.Y.Z (可选前缀 v), 忽略 pre-release 后缀 "-..."
package semver

import (
	"regexp"
	"strconv"
	"strings"
)

type ver struct {
	major, minor, patch int
	pre                 string
}

var re = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-(.+))?$`)

func parse(v string) *ver {
	s := strings.TrimPrefix(strings.TrimSpace(v), "v")
	m := re.FindStringSubmatch(s)
	if m == nil {
		return nil
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return &ver{major: major, minor: minor, patch: patch, pre: m[4]}
}

// Newer 判断 remote 是否比 current 新 (解析失败返回 false)
func Newer(remote, current string) bool {
	a := parse(remote)
	b := parse(current)
	if a == nil || b == nil {
		return false
	}
	if a.major != b.major {
		return a.major > b.major
	}
	if a.minor != b.minor {
		return a.minor > b.minor
	}
	if a.patch != b.patch {
		return a.patch > b.patch
	}
	// 同版本: 无 pre 视为正式版 (比有 pre 的新)
	if a.pre == "" && b.pre != "" {
		return true
	}
	return false
}
