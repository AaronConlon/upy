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
      group: upy.watch
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

func TestLookupGitHubTokenForOwner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := []byte(`github:
  token: default-token
  tokens:
    WeiaiHealth-Software: org-token
    AaronConlon: user-token
`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UPY_CONFIG", path)
	t.Setenv("DEPLOY_GITHUB_TOKEN", "env-token")

	// 匹配指定组织 (大小写不敏感)
	tok, err := LookupGitHubTokenForRepo("WeiaiHealth-Software/my-repo")
	if err != nil || tok != "org-token" {
		t.Fatalf("期望 org-token, 得到 %q (err: %v)", tok, err)
	}
	tokCase, err := LookupGitHubTokenForRepo("weiaihealth-software/lower-case-repo")
	if err != nil || tokCase != "org-token" {
		t.Fatalf("期望大小写不敏感匹配 org-token, 得到 %q (err: %v)", tokCase, err)
	}

	// 匹配指定个人用户
	tokUser, err := LookupGitHubTokenForRepo("AaronConlon/private-project")
	if err != nil || tokUser != "user-token" {
		t.Fatalf("期望 user-token, 得到 %q (err: %v)", tokUser, err)
	}

	// 未在 tokens 中配置的仓库，回退到 default-token
	tokOther, err := LookupGitHubTokenForRepo("OtherOrg/other-repo")
	if err != nil || tokOther != "default-token" {
		t.Fatalf("期望回退 default-token, 得到 %q (err: %v)", tokOther, err)
	}
}

func TestSaveGitHubToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("UPY_CONFIG", path)

	// 1. 保存全局 token
	savedPath, err := SaveGitHubToken("", "my-global-token")
	if err != nil || savedPath != path {
		t.Fatalf("保存全局 token 失败: %v", err)
	}

	tok, err := LookupGitHubTokenForRepo("any/repo")
	if err != nil || tok != "my-global-token" {
		t.Fatalf("期望 my-global-token, 得到 %q (err: %v)", tok, err)
	}

	// 2. 保存特定组织 token
	_, err = SaveGitHubToken("SpecialOrg", "special-token")
	if err != nil {
		t.Fatalf("保存组织 token 失败: %v", err)
	}

	// 验证组织 token 与全局 token 共存
	tokSpecial, err := LookupGitHubTokenForRepo("SpecialOrg/special-project")
	if err != nil || tokSpecial != "special-token" {
		t.Fatalf("期望 special-token, 得到 %q (err: %v)", tokSpecial, err)
	}

	tokDefault, err := LookupGitHubTokenForRepo("Random/project")
	if err != nil || tokDefault != "my-global-token" {
		t.Fatalf("期望全局 token 仍存在, 得到 %q (err: %v)", tokDefault, err)
	}
}
