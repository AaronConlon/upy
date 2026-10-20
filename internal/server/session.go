package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const (
	SessionCookieName = "upy_session"
	SessionDuration   = 30 * time.Minute
)

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token -> expireAt
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]time.Time),
	}
}

// CreateSession 创建一个有效期为 30 分钟的 session token
func (s *SessionStore) CreateSession() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = time.Now().Add(SessionDuration)
	return token
}

// Validate 验证 token 是否存在且未过期
func (s *SessionStore) Validate(token string) bool {
	if token == "" {
		return false
	}
	s.mu.RLock()
	expireAt, ok := s.sessions[token]
	s.mu.RUnlock()

	if !ok {
		return false
	}
	if time.Now().After(expireAt) {
		s.Revoke(token)
		return false
	}
	return true
}

// Revoke 撤销指定的 session
func (s *SessionStore) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// SetSessionCookie 设置 30 分钟有效的 Session Cookie
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionDuration.Seconds()), // 1800 秒 = 30 分钟
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie 清理客户端 Session Cookie
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionToken 从 HTTP 请求 Cookie 中提取 session token
func GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie == nil {
		return ""
	}
	return cookie.Value
}
