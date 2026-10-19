// 从 zip 包内直接读取清单文件 (upy.yaml / deploy.yaml) 的 version 字段 (用于缓存版本校验)
package bundle

import (
	"archive/zip"
	"io"

	"github.com/AaronConlon/upy/internal/config"
)

// ManifestVersionFromZip 读取 zip 根部 upy.yaml (或 deploy.yaml) 的 version 字段
// 读取失败 (zip 损坏 / 缺少清单 / manifest 无效) 时返回空串
func ManifestVersionFromZip(zipPath string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ""
	}
	defer r.Close()

	var fallbackFile *zip.File
	for _, f := range r.File {
		if f.Name == "upy.yaml" || f.Name == "manifest.yaml" || f.Name == "bundle.yaml" {
			return readVersionFromZipFile(f)
		}
		if f.Name == "deploy.yaml" {
			fallbackFile = f
		}
	}
	if fallbackFile != nil {
		return readVersionFromZipFile(fallbackFile)
	}
	return ""
}

func readVersionFromZipFile(f *zip.File) string {
	rc, err := f.Open()
	if err != nil {
		return ""
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return ""
	}
	m, err := config.ParseManifest(data)
	if err != nil {
		return ""
	}
	return m.Version
}
