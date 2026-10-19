package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadProjectConfigBundlesKeep(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantKeep    int
		wantErrText string
	}{
		{
			name: "默认缺省为 3",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
`,
			wantKeep: 3,
		},
		{
			name: "显式配置 bundles.keep: 5",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
bundles:
  keep: 5
`,
			wantKeep: 5,
		},
		{
			name: "显式配置 bundles: 4 (直接写整数)",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
bundles: 4
`,
			wantKeep: 4,
		},
		{
			name: "单数形式 bundle.keep: 2",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
bundle:
  keep: 2
`,
			wantKeep: 2,
		},
		{
			name: "边界值 keep: 0",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
bundles:
  keep: 0
`,
			wantKeep: 0,
		},
		{
			name: "边界值 keep: 100",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
bundles:
  keep: 100
`,
			wantKeep: 100,
		},
		{
			name: "非法负数 keep: -1 报错",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
bundles:
  keep: -1
`,
			wantErrText: "bundles.keep 必须在 0 到 100 之间 (当前为 -1)",
		},
		{
			name: "非法超限 keep: 101 报错",
			yaml: `name: my-app
release:
  provider: github
  repository: acme/my-app
bundles:
  keep: 101
`,
			wantErrText: "bundles.keep 必须在 0 到 100 之间 (当前为 101)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(tt.yaml), 0o644)
			if err != nil {
				t.Fatalf("写入测试配置失败: %v", err)
			}

			cfg, err := ReadProjectConfig(dir)
			if tt.wantErrText != "" {
				if err == nil {
					t.Fatalf("期望报错包含 %q，但实际未报错", tt.wantErrText)
				}
				if !filepathContains(err.Error(), tt.wantErrText) {
					t.Fatalf("错误信息不匹配: 期望包含 %q, 得到 %q", tt.wantErrText, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("读取配置失败: %v", err)
			}
			if got := cfg.BundlesKeep(); got != tt.wantKeep {
				t.Fatalf("BundlesKeep() 结果不匹配: 期望 %d, 得到 %d", tt.wantKeep, got)
			}
		})
	}
}

func filepathContains(s, sub string) bool {
	return filepath.Clean(s) != "" && (len(sub) == 0 || (len(s) >= len(sub) && (s == sub || searchSubstr(s, sub))))
}

func searchSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
