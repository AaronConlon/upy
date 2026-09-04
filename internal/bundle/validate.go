// 从解压后的 bundle 目录读取 deploy.yaml 并校验为 Manifest
package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AaronConlon/upy/internal/config"
)

// ReadManifest 读取解压目录根部的 deploy.yaml 并解析校验
func ReadManifest(extractedDir string) (*config.Manifest, error) {
	path := filepath.Join(extractedDir, "deploy.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("bundle 根目录缺少 deploy.yaml 清单文件")
	}
	return config.ParseManifest(data)
}
