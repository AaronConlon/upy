package ui

import (
	"archive/zip"
	"io"

	"github.com/AaronConlon/upy/internal/config"
)

// manifestTypeFromZip 从 zip 中读取清单文件推断部署类型 (优先 upy.yaml, 兼容 deploy.yaml)
func manifestTypeFromZip(zipPath string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ""
	}
	defer r.Close()

	var fallbackFile *zip.File
	for _, f := range r.File {
		if f.Name == "upy.yaml" || f.Name == "manifest.yaml" || f.Name == "bundle.yaml" {
			return readTypeFromZipFile(f)
		}
		if f.Name == "deploy.yaml" {
			fallbackFile = f
		}
	}
	if fallbackFile != nil {
		return readTypeFromZipFile(fallbackFile)
	}
	return ""
}

func readTypeFromZipFile(f *zip.File) string {
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
