package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLILanguageAndInstaller(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("installer requires bash")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash unavailable")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "upy")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	home := filepath.Join(dir, "home")
	os.MkdirAll(home, 0700)
	configPath := filepath.Join(home, "config.yaml")
	env := append(os.Environ(), "HOME="+home, "UPY_CONFIG="+configPath, "UPLY_CONFIG=", "UPY_LANG=", "SUDO_USER=", "SHELL=/bin/bash")
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		return string(out)
	}
	if got := run("--help"); !strings.Contains(got, "Commands:") || strings.Contains(got, "命令:") {
		t.Fatal("CLI does not default to English")
	}
	run("__set-language", "zh-CN")
	if got := run("--help"); !strings.Contains(got, "命令:") {
		t.Fatal("configured Chinese not applied")
	}
	cmd := exec.Command(binary, "--help")
	cmd.Env = append(env, "UPY_LANG=en")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Commands:") {
		t.Fatal("temporary language override failed")
	}
	// 使用真实编译二进制模拟安装资产，curl 仅从本地 fixture 取内容，确保不访问网络。
	mockBin := filepath.Join(dir, "mocks")
	os.MkdirAll(mockBin, 0700)
	fakeCurl := `#!/usr/bin/env bash
set -eu
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in -o) out="$2"; shift 2;; *) url="$1"; shift;; esac
done
case "$url" in
 *api.github.com*) printf '{}';;
 */SHA256SUMS) cp "$FIXTURE_SUMS" "$out";;
 */upy-*) cp "$FIXTURE_BINARY" "$out";;
 *) printf 'unexpected network request: %s\n' "$url" >&2; exit 1;;
esac
`
	os.WriteFile(filepath.Join(mockBin, "curl"), []byte(fakeCurl), 0700)
	content, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	asset := "upy-" + runtime.GOOS + "-" + arch
	sumPath := filepath.Join(dir, "SHA256SUMS")
	os.WriteFile(sumPath, []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(content), asset)), 0600)
	installDir := filepath.Join(dir, "bin")
	os.MkdirAll(installDir, 0700)
	for _, tc := range []struct {
		args       []string
		lang, want string
	}{{[]string{"--lang", "zh-CN"}, "zh-CN", "运行环境:"}, {[]string{"--lang=en"}, "en", "Platform:"}} {
		installer := exec.Command("bash", append([]string{"install.sh"}, tc.args...)...)
		installer.Env = append(env, "PATH="+mockBin+":"+os.Getenv("PATH"), "INSTALL_DIR="+installDir, "VERSION=v9.9.9", "FORCE=1", "FIXTURE_BINARY="+binary, "FIXTURE_SUMS="+sumPath)
		output, err := installer.CombinedOutput()
		if err != nil {
			t.Fatalf("installer %s: %v\n%s", tc.lang, err, output)
		}
		if !strings.Contains(string(output), tc.want) {
			t.Fatalf("installer language not applied: %s", output)
		}
		saved, err := os.ReadFile(configPath)
		if err != nil || !strings.Contains(string(saved), "language: "+tc.lang) {
			t.Fatalf("installer did not persist language: %s %v", saved, err)
		}
	}
	// 同版本安装仍更新语言，不触发额外下载；非法语言必须在网络访问前退出。
	installer := exec.Command("bash", "install.sh", "--lang", "zh-CN")
	installer.Env = append(env, "PATH="+mockBin+":"+os.Getenv("PATH"), "INSTALL_DIR="+installDir, "VERSION=vdev", "FORCE=0")
	if output, err := installer.CombinedOutput(); err != nil {
		t.Fatalf("same-version install: %v %s", err, output)
	}
	installer = exec.Command("bash", "install.sh", "--lang", "invalid")
	installer.Env = env
	if output, err := installer.CombinedOutput(); err == nil || !strings.Contains(string(output), "Unsupported language") {
		t.Fatalf("invalid installer language accepted: %s", output)
	}
}
