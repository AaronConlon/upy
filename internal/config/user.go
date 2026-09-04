// 用户级配置: ~/.upy/config.yaml
// 放 Bark / webhook 这类跨项目通知渠道, 不进项目目录, 也不进 bundle。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// UserBark 一条 Bark 推送配置。支持多条, 失败/成功会逐条发送。
type UserBark struct {
	Name      string `yaml:"name"`
	ServerURL string `yaml:"serverUrl"` // 如 https://api.day.app
	Token     string `yaml:"token"`
	DeviceKey string `yaml:"deviceKey"` // 兼容字段, 与 token 二选一
	Group     string `yaml:"group"`
	Sound     string `yaml:"sound"`
	Icon      string `yaml:"icon"`
	Level     string `yaml:"level"`
	Enabled   *bool  `yaml:"enabled"`
}

// UserNotify 通知总开关。渠道先做 Bark, 结构预留 webhook / 飞书 / 邮件。
type UserNotify struct {
	Enabled bool       `yaml:"enabled"`
	Bark    []UserBark `yaml:"bark"`
}

// UserGitHub GitHub 访问配置。token 优先于环境变量 DEPLOY_GITHUB_TOKEN。
type UserGitHub struct {
	Token string `yaml:"token"`
}

// UserConfig ~/.upy/config.yaml
type UserConfig struct {
	GitHub UserGitHub `yaml:"github"`
	Notify UserNotify `yaml:"notify"`
}

func (b UserBark) active() bool {
	if b.Enabled != nil && !*b.Enabled {
		return false
	}
	return strings.TrimSpace(b.Key()) != ""
}

// Key 返回 Bark device key / token
func (b UserBark) Key() string {
	if k := strings.TrimSpace(b.Token); k != "" {
		return k
	}
	return strings.TrimSpace(b.DeviceKey)
}

// Endpoint 规范化 Bark 服务地址, 去掉末尾斜杠
func (b UserBark) Endpoint() string {
	s := strings.TrimSpace(b.ServerURL)
	if s == "" {
		s = "https://api.day.app"
	}
	return strings.TrimRight(s, "/")
}

// Label 日志里用的短名
func (b UserBark) Label() string {
	if n := strings.TrimSpace(b.Name); n != "" {
		return n
	}
	return "bark"
}

// UserConfigPath 返回配置文件路径。UPLY_CONFIG 可覆盖。
func UserConfigPath() string {
	if p := strings.TrimSpace(os.Getenv("UPY_CONFIG")); p != "" {
		return p
	}
	if p := strings.TrimSpace(os.Getenv("UPLY_CONFIG")); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	newPath := filepath.Join(home, ".upy", "config.yaml")
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	oldPath := filepath.Join(home, ".uply", "config.yaml")
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath
	}
	return newPath
}

// LoadUserConfig 读取用户配置。文件不存在时返回空配置, 不报错。
func LoadUserConfig() (*UserConfig, error) {
	path := UserConfigPath()
	if path == "" {
		return &UserConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &UserConfig{}, nil
		}
		return nil, fmt.Errorf("读取用户配置失败: %s (%v)", path, err)
	}
	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("用户配置 YAML 无效: %s (%v)", path, err)
	}
	return &cfg, nil
}

// ActiveBarks 返回已启用且带 key 的 Bark 渠道
func (c *UserConfig) ActiveBarks() []UserBark {
	if c == nil {
		return nil
	}
	out := make([]UserBark, 0, len(c.Notify.Bark))
	for _, b := range c.Notify.Bark {
		if b.active() {
			out = append(out, b)
		}
	}
	return out
}

// GitHubToken 返回配置文件里的 GitHub token
func (c *UserConfig) GitHubToken() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.GitHub.Token)
}

// LookupGitHubToken 优先读用户配置 github.token, 其次环境变量 DEPLOY_GITHUB_TOKEN。
// 配置文件损坏时返回错误; 两者都空时返回空字符串。
func LookupGitHubToken() (string, error) {
	cfg, err := LoadUserConfig()
	if err != nil {
		return "", err
	}
	if t := cfg.GitHubToken(); t != "" {
		return t, nil
	}
	return strings.TrimSpace(os.Getenv("DEPLOY_GITHUB_TOKEN")), nil
}

// ResolveGitHubToken 取 GitHub token; 配置和环境变量都没有时返回可操作错误。
func ResolveGitHubToken() (string, error) {
	t, err := LookupGitHubToken()
	if err != nil {
		return "", err
	}
	if t == "" {
		path := UserConfigPath()
		if path == "" {
			path = "~/.upy/config.yaml"
		}
		return "", fmt.Errorf("未配置 GitHub token。请在 %s 写入 github.token，或设置环境变量 DEPLOY_GITHUB_TOKEN。", path)
	}
	return t, nil
}
