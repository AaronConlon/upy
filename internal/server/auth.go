package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
)

var (
	ErrUnauthorized = errors.New("鉴权失败: 无效的 Token 或签名")
	ErrNoSecret     = errors.New("服务或项目未配置任何有效鉴权密钥")
)

// Authenticate 针对指定项目验证 HTTP 请求合法性。
// 支持:
// 1. GitHub Webhook HMAC-SHA256 (X-Hub-Signature-256)
// 2. HTTP Bearer Token (Authorization: Bearer <token> 或 X-Upy-Token)
func Authenticate(r *http.Request, globalToken, projectSecret string) error {
	globalToken = strings.TrimSpace(globalToken)
	projectSecret = strings.TrimSpace(projectSecret)

	// 若未配置任何密钥，拒绝所有请求
	if globalToken == "" && projectSecret == "" {
		return ErrNoSecret
	}

	// 1. 检查 GitHub HMAC-SHA256
	if sig := strings.TrimSpace(r.Header.Get("X-Hub-Signature-256")); sig != "" {
		bodyBytes, err := readAndRestoreBody(r)
		if err != nil {
			return err
		}
		secretToUse := projectSecret
		if secretToUse == "" {
			secretToUse = globalToken
		}
		if verifyGitHubHMAC(bodyBytes, secretToUse, sig) {
			return nil
		}
		return ErrUnauthorized
	}

	// 2. 检查 Bearer Token / X-Upy-Token
	token := extractBearerOrHeaderToken(r)
	if token == "" {
		return ErrUnauthorized
	}

	// 优先校验项目独立 secret
	if projectSecret != "" && subtle.ConstantTimeCompare([]byte(token), []byte(projectSecret)) == 1 {
		return nil
	}

	// 兜底校验全局 token
	if globalToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(globalToken)) == 1 {
		return nil
	}

	return ErrUnauthorized
}

func readAndRestoreBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes, nil
}

func verifyGitHubHMAC(payload []byte, secret, signatureHeader string) bool {
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		return false
	}
	sigHex := strings.TrimPrefix(signatureHeader, "sha256=")
	expectedMAC, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actualMAC := mac.Sum(nil)
	return hmac.Equal(actualMAC, expectedMAC)
}

func extractBearerOrHeaderToken(r *http.Request) string {
	if h := strings.TrimSpace(r.Header.Get("X-Upy-Token")); h != "" {
		return h
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
