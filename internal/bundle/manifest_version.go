// 从 zip 包内直接读取 deploy.yaml 的 version 字段 (用于缓存版本校验)
package bundle

import (
	"archive/zip"
	"io"

	"github.com/AaronConlon/upy/internal/config"
)

// ManifestVersionFromZip 读取 zip 根部 deploy.yaml 的 version 字段
// 读取失败 (zip 损坏 / 缺少 deploy.yaml / manifest 无效) 时返回空串
func ManifestVersionFromZip(zipPath string) string {
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
	return ""
}
