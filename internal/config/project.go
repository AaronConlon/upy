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

// ProjectBundles 压缩包保留策略 (默认保留最多 3 个，支持 0~100)
type ProjectBundles struct {
	Keep *int `yaml:"keep"`
}

// UnmarshalYAML 支持 bundles: 3 直接写数字，或 bundles: { keep: 3 }
func (pb *ProjectBundles) UnmarshalYAML(value *yaml.Node) error {
	var intVal int
	if err := value.Decode(&intVal); err == nil {
		pb.Keep = &intVal
		return nil
	}
	type rawProjectBundles ProjectBundles
	var raw rawProjectBundles
	if err := value.Decode(&raw); err != nil {
		return err
	}
	pb.Keep = raw.Keep
	return nil
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

// ProjectInboundWebhook 项目部署 Webhook 接收端配置
type ProjectInboundWebhook struct {
	Secret string `yaml:"secret,omitempty"` // 用于 HMAC-SHA256 签名校验或项目专属 Token
}

// ProjectConfig 服务器端项目配置
type ProjectConfig struct {
	Name     string                 `yaml:"name"`
	Release  *ProjectRelease        `yaml:"release"`
	Releases ProjectReleases        `yaml:"releases"`
	Bundles  *ProjectBundles        `yaml:"bundles"` // 可选: bundle 压缩包保留策略 (默认最多保留 3 个，范围 0~100)
	Bundle   *ProjectBundles        `yaml:"bundle"`  // 兼容单数别名 bundle
	Service  *ProjectService        `yaml:"service"` // 可选: 允许在服务器端显式配置运行账号与 unit
	Backup   *ProjectBackup         `yaml:"backup"`  // 可选: 备份脚本与轮转配置
	Notify   *ProjectNotify         `yaml:"notify"`  // 可选: 项目级部署/备份通知配置
	Webhook  *ProjectInboundWebhook `yaml:"webhook,omitempty"`
}

// InboundSecret 返回当前项目的入站 Webhook 密钥
func (c *ProjectConfig) InboundSecret() string {
	if c == nil || c.Webhook == nil {
		return ""
	}
	return strings.TrimSpace(c.Webhook.Secret)
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

// BundlesKeep 返回当前项目生效的 bundle 压缩包保留数量 (默认 3, 范围 0~100)
func (c *ProjectConfig) BundlesKeep() int {
	if c == nil {
		return 3
	}
	b := c.Bundles
	if b == nil {
		b = c.Bundle
	}
	if b == nil || b.Keep == nil {
		return 3
	}
	return *b.Keep
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

	// 校验并设置 bundles 保留策略 (默认保留最多 3 个，支持 0~100)
	bundlesCfg := cfg.Bundles
	if bundlesCfg == nil {
		bundlesCfg = cfg.Bundle
	}
	keep := 3
	if bundlesCfg != nil && bundlesCfg.Keep != nil {
		keep = *bundlesCfg.Keep
		if keep < 0 || keep > 100 {
			return nil, fmt.Errorf("deploy.yaml 无效: bundles.keep 必须在 0 到 100 之间 (当前为 %d)", keep)
		}
	}
	cfg.Bundles = &ProjectBundles{Keep: &keep}

	return &cfg, nil
}
