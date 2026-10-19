// Bundle manifest (bundle 内 deploy.yaml) 读取与校验
package config

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestHealthcheck 健康检查配置
type ManifestHealthcheck struct {
	URL     string `yaml:"url"`
	Timeout any    `yaml:"timeout"` // 数字秒 或 "30s"/"1m" 字符串
}

// ManifestStatic 静态部署配置
type ManifestStatic struct {
	Directory string `yaml:"directory"`
}

// ManifestDocker Docker 部署配置
type ManifestDocker struct {
	Mode        string   `yaml:"mode"`        // container | compose
	Dockerfile  string   `yaml:"dockerfile"`  // container 模式, 默认 Dockerfile
	ComposeFile string   `yaml:"composeFile"` // compose 模式, 默认 docker-compose.yml
	Port        string   `yaml:"port"`        // container 模式, 形如 "3000:3000"
	Network     string   `yaml:"network"`     // container 模式, 加入的 docker 网络
	Volumes     []string `yaml:"volumes"`     // 额外 -v 挂载
}

// ManifestTarget 目标平台声明
type ManifestTarget struct {
	OS   string `yaml:"os"`
	Arch string `yaml:"arch"`
}

// ManifestSystem system 服务配置
type ManifestSystem struct {
	Command          any                  `yaml:"command"`    // 启动命令: string 或 []string
	WorkingDirectory string               `yaml:"workingDir"` // 可选工作目录, 默认 releases/<version>
	Healthcheck      *ManifestHealthcheck `yaml:"healthcheck"`
}

// Manifest bundle manifest
type Manifest struct {
	Name        string               `yaml:"name"`
	Version     string               `yaml:"version"`
	Type        string               `yaml:"type"` // static | docker | system (或 service)
	Target      *ManifestTarget      `yaml:"target"`
	Static      *ManifestStatic      `yaml:"static"`
	Docker      *ManifestDocker      `yaml:"docker"`
	System      *ManifestSystem      `yaml:"system"`
	Healthcheck *ManifestHealthcheck `yaml:"healthcheck"`
	Backup      *ProjectBackup       `yaml:"backup"`
	Notify      *ProjectNotify       `yaml:"notify,omitempty"`
}

// ActiveBarks 返回 bundle 清单中生效的 Bark 渠道
func (m *Manifest) ActiveBarks() []UserBark {
	if m == nil {
		return nil
	}
	return m.Notify.ActiveBarks()
}

// ParseManifest 解析 bundle 内 deploy.yaml 内容并校验
func ParseManifest(content []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(content, &m); err != nil {
		return nil, fmt.Errorf("bundle 清单 YAML 无效: %v", err)
	}
	if strings.TrimSpace(m.Name) == "" {
		return nil, fmt.Errorf("bundle 清单无效: name 为必填")
	}
	if strings.TrimSpace(m.Version) == "" {
		return nil, fmt.Errorf("bundle 清单无效: version 为必填")
	}
	switch m.Type {
	case "static":
		if m.Docker != nil || m.System != nil {
			return nil, fmt.Errorf("bundle 清单无效: static 类型不允许携带 docker 或 system 配置")
		}
		if m.Static == nil || strings.TrimSpace(m.Static.Directory) == "" {
			m.Static = &ManifestStatic{Directory: "dist"}
		}
	case "docker":
		if m.Static != nil || m.System != nil {
			return nil, fmt.Errorf("bundle 清单无效: docker 类型不允许携带 static 或 system 配置")
		}
		if m.Docker == nil {
			return nil, fmt.Errorf("bundle 清单无效: type 为 docker 但缺少 docker 配置")
		}
		switch m.Docker.Mode {
		case "container":
			if m.Docker.Dockerfile == "" {
				m.Docker.Dockerfile = "Dockerfile"
			}
		case "compose":
			if m.Docker.ComposeFile == "" {
				m.Docker.ComposeFile = "docker-compose.yml"
			}
		default:
			return nil, fmt.Errorf("bundle 清单无效: docker.mode 必须为 'container' 或 'compose'，当前为 '%s'", m.Docker.Mode)
		}
	case "system", "service":
		if m.Static != nil || m.Docker != nil {
			return nil, fmt.Errorf("bundle 清单无效: %s 类型不允许携带 static 或 docker 配置", m.Type)
		}
		if m.System == nil {
			return nil, fmt.Errorf("bundle 清单无效: type 为 %s 但缺少 system 配置", m.Type)
		}
		m.Type = "system"
		cmdList, err := ParseCommand(m.System.Command)
		if err != nil {
			return nil, fmt.Errorf("bundle 清单无效: %v", err)
		}
		if len(cmdList) == 0 {
			return nil, fmt.Errorf("bundle 清单无效: system.command 不能为空")
		}
		if m.Healthcheck == nil && m.System.Healthcheck != nil {
			m.Healthcheck = m.System.Healthcheck
		}
	default:
		return nil, fmt.Errorf("bundle 清单无效: type 必须为 'static'、'docker' 或 'system'，当前为 '%s'", m.Type)
	}
	// 健康检查 timeout 校验
	if m.Healthcheck != nil {
		if strings.TrimSpace(m.Healthcheck.URL) == "" {
			return nil, fmt.Errorf("bundle 清单无效: healthcheck.url 为必填")
		}
		sec, err := ParseTimeout(m.Healthcheck.Timeout)
		if err != nil {
			return nil, err
		}
		m.Healthcheck.Timeout = sec
	}
	return &m, nil
}

// ParseCommand 解析 system.command 配置为参数列表，并严格校验控制字符
func ParseCommand(v any) ([]string, error) {
	var rawList []string
	switch t := v.(type) {
	case string:
		rawList = strings.Fields(strings.TrimSpace(t))
	case []string:
		rawList = t
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok {
				rawList = append(rawList, s)
			}
		}
	default:
		return nil, fmt.Errorf("system.command 必须是字符串或字符串列表")
	}

	if len(rawList) == 0 {
		return nil, fmt.Errorf("system.command 不能为空")
	}

	exec := strings.TrimSpace(rawList[0])
	if exec == "" {
		return nil, fmt.Errorf("可执行文件路径不能为空")
	}
	if strings.ContainsAny(exec, "\r\n\x00") {
		return nil, fmt.Errorf("system.command 参数包含非法控制字符或换行符: %q", exec)
	}

	res := []string{exec}
	// 后续参数原样保留 (SYS-13: 包含空字符串 '' 与含首尾空白的字符串 '  padded  ')
	for i := 1; i < len(rawList); i++ {
		arg := rawList[i]
		if strings.ContainsAny(arg, "\r\n\x00") {
			return nil, fmt.Errorf("system.command 参数包含非法控制字符或换行符: %q", arg)
		}
		res = append(res, arg)
	}
	return res, nil
}

// EscapeSystemdArg 对单个 systemd ExecStart 参数进行转义与防护
func EscapeSystemdArg(arg string) (string, error) {
	if strings.ContainsAny(arg, "\r\n\x00") {
		return "", fmt.Errorf("参数包含非法控制字符或换行符: %q", arg)
	}
	if arg == "" {
		return `""`, nil
	}
	// 转义 % 为 %%，避免触发 systemd specifier
	escaped := strings.ReplaceAll(arg, "%", "%%")
	// 转义 $ 为 $$，避免触发 systemd 环境变量自动展开
	escaped = strings.ReplaceAll(escaped, "$", "$$")
	// 转义反斜杠和双引号
	escaped = strings.ReplaceAll(escaped, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)

	// 若包含空格、制表符或双引号，外层包裹双引号
	if strings.ContainsAny(escaped, " \t\"") {
		return `"` + escaped + `"`, nil
	}
	return escaped, nil
}

// ParseTimeout 解析 healthcheck.timeout: 数字秒 或 "30s" / "1m" 字符串; 默认 30s
func ParseTimeout(v any) (int, error) {
	switch t := v.(type) {
	case nil:
		return 30, nil
	case int:
		if t > 0 {
			return t, nil
		}
	case int64:
		if t > 0 {
			return int(t), nil
		}
	case float64:
		if t > 0 {
			return int(t), nil
		}
	case string:
		s := strings.TrimSpace(t)
		mult := 1
		body := s
		if strings.HasSuffix(s, "m") {
			mult, body = 60, strings.TrimSuffix(s, "m")
		} else if strings.HasSuffix(s, "h") {
			mult, body = 3600, strings.TrimSuffix(s, "h")
		} else {
			body = strings.TrimSuffix(s, "s")
		}
		n, err := strconv.Atoi(strings.TrimSpace(body))
		if err == nil && n > 0 {
			return n * mult, nil
		}
	}
	return 0, fmt.Errorf("healthcheck.timeout 必须是正数秒或时长字符串，如 '30s'")
}
