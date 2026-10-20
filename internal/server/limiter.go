package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LoginLimiter 首页动态验证码尝试频率限制器。
// 每分钟最多 10 次尝试，成功不算次数；达到上限后直接禁止访问（403）。
// 全内存保存，重启服务即可清理尝试记录与禁止访问状态。
type LoginLimiter struct {
	mu          sync.Mutex
	maxAttempts int
	window      time.Duration
	attempts    map[string][]time.Time
	blocked     map[string]bool
}

// NewLoginLimiter 创建登录频率限制器。
func NewLoginLimiter(maxAttempts int, window time.Duration) *LoginLimiter {
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &LoginLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		attempts:    make(map[string][]time.Time),
		blocked:     make(map[string]bool),
	}
}

// IsBlocked 检查给定客户端 IP 是否已被禁止访问。
func (l *LoginLimiter) IsBlocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.blocked[ip]
}

// RecordSuccess 记录一次成功验证：成功不算次数，未被封禁时重置该 IP 的失败历史。
func (l *LoginLimiter) RecordSuccess(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.blocked[ip] {
		delete(l.attempts, ip)
	}
}

// RecordFailure 记录一次失败尝试。
// 若 1 分钟内的失败次数达到上限（默认 10 次），将其置为禁止访问状态并返回 false。
// 返回 allowed 为 false 表示已超限被直接禁止访问。
func (l *LoginLimiter) RecordFailure(ip string) (allowed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.blocked[ip] {
		return false
	}

	now := time.Now()
	cutoff := now.Add(-l.window)

	var recent []time.Time
	for _, t := range l.attempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	recent = append(recent, now)
	l.attempts[ip] = recent

	if len(recent) >= l.maxAttempts {
		l.blocked[ip] = true
		return false
	}
	return true
}

// Reset 清理限制器状态（主要用于测试或显式重置）。
func (l *LoginLimiter) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts = make(map[string][]time.Time)
	l.blocked = make(map[string]bool)
}

// ClientIP 解析请求客户端真实 IP，兼容反向代理标头与直连地址。
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
		return rip
	}
	if r.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil && host != "" {
			return host
		}
		return strings.TrimSpace(r.RemoteAddr)
	}
	return "127.0.0.1"
}
