package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUserConfigMissing(t *testing.T) {
	t.Setenv("UPLY_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ActiveBarks()) != 0 {
		t.Fatalf("期望无渠道, 得到 %d", len(cfg.ActiveBarks()))
	}
}

func TestLoadUserConfigBarks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := []byte(`notify:
  bark:
    - name: phone
      serverUrl: https://api.day.app
      token: key-a
    - name: disabled
      token: key-b
      enabled: false
    - name: empty
      token: "  "
    - name: watch
      deviceKey: key-c
      group: uply.watch
`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UPLY_CONFIG", path)
	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.ActiveBarks()
	if len(got) != 2 {
		t.Fatalf("期望 2 条启用渠道, 得到 %d", len(got))
	}
	if got[0].Label() != "phone" || got[0].Key() != "key-a" {
		t.Fatalf("第一条不对: %+v", got[0])
	}
	if got[1].Label() != "watch" || got[1].Key() != "key-c" {
		t.Fatalf("第二条应对 deviceKey: %+v", got[1])
	}
	if got[0].Endpoint() != "https://api.day.app" {
		t.Fatalf("endpoint 不对: %s", got[0].Endpoint())
	}
}

func TestLookupGitHubTokenPrefersConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("github: {token: from-file}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UPLY_CONFIG", path)
	t.Setenv("DEPLOY_GITHUB_TOKEN", "from-env")
	got, err := LookupGitHubToken()
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-file" {
		t.Fatalf("期望配置文件优先, 得到 %q", got)
	}
}

func TestLookupGitHubTokenFallsBackToEnv(t *testing.T) {
	t.Setenv("UPLY_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("DEPLOY_GITHUB_TOKEN", "from-env")
	got, err := LookupGitHubToken()
	if err != nil {
		t.Fatal(err)
	}
	if got != "from-env" {
		t.Fatalf("期望回退环境变量, 得到 %q", got)
	}
}

func TestResolveGitHubTokenMissing(t *testing.T) {
	t.Setenv("UPLY_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("DEPLOY_GITHUB_TOKEN", "")
	_, err := ResolveGitHubToken()
	if err == nil {
		t.Fatal("期望报错")
	}
}
