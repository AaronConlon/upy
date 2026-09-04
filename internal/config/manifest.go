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

// Manifest bundle manifest
type Manifest struct {
	Name        string               `yaml:"name"`
	Version     string               `yaml:"version"`
	Type        string               `yaml:"type"` // static | docker
	Target      *ManifestTarget      `yaml:"target"`
	Static      *ManifestStatic      `yaml:"static"`
	Docker      *ManifestDocker      `yaml:"docker"`
	Healthcheck *ManifestHealthcheck `yaml:"healthcheck"`
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
		if m.Docker != nil {
			return nil, fmt.Errorf("bundle 清单无效: static 类型不允许携带 docker 配置")
		}
		if m.Static == nil || strings.TrimSpace(m.Static.Directory) == "" {
			m.Static = &ManifestStatic{Directory: "dist"}
		}
	case "docker":
		if m.Static != nil {
			return nil, fmt.Errorf("bundle 清单无效: docker 类型不允许携带 static 配置")
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
	default:
		return nil, fmt.Errorf("bundle 清单无效: type 必须为 'static' 或 'docker'，当前为 '%s'", m.Type)
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
