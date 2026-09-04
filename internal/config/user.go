// 用户级配置: ~/.upy/config.yaml (兼容 ~/.uply/config.yaml)
// 放 Bark / webhook 这类跨项目通知渠道, 以及多组织 GitHub token
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
	Enabled   *bool  `yaml:"enabled,omitempty"`
}

// UserNotify 通知总开关。渠道先做 Bark, 结构预留 webhook / 飞书 / 邮件。
type UserNotify struct {
	Enabled bool       `yaml:"enabled"`
	Bark    []UserBark `yaml:"bark,omitempty"`
}

// UserGitHub GitHub 访问配置。支持单 token 及按组织/个人归属的多 token。
type UserGitHub struct {
	Token  string            `yaml:"token,omitempty"`  // 默认 / 兜底 token
	Tokens map[string]string `yaml:"tokens,omitempty"` // 按归属映射: WeiaiHealth-Software: ghp_xxx
}

// UserConfig ~/.upy/config.yaml
type UserConfig struct {
	GitHub   UserGitHub     `yaml:"github"`
	Notify   UserNotify     `yaml:"notify,omitempty"`
	Projects []LocalProject `yaml:"projects,omitempty"`
}

// LocalProject 一台机器上曾成功部署过的项目。Name 在注册表中唯一。
type LocalProject struct {
	Name string `yaml:"name"`
	Root string `yaml:"root"`
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

// DefaultUserConfigPath 默认用户配置保存路径: ~/.upy/config.yaml
func DefaultUserConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".upy/config.yaml"
	}
	return filepath.Join(home, ".upy", "config.yaml")
}

// UserConfigPath 返回当前生效的配置文件路径。优先 UPY_CONFIG，兜底 UPLY_CONFIG。
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
	// 若 ~/.upy/config.yaml 不存在但 ~/.uply/config.yaml 存在，自动兼容旧路径
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

// GitHubToken 返回配置文件里的默认 GitHub token
func (c *UserConfig) GitHubToken() string {
	if c == nil {
		return ""
	}
	if t := strings.TrimSpace(c.GitHub.Token); t != "" {
		return t
	}
	if len(c.GitHub.Tokens) > 0 {
		if t, ok := c.GitHub.Tokens["default"]; ok && strings.TrimSpace(t) != "" {
			return strings.TrimSpace(t)
		}
	}
	return ""
}

// GitHubTokenForOwner 按组织名/用户名精确或大小写不敏感匹配 token，未匹配时回退到默认 token
func (c *UserConfig) GitHubTokenForOwner(owner string) string {
	if c == nil {
		return ""
	}
	owner = strings.TrimSpace(owner)
	if owner != "" && len(c.GitHub.Tokens) > 0 {
		for k, v := range c.GitHub.Tokens {
			if strings.EqualFold(k, owner) && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return c.GitHubToken()
}

// ParseRepoOwner 辅助函数: 从 owner/repo 或 owner 中提取 owner
func ParseRepoOwner(repo string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	parts := strings.Split(repo, "/")
	return strings.TrimSpace(parts[0])
}

// LookupGitHubTokenForRepo 针对特定 repo/owner 查找 token:
// 1. 配置文件匹配 owner 的 tokens[owner]
// 2. 配置文件兜底默认 token
// 3. 环境变量 DEPLOY_GITHUB_TOKEN
func LookupGitHubTokenForRepo(repo string) (string, error) {
	cfg, err := LoadUserConfig()
	if err != nil {
		return "", err
	}
	owner := ParseRepoOwner(repo)
	if t := cfg.GitHubTokenForOwner(owner); t != "" {
		return t, nil
	}
	return strings.TrimSpace(os.Getenv("DEPLOY_GITHUB_TOKEN")), nil
}

// ResolveGitHubTokenForRepo 针对特定 repo/owner 取 token，均未配置时返回可操作错误提示
func ResolveGitHubTokenForRepo(repo string) (string, error) {
	t, err := LookupGitHubTokenForRepo(repo)
	if err != nil {
		return "", err
	}
	if t == "" {
		path := UserConfigPath()
		if path == "" {
			path = "~/.upy/config.yaml"
		}
		owner := ParseRepoOwner(repo)
		if owner != "" {
			return "", fmt.Errorf("未配置访问 %s 的 GitHub token。请在 %s 写入 github.tokens.%s，或通过 upy init %s <token> 添加。", owner, path, owner, owner)
		}
		return "", fmt.Errorf("未配置 GitHub token。请在 %s 写入 github.token，或通过 upy init <token> 添加。", path)
	}
	return t, nil
}

// LookupGitHubToken (兼容保留) 查询默认全局 token
func LookupGitHubToken() (string, error) {
	return LookupGitHubTokenForRepo("")
}

// ResolveGitHubToken (兼容保留) 获取默认全局 token
func ResolveGitHubToken() (string, error) {
	return ResolveGitHubTokenForRepo("")
}

// SaveGitHubToken 保存/更新 GitHub Token。owner 为空或 "default" 时保存为默认全局 token
func SaveGitHubToken(owner, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("token 不能为空")
	}

	path := UserConfigPath()
	if path == "" {
		path = DefaultUserConfigPath()
	}

	cfg, err := LoadUserConfig()
	if err != nil {
		cfg = &UserConfig{}
	}

	owner = strings.TrimSpace(owner)
	if owner == "" || strings.EqualFold(owner, "default") {
		cfg.GitHub.Token = token
	} else {
		if cfg.GitHub.Tokens == nil {
			cfg.GitHub.Tokens = make(map[string]string)
		}
		cfg.GitHub.Tokens[owner] = token
	}

	return saveUserConfig(cfg)
}

// ListLocalProjects 返回可用的本地项目。根目录丢失、不是目录或缺少 deploy.yaml 的记录会被自动清理。
func ListLocalProjects() ([]LocalProject, error) {
	cfg, err := LoadUserConfig()
	if err != nil {
		return nil, err
	}

	projects := make([]LocalProject, 0, len(cfg.Projects))
	seen := make(map[string]bool, len(cfg.Projects))
	changed := false
	for _, project := range cfg.Projects {
		name := strings.TrimSpace(project.Name)
		root := strings.TrimSpace(project.Root)
		if name == "" || root == "" {
			changed = true
			continue
		}
		absRoot, err := filepath.Abs(root)
		if err != nil || !isProjectRoot(absRoot) || seen[name] {
			changed = true
			continue
		}
		if absRoot != project.Root || name != project.Name {
			changed = true
		}
		seen[name] = true
		projects = append(projects, LocalProject{Name: name, Root: absRoot})
	}

	if changed {
		cfg.Projects = projects
		if _, err := saveUserConfig(cfg); err != nil {
			return nil, err
		}
	}
	return projects, nil
}

// SaveLocalProject 在 release 成功后保存项目名称及绝对根目录。同名或同根目录旧记录会被替换。
func SaveLocalProject(name, root string) error {
	name = strings.TrimSpace(name)
	root = strings.TrimSpace(root)
	absRoot, err := filepath.Abs(root)
	if err != nil || name == "" || !isProjectRoot(absRoot) {
		return fmt.Errorf("项目名称或根目录无效，无法保存本地项目")
	}

	cfg, err := LoadUserConfig()
	if err != nil {
		return err
	}
	projects, err := ListLocalProjects()
	if err != nil {
		return err
	}
	cfg.Projects = append([]LocalProject{{Name: name, Root: absRoot}}, filterDifferentProject(projects, name, absRoot)...)
	_, err = saveUserConfig(cfg)
	return err
}

func filterDifferentProject(projects []LocalProject, name, root string) []LocalProject {
	filtered := make([]LocalProject, 0, len(projects))
	for _, project := range projects {
		if project.Name != name && project.Root != root {
			filtered = append(filtered, project)
		}
	}
	return filtered
}

func isProjectRoot(root string) bool {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return false
	}
	manifest, err := os.Stat(filepath.Join(root, "deploy.yaml"))
	return err == nil && manifest.Mode().IsRegular()
}

func saveUserConfig(cfg *UserConfig) (string, error) {
	path := UserConfigPath()
	if path == "" {
		path = DefaultUserConfigPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("无法创建配置目录 %s: %v", filepath.Dir(path), err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("写入配置文件失败 %s: %v", path, err)
	}
	return path, nil
}
