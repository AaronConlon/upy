package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpyCryptCLI(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "upy-crypt")

	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("编译 upy-crypt 失败: %v, %s", err, out)
	}

	testData := []byte("plain text content for bundle zip testing")
	srcPath := filepath.Join(tmpDir, "bundle.zip")
	encPath := filepath.Join(tmpDir, "bundle.zip.enc")
	decPath := filepath.Join(tmpDir, "bundle.zip.restored")

	if err := os.WriteFile(srcPath, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. 帮助信息
	helpCmd := exec.Command(binPath, "-h")
	helpOut, err := helpCmd.CombinedOutput()
	if err != nil || !strings.Contains(string(helpOut), "upy-crypt") {
		t.Fatalf("帮助信息不正确: %v, %s", err, helpOut)
	}

	// 2. 正常加密 (-p 传参)
	encCmd := exec.Command(binPath, "-p", "secret-key-123", srcPath, encPath)
	if out, err := encCmd.CombinedOutput(); err != nil {
		t.Fatalf("加密失败: %v, %s", err, out)
	}

	if _, err := os.Stat(encPath); err != nil {
		t.Fatalf("未生成加密文件: %v", err)
	}

	// 3. 正常解密 (-p 传参)
	decCmd := exec.Command(binPath, "-d", "-p", "secret-key-123", encPath, decPath)
	if out, err := decCmd.CombinedOutput(); err != nil {
		t.Fatalf("解密失败: %v, %s", err, out)
	}

	restored, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(testData, restored) {
		t.Fatalf("解密还原数据不匹配: %s != %s", string(testData), string(restored))
	}

	// 4. 环境变量传密码加密
	autoEncPath := filepath.Join(tmpDir, "bundle2.zip.enc")
	src2Path := filepath.Join(tmpDir, "bundle2.zip")
	_ = os.WriteFile(src2Path, testData, 0o644)

	envEncCmd := exec.Command(binPath, src2Path)
	envEncCmd.Env = append(os.Environ(), "UPY_CRYPT_PASSWORD=env-secret-888")
	if out, err := envEncCmd.CombinedOutput(); err != nil {
		t.Fatalf("环境变量加密失败: %v, %s", err, out)
	}

	if _, err := os.Stat(autoEncPath); err != nil {
		t.Fatalf("默认输出文件 %s 应该存在: %v", autoEncPath, err)
	}

	// 5. 错误密码解密应失败
	failDecCmd := exec.Command(binPath, "-d", "-p", "wrong-secret", autoEncPath)
	if err := failDecCmd.Run(); err == nil {
		t.Fatal("使用错误密码解密应当返回非零退出码，但返回了 0")
	}
}
