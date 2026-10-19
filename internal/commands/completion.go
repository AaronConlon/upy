// Shell 命令补全安装。仅提供 upy 的一级命令，不读取项目配置或访问网络。
package commands

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/AaronConlon/upy/internal/platform"
)

const (
	completionBlockStart = "# >>> upy shell completion >>>"
	completionBlockEnd   = "# <<< upy shell completion <<<"
)

var completionCommands = []completionCommand{
	{Name: "release", Description: "从 GitHub Release 部署 bundle"},
	{Name: "deploy", Description: "部署已登记项目的最新版本"},
	{Name: "clean", Description: "清理项目数据 (缓存/旧版本/临时文件)"},
	{Name: "backup", Description: "执行项目约定备份 (运行 backup.sh)"},
	{Name: "analysis", Description: "分析已部署项目与磁盘空间开销"},
	{Name: "bundle", Description: "部署本地 bundle"},
	{Name: "version", Description: "查看当前版本"},
	{Name: "update", Description: "更新 upy"},
	{Name: "init", Description: "初始化 GitHub token"},
}

type completionCommand struct {
	Name        string
	Description string
}

// CompletionInstallResult 描述自动补全配置的安装结果。
type CompletionInstallResult struct {
	Shell       string
	Supported   bool
	ScriptPath  string
	ScriptPaths []string
}

type completionTarget struct {
	home        string
	uid         int
	gid         int
	shouldChown bool
}

// InstallShellCompletion 自动识别当前 shell 及系统中已配置的 shell 并同步安装/更新一级命令补全。
func InstallShellCompletion() (CompletionInstallResult, error) {
	curShell := detectShell()
	result := CompletionInstallResult{Shell: curShell}

	target, err := completionTargetUser()
	if err != nil {
		return result, err
	}

	// 收集所有需要安装/更新补全的 shell (支持 zsh, bash, fish)
	shellsToInstall := make(map[string]bool)
	if curShell == "zsh" || curShell == "bash" || curShell == "fish" {
		shellsToInstall[curShell] = true
	}

	// 探测用户主目录下是否已配置相应 shell，若有则一并保持同步更新 (防止多 shell 或非默认登录 shell 补全失效)
	if fileOrDirExists(target.home, ".zshrc") || fileOrDirExists(target.home, ".upy", "completions", "upy.zsh") {
		shellsToInstall["zsh"] = true
	}
	if fileOrDirExists(target.home, ".bashrc") || fileOrDirExists(target.home, ".bash_profile") || fileOrDirExists(target.home, ".upy", "completions", "upy.bash") {
		shellsToInstall["bash"] = true
	}
	if fileOrDirExists(target.home, ".config", "fish") || fileOrDirExists(target.home, ".config", "fish", "completions", "upy.fish") {
		shellsToInstall["fish"] = true
	}

	// 若未检测到任何已有配置且当前 shell 不在支持列表中，根据操作系统选用默认交互 shell
	if len(shellsToInstall) == 0 {
		if platform.CurrentOS() == "darwin" {
			shellsToInstall["zsh"] = true
		} else {
			shellsToInstall["bash"] = true
		}
	}

	orderedShells := []string{"zsh", "bash", "fish"}
	var scriptPaths []string
	var errs []error

	for _, sh := range orderedShells {
		if !shellsToInstall[sh] {
			continue
		}
		path, err := installCompletionForShell(target, sh)
		if err != nil {
			errs = append(errs, fmt.Errorf("安装 %s 补全失败: %w", sh, err))
		} else if path != "" {
			scriptPaths = append(scriptPaths, path)
		}
	}

	if len(scriptPaths) > 0 {
		result.Supported = true
		result.ScriptPaths = scriptPaths
		result.ScriptPath = strings.Join(scriptPaths, ", ")
	}

	return result, errors.Join(errs...)
}

func fileOrDirExists(parts ...string) bool {
	p := filepath.Join(parts...)
	_, err := os.Stat(p)
	return err == nil
}

func installCompletionForShell(target completionTarget, shell string) (string, error) {
	if shell == "fish" {
		configDir := filepath.Join(target.home, ".config")
		dir := filepath.Join(configDir, "fish", "completions")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		_ = target.chown(configDir, filepath.Join(configDir, "fish"), dir)
		scriptPath := filepath.Join(dir, "upy.fish")
		if err := os.WriteFile(scriptPath, []byte(completionScript(shell)), 0o644); err != nil {
			return "", err
		}
		_ = target.chown(scriptPath)
		return scriptPath, nil
	}

	configDir := filepath.Join(target.home, ".upy")
	dir := filepath.Join(configDir, "completions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	_ = target.chown(configDir, dir)
	scriptPath := filepath.Join(dir, "upy."+shell)
	if err := os.WriteFile(scriptPath, []byte(completionScript(shell)), 0o644); err != nil {
		return "", err
	}
	_ = target.chown(scriptPath)
	rcPath := filepath.Join(target.home, rcFile(shell))
	if err := installCompletionSource(rcPath, shell); err != nil {
		return scriptPath, err
	}
	_ = target.chown(rcPath)
	return scriptPath, nil
}

func detectShell() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return "unknown"
	}
	name := strings.ToLower(filepath.Base(shell))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "unknown"
	}
	return name
}

// completionTargetUser 在 sudo update 时优先写回原调用用户，而不是 root。
func completionTargetUser() (completionTarget, error) {
	if sudoUser := strings.TrimSpace(os.Getenv("SUDO_USER")); sudoUser != "" && sudoUser != "root" && os.Geteuid() == 0 {
		account, err := user.Lookup(sudoUser)
		if err == nil && strings.TrimSpace(account.HomeDir) != "" {
			uid, uidErr := strconv.Atoi(account.Uid)
			gid, gidErr := strconv.Atoi(account.Gid)
			if uidErr == nil && gidErr == nil {
				return completionTarget{home: account.HomeDir, uid: uid, gid: gid, shouldChown: true}, nil
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return completionTarget{}, fmt.Errorf("无法确定当前用户的主目录")
	}
	return completionTarget{home: home}, nil
}

func (target completionTarget) chown(paths ...string) error {
	if !target.shouldChown {
		return nil
	}
	for _, path := range paths {
		if err := os.Chown(path, target.uid, target.gid); err != nil {
			return err
		}
	}
	return nil
}

func rcFile(shell string) string {
	if shell == "zsh" {
		return ".zshrc"
	}
	return ".bashrc"
}

func installCompletionSource(path, shell string) error {
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	sourcePath := "$HOME/.upy/completions/upy." + shell
	block := strings.Join([]string{
		completionBlockStart,
		"if [ -r \"" + sourcePath + "\" ]; then",
		"  source \"" + sourcePath + "\"",
		"fi",
		completionBlockEnd,
	}, "\n")

	next := string(current)
	if start := strings.Index(next, completionBlockStart); start >= 0 {
		end := strings.Index(next[start:], completionBlockEnd)
		if end >= 0 {
			end += start + len(completionBlockEnd)
			next = next[:start] + block + next[end:]
		} else {
			next = strings.TrimRight(next, "\n") + "\n\n" + block + "\n"
		}
	} else {
		next = strings.TrimRight(next, "\n") + "\n\n" + block + "\n"
	}
	return os.WriteFile(path, []byte(next), 0o644)
}

func completionScript(shell string) string {
	switch shell {
	case "zsh":
		lines := []string{
			"#compdef upy",
			"# Generated by upy. Reinstalled automatically during install and update.",
			"if ! (( $+functions[compdef] )); then",
			"  autoload -Uz compinit",
			"  compinit",
			"fi",
			"_upy() {",
			"  local -a commands",
			"  commands=(",
		}
		for _, command := range completionCommands {
			lines = append(lines, "    '"+command.Name+":"+command.Description+"'")
		}
		lines = append(lines,
			"  )",
			"  if (( CURRENT == 2 )); then",
			"    _describe -t commands 'upy command' commands",
			"  fi",
			"}",
			"compdef _upy upy",
			"",
		)
		return strings.Join(lines, "\n")
	case "bash":
		names := make([]string, 0, len(completionCommands))
		for _, command := range completionCommands {
			names = append(names, command.Name)
		}
		return strings.Join([]string{
			"# Generated by upy. Reinstalled automatically during install and update.",
			"_upy_completion() {",
			"  local current=\"${COMP_WORDS[COMP_CWORD]}\"",
			"  COMPREPLY=()",
			"  if (( COMP_CWORD == 1 )); then",
			"    COMPREPLY=( $(compgen -W '" + strings.Join(names, " ") + "' -- \"$current\") )",
			"  fi",
			"}",
			"complete -F _upy_completion upy",
			"",
		}, "\n")
	default: // fish
		lines := []string{"# Generated by upy. Reinstalled automatically during install and update."}
		for _, command := range completionCommands {
			lines = append(lines, "complete -c upy -n '__fish_use_subcommand' -a "+command.Name+" -d '"+command.Description+"'")
		}
		lines = append(lines, "")
		return strings.Join(lines, "\n")
	}
}
