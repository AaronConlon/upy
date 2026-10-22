// Package crypto 提供纯 Go 实现的 bundle 强加密与解密支持 (AES-256-GCM + PBKDF2-HMAC-SHA256)。
// 封包格式:
// [ 4B Magic ("UPYC") ][ 1B Version (0x01) ][ 4B Iterations (uint32 BE) ][ 16B Salt ][ 12B Nonce ][ Ciphertext + 16B GCM Tag ]
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/AaronConlon/upy/internal/i18n"
)

const (
	// Magic 标识加密 bundle 的头部魔数
	Magic = "UPYC"
	// CurrentVersion 当前封包版本
	CurrentVersion byte = 0x01
	// DefaultIterations PBKDF2 默认迭代次数
	DefaultIterations = 100_000
	// SaltSize 盐长度 (字节)
	SaltSize = 16
	// NonceSize GCM 推荐标准 Nonce 长度 (字节)
	NonceSize = 12
	// KeySize AES-256 密钥长度 (字节)
	KeySize = 32
	// TagSize GCM 认证标签长度 (字节)
	TagSize = 16
	// HeaderSize 封包头部固定长度: 4(Magic) + 1(Ver) + 4(Iter) + 16(Salt) + 12(Nonce) = 37
	HeaderSize = 4 + 1 + 4 + SaltSize + NonceSize
	// MinEncryptedSize 最小有效密文大小 (包含头部和 Tag)
	MinEncryptedSize = HeaderSize + TagSize
)

// DeriveKey 使用 RFC 8018 PBKDF2-HMAC-SHA256 纯 Go 标准库派生密钥
func DeriveKey(password, salt []byte, iterations, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var dk []byte
	var u []byte
	var block []byte
	for i := 1; i <= numBlocks; i++ {
		prf.Reset()
		prf.Write(salt)
		var buf [4]byte
		binary.BigEndian.PutUint32(buf[:], uint32(i))
		prf.Write(buf[:])
		u = prf.Sum(u[:0])
		if block == nil {
			block = make([]byte, hashLen)
		}
		copy(block, u)
		for j := 2; j <= iterations; j++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for k := 0; k < hashLen; k++ {
				block[k] ^= u[k]
			}
		}
		dk = append(dk, block...)
	}
	return dk[:keyLen]
}

// Encrypt 使用 AES-256-GCM 加密明文字节切片
func Encrypt(plainData []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, i18n.Error("加密密码不能为空")
	}

	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf(i18n.T("生成随机盐失败: %w"), err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf(i18n.T("生成随机 Nonce 失败: %w"), err)
	}

	key := DeriveKey([]byte(password), salt, DefaultIterations, KeySize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 组装头部
	var buf bytes.Buffer
	buf.WriteString(Magic)
	buf.WriteByte(CurrentVersion)
	var iterBuf [4]byte
	binary.BigEndian.PutUint32(iterBuf[:], uint32(DefaultIterations))
	buf.Write(iterBuf[:])
	buf.Write(salt)
	buf.Write(nonce)

	// 密文 + Tag 紧随头部
	ciphertext := aesGCM.Seal(nil, nonce, plainData, nil)
	buf.Write(ciphertext)

	return buf.Bytes(), nil
}

// EncryptFile 加密文件并输出到目标路径
func EncryptFile(srcPath, dstPath, password string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf(i18n.T("读取源文件失败: %w"), err)
	}
	encrypted, err := Encrypt(data, password)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, encrypted, 0o644)
}

// Decrypt 使用 AES-256-GCM 解密密文字节切片
func Decrypt(cipherData []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, i18n.Error("解密密码不能为空")
	}

	if len(cipherData) < MinEncryptedSize {
		return nil, i18n.Error("数据过短，不是有效的加密 bundle")
	}

	if string(cipherData[:4]) != Magic {
		return nil, i18n.Error("无效的加密 bundle: 缺少 UPYC 标识魔数")
	}

	version := cipherData[4]
	if version != CurrentVersion {
		return nil, fmt.Errorf(i18n.T("不支持的加密封包版本: %d"), version)
	}

	iterations := int(binary.BigEndian.Uint32(cipherData[5:9]))
	if iterations <= 0 || iterations > 10_000_000 {
		return nil, i18n.Error("加密封包中的迭代次数异常")
	}

	offset := 9
	salt := cipherData[offset : offset+SaltSize]
	offset += SaltSize

	nonce := cipherData[offset : offset+NonceSize]
	offset += NonceSize

	ciphertext := cipherData[offset:]

	key := DeriveKey([]byte(password), salt, iterations, KeySize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, i18n.Error("bundle 解密失败：密码错误或数据已损坏/被篡改")
	}

	return plaintext, nil
}

// DecryptFile 解密文件并输出到目标路径
func DecryptFile(srcPath, dstPath, password string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf(i18n.T("读取待解密文件失败: %w"), err)
	}
	decrypted, err := Decrypt(data, password)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, decrypted, 0o644)
}

// IsEncrypted 检查数据头部是否包含 UPYC 加密魔数
func IsEncrypted(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == Magic
}

// IsEncryptedFile 检查文件是否为 UPYC 加密文件
func IsEncryptedFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	header := make([]byte, 4)
	n, err := io.ReadFull(f, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	return n >= 4 && string(header[:4]) == Magic, nil
}
