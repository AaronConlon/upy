// 安全解压 ZIP, 防 ZIP Slip (每个文件 resolve 后必须仍在目标目录内, 禁止 ../)
package bundle

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SafeExtract 解压 zipPath 到 destDir, 逐条校验路径防 ZIP Slip
func SafeExtract(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("无法读取压缩包: %s (%v)", zipPath, err)
	}
	defer r.Close()

	absDest, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absDest, 0o755); err != nil {
		return err
	}

	for _, f := range r.File {
		name := f.Name
		absPath := filepath.Join(absDest, filepath.FromSlash(name))
		rel, err := filepath.Rel(absDest, absPath)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return fmt.Errorf("检测到 zip slip 攻击: 条目 '%s' 试图逃逸目标目录，已拒绝", name)
		}
		// 进一步: 任意路径段为 ".." 也拒绝
		for _, seg := range strings.Split(filepath.ToSlash(rel), "/") {
			if seg == ".." {
				return fmt.Errorf("检测到 zip slip 攻击: 条目 '%s' 试图逃逸目标目录，已拒绝", name)
			}
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(absPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(absPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}
