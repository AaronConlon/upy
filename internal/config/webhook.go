package config

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// ProjectWebhook 一条项目专属的 Webhook 推送配置 (每一个项目独立配置)
type ProjectWebhook struct {
	URL     string            `yaml:"url"`
	Secret  string            `yaml:"secret,omitempty"`  // 可选 HMAC-SHA256 签名密钥
	Headers map[string]string `yaml:"headers,omitempty"` // 可选自定义请求头
	Enabled *bool             `yaml:"enabled,omitempty"` // 单个 webhook 启用开关
}

func (w ProjectWebhook) active() bool {
	if w.Enabled != nil && !*w.Enabled {
		return false
	}
	return strings.TrimSpace(w.URL) != ""
}

// WebhookChannels 支持单 URL 字符串、URL 列表、单对象及对象列表等多形态 YAML
type WebhookChannels []ProjectWebhook

func (wc *WebhookChannels) UnmarshalYAML(value *yaml.Node) error {
	var list []ProjectWebhook
	switch value.Kind {
	case yaml.ScalarNode:
		if val := strings.TrimSpace(value.Value); val != "" {
			list = append(list, ProjectWebhook{URL: val})
		}
	case yaml.SequenceNode:
		for _, child := range value.Content {
			if child.Kind == yaml.ScalarNode {
				if val := strings.TrimSpace(child.Value); val != "" {
					list = append(list, ProjectWebhook{URL: val})
				}
			} else if child.Kind == yaml.MappingNode {
				var item ProjectWebhook
				if err := child.Decode(&item); err == nil && strings.TrimSpace(item.URL) != "" {
					list = append(list, item)
				}
			}
		}
	case yaml.MappingNode:
		var helper struct {
			URL      string            `yaml:"url"`
			URLs     []string          `yaml:"urls"`
			Webhooks []ProjectWebhook  `yaml:"webhooks"`
			Secret   string            `yaml:"secret"`
			Headers  map[string]string `yaml:"headers"`
			Enabled  *bool             `yaml:"enabled"`
		}
		if err := value.Decode(&helper); err == nil {
			if len(helper.Webhooks) > 0 {
				for _, item := range helper.Webhooks {
					if strings.TrimSpace(item.URL) != "" {
						merged := item
						if strings.TrimSpace(merged.Secret) == "" {
							merged.Secret = helper.Secret
						}
						if merged.Enabled == nil {
							merged.Enabled = helper.Enabled
						}
						list = append(list, merged)
					}
				}
			} else if len(helper.URLs) > 0 {
				for _, u := range helper.URLs {
					if strings.TrimSpace(u) != "" {
						list = append(list, ProjectWebhook{
							URL:     strings.TrimSpace(u),
							Secret:  helper.Secret,
							Headers: helper.Headers,
							Enabled: helper.Enabled,
						})
					}
				}
			} else if strings.TrimSpace(helper.URL) != "" {
				list = append(list, ProjectWebhook{
					URL:     strings.TrimSpace(helper.URL),
					Secret:  helper.Secret,
					Headers: helper.Headers,
					Enabled: helper.Enabled,
				})
			}
		}
	}
	*wc = list
	return nil
}

// Active 返回生效且启用的有效 Webhook 列表
func (wc WebhookChannels) Active() []ProjectWebhook {
	out := make([]ProjectWebhook, 0, len(wc))
	for _, w := range wc {
		if w.active() {
			out = append(out, w)
		}
	}
	return out
}
