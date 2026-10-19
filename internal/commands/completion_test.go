package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionScriptsContainEveryTopLevelCommand(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		script := completionScript(shell)
		for _, command := range completionCommands {
			if !strings.Contains(script, command.Name) {
				t.Fatalf("%s 补全脚本缺少命令 %q:\n%s", shell, command.Name, script)
			}
		}
	}
}

func TestInstallCompletionSourceIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zshrc")
	if err := os.WriteFile(path, []byte("export PATH=/usr/local/bin:$PATH\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installCompletionSource(path, "zsh"); err != nil {
		t.Fatal(err)
	}
	if err := installCompletionSource(path, "zsh"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if strings.Count(content, completionBlockStart) != 1 || strings.Count(content, completionBlockEnd) != 1 {
		t.Fatalf("补全配置块不应重复追加:\n%s", content)
	}
	if !strings.Contains(content, "$HOME/.upy/completions/upy.zsh") {
		t.Fatalf("补全配置块未引用 zsh 脚本:\n%s", content)
	}
}

func TestInstallShellCompletion(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("SUDO_USER", "")

	// 创建初始 .zshrc
	zshrc := filepath.Join(tempHome, ".zshrc")
	_ = os.WriteFile(zshrc, []byte("# existing config\n"), 0o644)

	res, err := InstallShellCompletion()
	if err != nil {
		t.Fatalf("InstallShellCompletion failed: %v", err)
	}
	if !res.Supported {
		t.Fatalf("期望 Supported=true, 实际 false")
	}

	zshScript := filepath.Join(tempHome, ".upy", "completions", "upy.zsh")
	data, err := os.ReadFile(zshScript)
	if err != nil {
		t.Fatalf("读取生成的 zsh 补全脚本失败: %v", err)
	}
	if !strings.Contains(string(data), "analysis") {
		t.Fatalf("生成的补全脚本缺少 analysis 命令:\n%s", string(data))
	}
}
