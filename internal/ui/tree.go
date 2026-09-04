// 目录树渲染: 部署成功后打印项目结构, 用颜色区分 目录/文件/软链
package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AaronConlon/uply/internal/log"
)

const (
	branch = "├── "
	last   = "└── "
	pipe   = "│   "
	space  = "    "
)

// PrintProjectLayout 打印项目目录树
func PrintProjectLayout(root string) {
	os.Stderr.WriteString("  " + log.Cyan("project layout:") + "\n")
	lines := renderTree(root, 4, map[string]bool{"node_modules": true, ".git": true}, map[string]bool{"current": true, "state.json": true, "deploy.yaml": true})
	for _, l := range lines {
		os.Stderr.WriteString("    " + l + "\n")
	}
}

func renderTree(dir string, depth int, skipDirs, highlight map[string]bool) []string {
	root := filepath.Base(dir)
	var out []string
	out = append(out, log.Path(dir))
	walkTree(dir, "", depth, skipDirs, highlight, &out)
	_ = root
	return out
}

func walkTree(dir, prefix string, depth int, skipDirs, highlight map[string]bool, out *[]string) {
	if depth <= 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	// 目录优先, 再按字母序
	sort.Slice(entries, func(i, j int) bool {
		di, dj := entries[i].IsDir(), entries[j].IsDir()
		if di != dj {
			return di
		}
		return entries[i].Name() < entries[j].Name()
	})
	filtered := make([]os.DirEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && skipDirs[e.Name()] {
			continue
		}
		if !e.IsDir() && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		filtered = append(filtered, e)
	}
	for i, e := range filtered {
		isLast := i == len(filtered)-1
		conn := branch
		if isLast {
			conn = last
		}
		childPrefix := prefix + pipe
		if isLast {
			childPrefix = prefix + space
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(filepath.Join(dir, e.Name()))
			name := log.Green(e.Name())
			if highlight[e.Name()] {
				name = log.Bold(log.Green(e.Name()))
			}
			*out = append(*out, log.Dim(prefix+conn)+name+log.Dim(" -> "+target))
			continue
		}
		if e.IsDir() {
			*out = append(*out, log.Dim(prefix+conn)+log.Sky(e.Name())+log.Dim("/"))
			walkTree(filepath.Join(dir, e.Name()), childPrefix, depth-1, skipDirs, highlight, out)
			continue
		}
		if highlight[e.Name()] {
			*out = append(*out, log.Dim(prefix+conn)+log.Bold(log.Green(e.Name())))
		} else {
			*out = append(*out, log.Dim(prefix+conn)+log.Dim(e.Name()))
		}
	}
}
