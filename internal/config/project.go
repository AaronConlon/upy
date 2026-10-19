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

// ProjectService 服务器端系统服务配置
type ProjectService struct {
	User  string `yaml:"user"`
	Group string `yaml:"group"`
	Unit  string `yaml:"unit"`
}

// ProjectBackup 服务器端备份配置 (可选)
type ProjectBackup struct {
	Script    string `yaml:"script"`    // 自定义脚本路径 (相对项目根目录，缺省自动探测)
	Dir       string `yaml:"dir"`       // 备份产物归档目录 (相对项目根目录，默认 backups)
	Retention int    `yaml:"retention"` // 自动轮转保留的最大备份份数 (0 表示不自动清理)
	Timeout   string `yaml:"timeout"`   // 备份超时时间 (默认 15m)
}

// ProjectConfig 服务器端项目配置
type ProjectConfig struct {
	Name     string          `yaml:"name"`
	Release  *ProjectRelease `yaml:"release"`
	Releases ProjectReleases `yaml:"releases"`
	Service  *ProjectService `yaml:"service"` // 可选: 允许在服务器端显式配置运行账号与 unit
	Backup   *ProjectBackup  `yaml:"backup"`  // 可选: 备份脚本与轮转配置
	Notify   *ProjectNotify  `yaml:"notify"`  // 可选: 项目级部署/备份通知配置
}

// ActiveBarks 返回当前项目生效的 Bark 渠道
func (c *ProjectConfig) ActiveBarks() []UserBark {
	if c == nil {
		return nil
	}
	return c.Notify.ActiveBarks()
}

// ActiveWebhooks 返回当前项目生效的 Webhook 渠道
func (c *ProjectConfig) ActiveWebhooks() []ProjectWebhook {
	if c == nil {
		return nil
	}
	return c.Notify.ActiveWebhooks()
}

// ReadProjectConfig 读取并校验服务器端 deploy.yaml
func ReadProjectConfig(root string) (*ProjectConfig, error) {
	path := filepath.Join(root, "deploy.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("在 %s 中找不到 deploy.yaml（请在项目目录中运行 upy，或用 --root 指定目录）", root)
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
