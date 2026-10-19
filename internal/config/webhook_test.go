package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWebhookChannelsFormats(t *testing.T) {
	t.Run("SingleStringURL", func(t *testing.T) {
		input := `https://hooks.slack.com/services/xxx`
		var wc WebhookChannels
		if err := yaml.Unmarshal([]byte(input), &wc); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		active := wc.Active()
		if len(active) != 1 || active[0].URL != "https://hooks.slack.com/services/xxx" {
			t.Fatalf("unexpected active: %+v", active)
		}
	})

	t.Run("StringList", func(t *testing.T) {
		input := `
- "https://hook1.example.com"
- "https://hook2.example.com"
`
		var wc WebhookChannels
		if err := yaml.Unmarshal([]byte(input), &wc); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		active := wc.Active()
		if len(active) != 2 {
			t.Fatalf("expected 2 hooks, got %d", len(active))
		}
		if active[0].URL != "https://hook1.example.com" || active[1].URL != "https://hook2.example.com" {
			t.Fatalf("unexpected hooks: %+v", active)
		}
	})

	t.Run("SingleObjectWithSecretAndHeaders", func(t *testing.T) {
		input := `
url: "https://api.mycorp.internal/deploy-webhook"
secret: "my-secret-key"
headers:
  Authorization: "Bearer token123"
  X-Custom-Env: "production"
`
		var wc WebhookChannels
		if err := yaml.Unmarshal([]byte(input), &wc); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		active := wc.Active()
		if len(active) != 1 {
			t.Fatalf("expected 1 hook, got %d", len(active))
		}
		h := active[0]
		if h.URL != "https://api.mycorp.internal/deploy-webhook" || h.Secret != "my-secret-key" {
			t.Fatalf("unexpected hook: %+v", h)
		}
		if h.Headers["Authorization"] != "Bearer token123" || h.Headers["X-Custom-Env"] != "production" {
			t.Fatalf("unexpected headers: %+v", h.Headers)
		}
	})

	t.Run("ObjectListWithDisabled", func(t *testing.T) {
		input := `
- url: "https://hook1.example.com"
  secret: "sec1"
- url: "https://hook2.example.com"
  enabled: false
`
		var wc WebhookChannels
		if err := yaml.Unmarshal([]byte(input), &wc); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		active := wc.Active()
		if len(active) != 1 {
			t.Fatalf("expected 1 active hook, got %d", len(active))
		}
		if active[0].URL != "https://hook1.example.com" {
			t.Fatalf("unexpected active hook: %+v", active[0])
		}
	})

	t.Run("BatchURLsWithSharedSecret", func(t *testing.T) {
		input := `
secret: "shared-sec"
urls:
  - "https://hook1.example.com"
  - "https://hook2.example.com"
`
		var wc WebhookChannels
		if err := yaml.Unmarshal([]byte(input), &wc); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		active := wc.Active()
		if len(active) != 2 {
			t.Fatalf("expected 2 active hooks, got %d", len(active))
		}
		if active[0].Secret != "shared-sec" || active[1].Secret != "shared-sec" {
			t.Fatalf("expected inherited secret, got: %+v", active)
		}
	})
}
