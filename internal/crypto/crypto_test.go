package crypto

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeriveKeyPBKDF2HMACSHA256(t *testing.T) {
	// PBKDF2 HMAC-SHA256 标准测试
	// Password = "password", Salt = "salt", c = 1, dkLen = 32
	password := []byte("password")
	salt := []byte("salt")
	key := DeriveKey(password, salt, 1, 32)
	expectedHex := "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"
	got := hex.EncodeToString(key)
	if got != expectedHex {
		t.Fatalf("PBKDF2 结果不符合标准: 期望 %s, 实际 %s", expectedHex, got)
	}

	// 2 次迭代测试: c = 2
	key2 := DeriveKey(password, salt, 2, 32)
	expectedHex2 := "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"
	if hex.EncodeToString(key2) != expectedHex2 {
		t.Fatalf("PBKDF2 c=2 结果不符合标准: 期望 %s, 实际 %s", expectedHex2, hex.EncodeToString(key2))
	}
}

func TestEncryptDecrypt(t *testing.T) {
	original := []byte("hello world! this is a secret bundle content with zip binary data.")
	pass := "super-secure-pass-123"

	enc, err := Encrypt(original, pass)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	if !IsEncrypted(enc) {
		t.Fatal("未识别出加密魔数")
	}

	dec, err := Decrypt(enc, pass)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if !bytes.Equal(original, dec) {
		t.Fatalf("解密内容与原文不一致: 得到 %q, 期望 %q", string(dec), string(original))
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	original := []byte("confidential deploy payload")
	enc, err := Encrypt(original, "correct-password")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	_, err = Decrypt(enc, "wrong-password")
	if err == nil {
		t.Fatal("使用错误密码解密应该报错，但成功了解密")
	}
	errStr := strings.ToLower(err.Error())
	if !strings.Contains(errStr, "password") && !strings.Contains(errStr, "decryption failed") &&
		!strings.Contains(err.Error(), "密码错误") && !strings.Contains(err.Error(), "解密失败") {
		t.Fatalf("错误提示应说明密码错误或解密失败: %v", err)
	}
}

func TestDecryptTamperedData(t *testing.T) {
	original := []byte("test tamper resistance")
	enc, err := Encrypt(original, "pwd")
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 篡改密文最后 1 个字节
	enc[len(enc)-1] ^= 0x01

	_, err = Decrypt(enc, "pwd")
	if err == nil {
		t.Fatal("篡改密文后解密应该失败，但成功了")
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "bundle.zip")
	encFile := filepath.Join(tmpDir, "bundle.zip.enc")
	decFile := filepath.Join(tmpDir, "bundle-restored.zip")

	content := []byte("PK\x03\x04 fake zip content for testing 1234567890")
	if err := os.WriteFile(srcFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	isEnc, err := IsEncryptedFile(srcFile)
	if err != nil || isEnc {
		t.Fatalf("未加密文件不应返回 true: isEnc=%v, err=%v", isEnc, err)
	}

	if err := EncryptFile(srcFile, encFile, "file-pwd"); err != nil {
		t.Fatalf("EncryptFile 失败: %v", err)
	}

	isEnc, err = IsEncryptedFile(encFile)
	if err != nil || !isEnc {
		t.Fatalf("加密文件应当返回 true: isEnc=%v, err=%v", isEnc, err)
	}

	if err := DecryptFile(encFile, decFile, "file-pwd"); err != nil {
		t.Fatalf("DecryptFile 失败: %v", err)
	}

	restored, err := os.ReadFile(decFile)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, restored) {
		t.Fatalf("文件还原不一致: %s != %s", string(content), string(restored))
	}
}

func TestInvalidPayloads(t *testing.T) {
	if _, err := Decrypt([]byte("too-short"), "pwd"); err == nil {
		t.Fatal("过短数据应该报错")
	}

	fakeMagic := []byte("ABCD\x01\x00\x00\x00\x01" + strings.Repeat("a", 60))
	if _, err := Decrypt(fakeMagic, "pwd"); err == nil {
		t.Fatal("魔数错误应该报错")
	}
}
