// 服务器项目配置 (cwd 或 --root 目录下的 deploy.yaml)
// 字段: name / release{provider, repository, asset} / releases{keep}
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectRelease 描述 bundle 来源
type ProjectRelease struct {
	Provider   string `yaml:"provider"`
	Repository string `yaml:"repository"` // owner/repo
	Asset      string `yaml:"asset"`      // 默认 bundle.zip
}

// ProjectReleases 保留策略 (清理逻辑目前只保留当前版本, keep 仅作配置兼容)
type ProjectReleases struct {
	Keep int `yaml:"keep"`
}

// ProjectConfig 服务器端项目配置
type ProjectConfig struct {
	Name     string          `yaml:"name"`
	Release  *ProjectRelease `yaml:"release"`
	Releases ProjectReleases `yaml:"releases"`
}

// ReadProjectConfig 读取并校验服务器端 deploy.yaml
func ReadProjectConfig(root string) (*ProjectConfig, error) {
	path := filepath.Join(root, "deploy.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("在 %s 中找不到 deploy.yaml（请在项目目录中运行 uply，或用 --root 指定目录）", root)
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("deploy.yaml 无效: %v", err)
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return nil, fmt.Errorf("deploy.yaml 无效: name 为必填且不能为空")
	}
	if cfg.Release == nil {
		return nil, fmt.Errorf("deploy.yaml 无效: 缺少 release 配置块 (provider, repository, asset)")
	}
	if cfg.Release.Provider != "github" {
		return nil, fmt.Errorf("deploy.yaml 无效: release.provider 仅支持 'github'，当前为 '%s'", cfg.Release.Provider)
	}
	parts := strings.Split(cfg.Release.Repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("deploy.yaml 无效: release.repository 格式应为 'owner/repo'")
	}
	if strings.TrimSpace(cfg.Release.Asset) == "" {
		cfg.Release.Asset = "bundle.zip"
	}
	if cfg.Releases.Keep <= 0 {
		cfg.Releases.Keep = 1
	}
	return &cfg, nil
}
