package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadUserConfigMissing(t *testing.T) {
	t.Setenv("UPY_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("UPLY_CONFIG", "")
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
	t.Setenv("UPY_CONFIG", path)
	t.Setenv("UPLY_CONFIG", "")
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
	t.Setenv("UPY_CONFIG", path)
	t.Setenv("UPLY_CONFIG", "")
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
	t.Setenv("UPY_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("UPLY_CONFIG", "")
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
	t.Setenv("UPY_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("UPLY_CONFIG", "")
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

func TestLocalProjectsAreSavedAndStaleRootsAreCleaned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("UPY_CONFIG", path)

	root := filepath.Join(dir, "codia")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deploy.yaml"), []byte("name: codia\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SaveGitHubToken("", "kept-token"); err != nil {
		t.Fatal(err)
	}
	if err := SaveLocalProject("codia", root); err != nil {
		t.Fatal(err)
	}

	projects, err := ListLocalProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Name != "codia" || projects[0].Root != root {
		t.Fatalf("项目记录不正确: %+v", projects)
	}
	if token, err := LookupGitHubToken(); err != nil || token != "kept-token" {
		t.Fatalf("保存项目不应覆盖 token: token=%q err=%v", token, err)
	}

	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	projects, err = ListLocalProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("失效根目录应被清理，得到: %+v", projects)
	}
	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("清理结果未写回配置: %+v", cfg.Projects)
	}
}

func TestUnregisterLocalProject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("UPY_CONFIG", path)

	rootA := filepath.Join(dir, "projectA")
	_ = os.MkdirAll(rootA, 0o755)
	_ = os.WriteFile(filepath.Join(rootA, "deploy.yaml"), []byte("name: projectA\n"), 0o644)

	rootB := filepath.Join(dir, "projectB")
	_ = os.MkdirAll(rootB, 0o755)
	_ = os.WriteFile(filepath.Join(rootB, "deploy.yaml"), []byte("name: projectB\n"), 0o644)

	if err := SaveLocalProject("projectA", rootA); err != nil {
		t.Fatal(err)
	}
	if err := SaveLocalProject("projectB", rootB); err != nil {
		t.Fatal(err)
	}

	projects, _ := ListLocalProjects()
	if len(projects) != 2 {
		t.Fatalf("期望 2 个项目, 得到: %d", len(projects))
	}

	// 注销 projectA
	if err := UnregisterLocalProject("projectA", ""); err != nil {
		t.Fatalf("UnregisterLocalProject 失败: %v", err)
	}

	projects, _ = ListLocalProjects()
	if len(projects) != 1 || projects[0].Name != "projectB" {
		t.Fatalf("期望仅剩 projectB, 得到: %+v", projects)
	}
}

func TestBarkChannelsFormats(t *testing.T) {
	// 1. 单 token 标量
	t.Run("ScalarToken", func(t *testing.T) {
		var cfg UserConfig
		raw := []byte(`notify:
  bark: my-single-token
`)
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		barks := cfg.ActiveBarks()
		if len(barks) != 1 || barks[0].Key() != "my-single-token" {
			t.Fatalf("期望 1 个 token, 得到: %+v", barks)
		}
	})

	// 2. 字符串数组 (多用户 token)
	t.Run("StringList", func(t *testing.T) {
		var cfg UserConfig
		raw := []byte(`notify:
  bark:
    - token_alice
    - token_bob
    - https://api.day.app/token_carol
`)
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		barks := cfg.ActiveBarks()
		if len(barks) != 3 {
			t.Fatalf("期望 3 个 token, 得到: %d", len(barks))
		}
		if barks[0].Key() != "token_alice" || barks[1].Key() != "token_bob" || barks[2].Key() != "token_carol" {
			t.Fatalf("tokens 不对: %+v", barks)
		}
		if barks[2].Endpoint() != "https://api.day.app" {
			t.Fatalf("url endpoint 不对: %s", barks[2].Endpoint())
		}
	})

	// 3. 带 tokens 数组的对象
	t.Run("ObjectWithTokensList", func(t *testing.T) {
		var cfg UserConfig
		raw := []byte(`notify:
  bark:
    serverUrl: https://bark.example.com
    group: my-cluster
    sound: chime
    tokens:
      - dev1_token
      - dev2_token
`)
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		barks := cfg.ActiveBarks()
		if len(barks) != 2 {
			t.Fatalf("期望 2 个 token, 得到: %d", len(barks))
		}
		for _, b := range barks {
			if b.Endpoint() != "https://bark.example.com" {
				t.Fatalf("endpoint 不对: %s", b.Endpoint())
			}
			if b.Group != "my-cluster" || b.Sound != "chime" {
				t.Fatalf("属性未继承: %+v", b)
			}
		}
		if barks[0].Key() != "dev1_token" || barks[1].Key() != "dev2_token" {
			t.Fatalf("keys 不对: %+v", barks)
		}
	})

	// 4. 统一写法: 通用属性 + 按 - name 分多个用户，用户级字段优先级高于通用字段
	t.Run("UnifiedUsersConfig", func(t *testing.T) {
		var cfg UserConfig
		raw := []byte(`notify:
  bark:
    serverUrl: https://api.day.app
    group: unified-app
    sound: chime
    icon: https://example.com/icon.png
    level: active
    users:
      - name: Alice
        token: token_alice
        sound: glass # 覆盖通用 sound
      - name: Bob
        token: token_bob
        serverUrl: https://bark.private.internal # 覆盖通用 serverUrl
        level: timeSensitive
      - name: Carol
        token: token_carol # 完全继承通用属性
`)
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		barks := cfg.ActiveBarks()
		if len(barks) != 3 {
			t.Fatalf("期望 3 个用户, 得到: %d", len(barks))
		}

		// Alice: sound 为 glass，其他继承
		if barks[0].Name != "Alice" || barks[0].Key() != "token_alice" {
			t.Errorf("Alice 基本信息不对: %+v", barks[0])
		}
		if barks[0].Sound != "glass" {
			t.Errorf("Alice 期望 sound=glass (覆盖通用), 得到: %s", barks[0].Sound)
		}
		if barks[0].ServerURL != "https://api.day.app" || barks[0].Group != "unified-app" {
			t.Errorf("Alice 通用属性未正确继承: %+v", barks[0])
		}

		// Bob: serverUrl 和 level 覆盖，其他继承
		if barks[1].Name != "Bob" || barks[1].Key() != "token_bob" {
			t.Errorf("Bob 基本信息不对: %+v", barks[1])
		}
		if barks[1].ServerURL != "https://bark.private.internal" || barks[1].Level != "timeSensitive" {
			t.Errorf("Bob 覆盖字段不对: %+v", barks[1])
		}
		if barks[1].Sound != "chime" || barks[1].Group != "unified-app" {
			t.Errorf("Bob 通用属性未继承: %+v", barks[1])
		}

		// Carol: 完全继承
		if barks[2].Name != "Carol" || barks[2].Key() != "token_carol" {
			t.Errorf("Carol 基本信息不对: %+v", barks[2])
		}
		if barks[2].ServerURL != "https://api.day.app" || barks[2].Sound != "chime" || barks[2].Group != "unified-app" {
			t.Errorf("Carol 继承属性不对: %+v", barks[2])
		}
	})
}

func TestBarkUrlParsing(t *testing.T) {
	b1 := UserBark{Token: "https://api.day.app/abc123xyz"}
	if b1.Endpoint() != "https://api.day.app" {
		t.Fatalf("b1 endpoint=%s", b1.Endpoint())
	}
	if b1.Key() != "abc123xyz" {
		t.Fatalf("b1 key=%s", b1.Key())
	}

	b2 := UserBark{
		ServerURL: "https://custom.bark.net",
		Token:     "https://custom.bark.net/subpath/device_key_888",
	}
	if b2.Endpoint() != "https://custom.bark.net" {
		t.Fatalf("b2 endpoint=%s", b2.Endpoint())
	}
	if b2.Key() != "subpath/device_key_888" {
		t.Fatalf("b2 key=%s", b2.Key())
	}

	b3 := UserBark{
		ServerURL: "https://custom.bark.net/subpath",
		Token:     "https://custom.bark.net/subpath/device_key_999",
	}
	if b3.Endpoint() != "https://custom.bark.net/subpath" {
		t.Fatalf("b3 endpoint=%s", b3.Endpoint())
	}
	if b3.Key() != "device_key_999" {
		t.Fatalf("b3 key=%s", b3.Key())
	}
}

func TestLoadUserConfigServer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	raw := []byte(`server:
  listen: "0.0.0.0:9000"
  token: "sec_token_123"
  public_url: "https://ci.example.com"
  max_log_history: 50
`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("UPY_CONFIG", path)
	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.EffectiveListen() != "0.0.0.0:9000" {
		t.Fatalf("listen 不对: %s", cfg.Server.EffectiveListen())
	}
	if cfg.Server.Token != "sec_token_123" {
		t.Fatalf("token 不对: %s", cfg.Server.Token)
	}
	if cfg.Server.PublicURL != "https://ci.example.com" {
		t.Fatalf("public_url 不对: %s", cfg.Server.PublicURL)
	}
	if cfg.Server.EffectiveMaxLogHistory() != 50 {
		t.Fatalf("max_log_history 不对: %d", cfg.Server.EffectiveMaxLogHistory())
	}

	// 验证缺省值
	emptyCfg := ServerConfig{}
	if emptyCfg.EffectiveListen() != "127.0.0.1:8999" {
		t.Fatalf("默认 listen 不对: %s", emptyCfg.EffectiveListen())
	}
	if emptyCfg.EffectiveMaxLogHistory() != 30 {
		t.Fatalf("默认 max_log_history 不对: %d", emptyCfg.EffectiveMaxLogHistory())
	}

	if UserDataDir() != dir {
		t.Fatalf("UserDataDir 不对: %s, 期望 %s", UserDataDir(), dir)
	}
}

func TestUpdateProxyConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("UPY_CONFIG", path)
	for _, tc := range []struct {
		yaml    string
		enabled bool
	}{{"", false}, {"update: {}", false}, {"update:\n  proxy_enabled: false\n", false}, {"update:\n  proxy_enabled: true\n", true}} {
		if err := os.WriteFile(path, []byte(tc.yaml), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadUserConfig()
		if err != nil || cfg.Update.ProxyEnabled != tc.enabled {
			t.Fatalf("config %q: %+v %v", tc.yaml, cfg, err)
		}
		if _, err := saveUserConfig(cfg); err != nil {
			t.Fatal(err)
		}
		again, err := LoadUserConfig()
		if err != nil || again.Update.ProxyEnabled != tc.enabled {
			t.Fatal("update option lost after saving config")
		}
	}
}

func TestSaveLanguagePreservesConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("UPY_CONFIG", path)
	original := "github:\n  token: keep-token\nupdate:\n  proxy_enabled: true\nserver:\n  listen: 127.0.0.1:8999\nprojects:\n  - name: demo\n    root: /srv/demo\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveLanguage("zh"); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "zh-CN" || cfg.GitHub.Token != "keep-token" || !cfg.Update.ProxyEnabled || len(cfg.Projects) != 1 || cfg.Server.Listen != "127.0.0.1:8999" {
		t.Fatalf("configuration lost: %+v", cfg)
	}
	before, _ := os.ReadFile(path)
	if _, err := SaveLanguage("unsupported"); err == nil {
		t.Fatal("invalid language accepted")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("invalid language changed configuration")
	}
}
