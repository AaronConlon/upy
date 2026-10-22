package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AaronConlon/upy/internal/crypto"
)

func TestEncryptCommand(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.zip")
	dst := filepath.Join(tmpDir, "test.zip.enc")

	if err := os.WriteFile(src, []byte("dummy zip content for bundle"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 缺少输入文件参数报错
	if err := Encrypt(EncryptArgs{}); err == nil {
		t.Fatal("未提供输入文件应该报错")
	}

	// 正常加密
	err := Encrypt(EncryptArgs{
		Input:    src,
		Output:   dst,
		Password: "pass-123456-secret",
	})
	if err != nil {
		t.Fatalf("Encrypt 失败: %v", err)
	}

	isEnc, err := crypto.IsEncryptedFile(dst)
	if err != nil || !isEnc {
		t.Fatalf("生成的文件应为加密格式: isEnc=%v, err=%v", isEnc, err)
	}

	// 解密验证
	restored := filepath.Join(tmpDir, "restored.zip")
	if err := crypto.DecryptFile(dst, restored, "pass-123456-secret"); err != nil {
		t.Fatalf("解密还原失败: %v", err)
	}

	data, err := os.ReadFile(restored)
	if err != nil || string(data) != "dummy zip content for bundle" {
		t.Fatalf("内容不一致: %v, %s", err, string(data))
	}
}
