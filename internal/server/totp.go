package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/skip2/go-qrcode"
)

var (
	ErrTOTPLimitExceeded = errors.New("TOTP 绑定查看次数已达上限")
)

type TOTPData struct {
	Secret    string    `json:"secret"`
	Views     int       `json:"views"`
	CreatedAt time.Time `json:"created_at"`
}

type TOTPManager struct {
	mu         sync.Mutex
	filePath   string
	data       TOTPData
	fixedSecret string
}

func NewTOTPManager(filePath string, fixedSecret string) (*TOTPManager, error) {
	if filePath == "" {
		filePath = filepath.Join(config.UserDataDir(), "totp.json")
	}

	tm := &TOTPManager{
		filePath:    filePath,
		fixedSecret: strings.TrimSpace(fixedSecret),
	}

	if err := tm.loadOrInit(); err != nil {
		return nil, err
	}
	return tm, nil
}

func (tm *TOTPManager) loadOrInit() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(tm.filePath), 0o755); err != nil {
		return err
	}

	content, err := os.ReadFile(tm.filePath)
	if err == nil {
		var d TOTPData
		if jerr := json.Unmarshal(content, &d); jerr == nil && d.Secret != "" {
			tm.data = d
			if tm.fixedSecret != "" {
				tm.data.Secret = tm.fixedSecret
			}
			return nil
		}
	}

	// 初始化新 secret
	secret := tm.fixedSecret
	if secret == "" {
		secret = generateBase32Secret(20)
	}

	tm.data = TOTPData{
		Secret:    secret,
		Views:     0,
		CreatedAt: time.Now(),
	}

	return tm.saveLocked()
}

func (tm *TOTPManager) saveLocked() error {
	data, err := json.MarshalIndent(tm.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tm.filePath, data, 0o600)
}

// CanView 检查是否仍有查看 /auth 的配额
func (tm *TOTPManager) CanView(maxViews int) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.data.Views < maxViews
}

// RecordView 记录一次 /auth 页面访问，若超出配额则返回 false
func (tm *TOTPManager) RecordView(maxViews int) (currentViews int, allowed bool, err error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.data.Views >= maxViews {
		return tm.data.Views, false, nil
	}

	tm.data.Views++
	if err := tm.saveLocked(); err != nil {
		return tm.data.Views, false, err
	}
	return tm.data.Views, true, nil
}

// Secret 获取当前的 TOTP Secret
func (tm *TOTPManager) Secret() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.data.Secret
}

// ProvisioningURI 生成可供 Authenticator 扫码的 URI
func (tm *TOTPManager) ProvisioningURI(account, issuer string) string {
	secret := tm.Secret()
	if issuer == "" {
		issuer = "upy"
	}
	if account == "" {
		account = "admin"
	}
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(issuer), url.PathEscape(account), secret, url.QueryEscape(issuer))
}

// GenerateQRCodePNG 生成二维码 PNG 图片字节
func (tm *TOTPManager) GenerateQRCodePNG(uri string) ([]byte, error) {
	return qrcode.Encode(uri, qrcode.Medium, 256)
}

// Validate 校验 6 位动态验证码
func (tm *TOTPManager) Validate(code string) bool {
	secret := tm.Secret()
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	now := time.Now()
	// 校验当前窗口及前后各 30 秒窗口（容忍一定时钟漂移）
	times := []time.Time{
		now.Add(-30 * time.Second),
		now,
		now.Add(30 * time.Second),
	}
	for _, t := range times {
		expected, err := generateTOTP(secret, t)
		if err == nil && subtle.ConstantTimeCompare([]byte(code), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}

// generateBase32Secret 生成随机 Base32 密钥
func generateBase32Secret(byteLen int) string {
	b := make([]byte, byteLen)
	_, _ = rand.Read(b)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}

// generateTOTP 根据 RFC 6238 标准生成 6 位数字验证码
func generateTOTP(secret string, t time.Time) (string, error) {
	cleanSecret := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanSecret)
	if err != nil {
		// 尝试带填充补全后解析
		cleanSecret = padBase32(cleanSecret)
		key, err = base32.StdEncoding.DecodeString(cleanSecret)
		if err != nil {
			return "", fmt.Errorf("无效的 Base32 密钥: %w", err)
		}
	}

	counter := uint64(t.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	hash := mac.Sum(nil)

	offset := hash[len(hash)-1] & 0x0f
	codeInt := (binary.BigEndian.Uint32(hash[offset:]) & 0x7fffffff) % 1000000
	return fmt.Sprintf("%06d", codeInt), nil
}

func padBase32(s string) string {
	rem := len(s) % 8
	if rem == 0 {
		return s
	}
	return s + strings.Repeat("=", 8-rem)
}
