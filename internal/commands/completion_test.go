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
