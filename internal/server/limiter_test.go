package server

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoginLimiterMaxAttempts(t *testing.T) {
	limiter := NewLoginLimiter(10, time.Minute)
	ip := "192.168.1.100"

	if limiter.IsBlocked(ip) {
		t.Fatal("new IP should not be blocked")
	}

	// 1 到 9 次失败，仍在允许范围内
	for i := 1; i <= 9; i++ {
		allowed := limiter.RecordFailure(ip)
		if !allowed {
			t.Fatalf("attempt %d should be allowed", i)
		}
		if limiter.IsBlocked(ip) {
			t.Fatalf("attempt %d should not block the IP", i)
		}
	}

	// 第 10 次失败，达到每分钟上限，直接禁止访问
	allowed := limiter.RecordFailure(ip)
	if allowed {
		t.Fatal("10th attempt should reach limit and not be allowed")
	}
	if !limiter.IsBlocked(ip) {
		t.Fatal("IP should be marked as blocked after 10 failed attempts")
	}

	// 后续再次尝试，直接拒绝
	if limiter.RecordFailure(ip) {
		t.Fatal("subsequent attempt after block must be rejected")
	}
}

func TestLoginLimiterSuccessNotCounted(t *testing.T) {
	limiter := NewLoginLimiter(10, time.Minute)
	ip := "192.168.1.101"

	// 失败 5 次
	for i := 1; i <= 5; i++ {
		if !limiter.RecordFailure(ip) {
			t.Fatalf("attempt %d should be allowed", i)
		}
	}

	// 成功验证，成功不算次数且重置未封禁的失败历史
	limiter.RecordSuccess(ip)
	if limiter.IsBlocked(ip) {
		t.Fatal("successful login should not block IP")
	}

	// 再次失败 9 次，仍然不会被封禁
	for i := 1; i <= 9; i++ {
		if !limiter.RecordFailure(ip) {
			t.Fatalf("subsequent attempt %d should be allowed after reset", i)
		}
	}
	if limiter.IsBlocked(ip) {
		t.Fatal("IP should not be blocked yet")
	}
}

func TestLoginLimiterWindowExpiry(t *testing.T) {
	limiter := NewLoginLimiter(10, 50*time.Millisecond)
	ip := "192.168.1.102"

	// 快速失败 5 次
	for i := 0; i < 5; i++ {
		limiter.RecordFailure(ip)
	}

	// 等待窗口过期
	time.Sleep(60 * time.Millisecond)

	// 再次失败 5 次，由于前面的已过期，总数仍不会达到 10
	for i := 0; i < 5; i++ {
		if !limiter.RecordFailure(ip) {
			t.Fatalf("attempt %d after expiry should be allowed", i)
		}
	}
	if limiter.IsBlocked(ip) {
		t.Fatal("IP should not be blocked as old attempts expired")
	}
}

func TestLoginLimiterResetOnRestart(t *testing.T) {
	limiter := NewLoginLimiter(10, time.Minute)
	ip := "192.168.1.103"

	// 触发封禁
	for i := 0; i < 10; i++ {
		limiter.RecordFailure(ip)
	}
	if !limiter.IsBlocked(ip) {
		t.Fatal("IP should be blocked")
	}

	// 模拟服务重启（清理内存状态）
	limiter.Reset()
	if limiter.IsBlocked(ip) {
		t.Fatal("IP block should be cleared after reset/restart")
	}
	if !limiter.RecordFailure(ip) {
		t.Fatal("should allow fresh attempts after reset")
	}
}

func TestClientIPResolution(t *testing.T) {
	// X-Forwarded-For (多 IP 取首个)
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
	if ip := ClientIP(req1); ip != "203.0.113.195" {
		t.Fatalf("expected 203.0.113.195, got %s", ip)
	}

	// X-Real-IP
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("X-Real-IP", "198.51.100.22")
	if ip := ClientIP(req2); ip != "198.51.100.22" {
		t.Fatalf("expected 198.51.100.22, got %s", ip)
	}

	// RemoteAddr 带端口
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.RemoteAddr = "192.168.1.50:54321"
	if ip := ClientIP(req3); ip != "192.168.1.50" {
		t.Fatalf("expected 192.168.1.50, got %s", ip)
	}
}
