package ui

import (
	"archive/zip"
	"io"

	"github.com/AaronConlon/upy/internal/config"
)

// manifestTypeFromZip 从 zip 中读取 deploy.yaml 推断部署类型 (失败返回空串)
func manifestTypeFromZip(zipPath string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ""
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != "deploy.yaml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return ""
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return ""
		}
		m, err := config.ParseManifest(data)
		if err != nil {
			return ""
		}
		return DescribeType(m)
	}
	return ""
}
