package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
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

type TOTPData struct {
	Secret    string    `json:"secret"`
	Views     int       `json:"views"`
	CreatedAt time.Time `json:"created_at"`
}

type TOTPManager struct {
	mu          sync.Mutex
	filePath    string
	data        TOTPData
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

// 已有密钥只读复用；损坏文件报错，不能静默生成新密钥使已有绑定失效。
func (tm *TOTPManager) loadOrInit() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(tm.filePath), 0700); err != nil {
		return err
	}
	load := func() error {
		content, err := os.ReadFile(tm.filePath)
		if err != nil {
			return err
		}
		var d TOTPData
		if err := json.Unmarshal(content, &d); err != nil || strings.TrimSpace(d.Secret) == "" {
			return fmt.Errorf("TOTP 密钥文件无效，请检查 %s", tm.filePath)
		}
		if tm.fixedSecret != "" {
			d.Secret = tm.fixedSecret
		}
		if _, err := generateTOTP(d.Secret, time.Now()); err != nil {
			return fmt.Errorf("TOTP 密钥无效，请检查服务器配置和 %s", tm.filePath)
		}
		tm.data = d
		return nil
	}
	if err := load(); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	secret := tm.fixedSecret
	if secret == "" {
		var err error
		secret, err = generateBase32Secret(20)
		if err != nil {
			return err
		}
	}
	if _, err := generateTOTP(secret, time.Now()); err != nil {
		return fmt.Errorf("server.totp_secret 不是有效的 Base32 密钥")
	}
	d := TOTPData{Secret: secret, Views: 0, CreatedAt: time.Now()}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	// 完整写入后排他发布，避免 CLI 与 server 并发初始化覆盖彼此的密钥。
	f, err := os.CreateTemp(filepath.Dir(tm.filePath), ".totp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Link(tmp, tm.filePath); err != nil {
		if os.IsExist(err) {
			return load()
		}
		return err
	}
	tm.data = d
	return nil
}

func (tm *TOTPManager) saveLocked() error {
	data, err := json.MarshalIndent(tm.data, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(tm.filePath), ".totp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, tm.filePath)
}

// Views 获取当前的已查看次数
func (tm *TOTPManager) Views() int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.data.Views
}

// CanView 检查是否仍有展示配额
func (tm *TOTPManager) CanView(maxViews int) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.data.Views < maxViews
}

// RemainingViews 返回剩余查看次数
func (tm *TOTPManager) RemainingViews(maxViews int) int {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	r := maxViews - tm.data.Views
	if r < 0 {
		return 0
	}
	return r
}

// RecordView 记录一次绑定二维码查看，若超出配额则返回 allowed=false
func (tm *TOTPManager) RecordView(maxViews int) (remaining int, allowed bool, err error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.data.Views >= maxViews {
		return 0, false, nil
	}

	tm.data.Views++
	if err := tm.saveLocked(); err != nil {
		tm.data.Views--
		return maxViews - tm.data.Views, false, err
	}
	remaining = maxViews - tm.data.Views
	if remaining < 0 {
		remaining = 0
	}
	return remaining, true, nil
}

// Secret 获取当前的 TOTP Secret
func (tm *TOTPManager) Secret() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.data.Secret
}

// ProvisioningURI 生成可供 Authenticator 扫码的 URI
func (tm *TOTPManager) ProvisioningURI(account, issuer string) string {
	secret := strings.ToUpper(strings.ReplaceAll(tm.Secret(), " ", ""))
	if issuer == "" {
		issuer = "upy"
	}
	if account == "" {
		account = "admin"
	}
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		url.PathEscape(issuer), url.PathEscape(account), url.QueryEscape(secret), url.QueryEscape(issuer))
}

// TerminalQRCode 生成终端黑白二维码，不经过 CLI 日志记录器。
func (tm *TOTPManager) TerminalQRCode(account string) (string, error) {
	qr, err := qrcode.New(tm.ProvisioningURI(account, "upy"), qrcode.Medium)
	if err != nil {
		return "", err
	}
	return "\x1b[30;47m" + qr.ToSmallString(true) + "\x1b[0m\n", nil
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
func generateBase32Secret(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
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
