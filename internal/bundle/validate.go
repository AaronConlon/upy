// 从解压后的 bundle 目录读取 upy.yaml 并校验为 Manifest (兼容 deploy.yaml)
package bundle

import (
	"os"
	"path/filepath"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/i18n"
)

// ReadManifest 读取解压目录根部的清单文件并解析校验 (优先 upy.yaml, 兼容 deploy.yaml)
func ReadManifest(extractedDir string) (*config.Manifest, error) {
	for _, name := range []string{"upy.yaml", "manifest.yaml", "bundle.yaml", "deploy.yaml"} {
		path := filepath.Join(extractedDir, name)
		data, err := os.ReadFile(path)
		if err == nil {
			return config.ParseManifest(data)
		}
	}
	return nil, i18n.Error("bundle 根目录缺少 upy.yaml 清单文件")
}
