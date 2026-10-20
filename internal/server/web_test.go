package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AaronConlon/upy/internal/config"
)

func TestTOTPValidation(t *testing.T) {
	// 使用固定 Secret 测试 TOTP 生成与校验
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	now := time.Now()

	code, err := generateTOTP(secret, now)
	if err != nil {
		t.Fatalf("生成 TOTP 失败: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("TOTP 验证码长度不为 6: %s", code)
	}

	tmpDir := t.TempDir()
	totpPath := filepath.Join(tmpDir, "totp.json")
	tm, err := NewTOTPManager(totpPath, secret)
	if err != nil {
		t.Fatalf("创建 TOTPManager 失败: %v", err)
	}

	if !tm.Validate(code) {
		t.Fatalf("有效验证码校验失败: %s", code)
	}
	if tm.Validate("000000") && code != "000000" {
		t.Fatalf("错误验证码不应通过校验")
	}
}

func TestTOTPQuotaAndLimit(t *testing.T) {
	tmpDir := t.TempDir()
	totpPath := filepath.Join(tmpDir, "totp.json")
	tm, err := NewTOTPManager(totpPath, "")
	if err != nil {
		t.Fatalf("创建 TOTPManager 失败: %v", err)
	}

	maxViews := 5
	for i := 1; i <= maxViews; i++ {
		views, allowed, err := tm.RecordView(maxViews)
		if err != nil {
			t.Fatalf("第 %d 次记录失败: %v", i, err)
		}
		if !allowed || views != i {
			t.Fatalf("第 %d 次访问应该允许，得到 allowed=%v, views=%d", i, allowed, views)
		}
	}

	// 第 6 次应被拒绝
	views, allowed, err := tm.RecordView(maxViews)
	if err != nil {
		t.Fatalf("超限记录失败: %v", err)
	}
	if allowed {
		t.Fatalf("第 6 次访问应该被拒绝，实际 allowed=%v, views=%d", allowed, views)
	}
}

func TestSessionStore(t *testing.T) {
	store := NewSessionStore()

	token := store.CreateSession()
	if token == "" {
		t.Fatalf("生成的 token 不能为空")
	}
	if !store.Validate(token) {
		t.Fatalf("新建的 session 应该有效")
	}

	// 撤销
	store.Revoke(token)
	if store.Validate(token) {
		t.Fatalf("撤销后的 session 应失效")
	}

	// 过期
	token2 := store.CreateSession()
	store.sessions[token2] = time.Now().Add(-1 * time.Minute)
	if store.Validate(token2) {
		t.Fatalf("过期 session 应失效")
	}
}

func TestWebRoutesFlow(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(tmpDir, "config.yaml"))

	totpPath := filepath.Join(tmpDir, "totp.json")
	fixedSecret := "JBSWY3DPEHPK3PXP"
	tm, err := NewTOTPManager(totpPath, fixedSecret)
	if err != nil {
		t.Fatalf("创建 TOTPManager 失败: %v", err)
	}

	srv := NewServer(config.ServerConfig{
		Listen:        "127.0.0.1:0",
		TOTPMaxViews:  5,
		MaxLogHistory: 10,
	}, nil)
	srv.SetTOTPManager(tm)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// 1. 访问 /auth 页面 (最多 5 次)
	for i := 1; i <= 5; i++ {
		req, _ := http.NewRequest("GET", "/auth", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("第 %d 次访问 /auth 期望 200, 得到 %d", i, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "两步验证 (2FA) 绑定") {
			t.Fatalf("页面内容缺失关键词")
		}
		if !strings.Contains(rr.Body.String(), "data:image/png;base64,") {
			t.Fatalf("页面应包含二维码数据")
		}
	}

	// 第 6 次访问 /auth 必须返回 404
	req6, _ := http.NewRequest("GET", "/auth", nil)
	rr6 := httptest.NewRecorder()
	mux.ServeHTTP(rr6, req6)
	if rr6.Code != http.StatusNotFound {
		t.Fatalf("超过 5 次后访问 /auth 必须返回 404, 得到 %d", rr6.Code)
	}

	// 2. 未登录访问 /logs，应 302 重定向到 /
	reqLogs, _ := http.NewRequest("GET", "/logs", nil)
	rrLogs := httptest.NewRecorder()
	mux.ServeHTTP(rrLogs, reqLogs)
	if rrLogs.Code != http.StatusFound || rrLogs.Header().Get("Location") != "/" {
		t.Fatalf("未登录访问 /logs 应重定向到 /, 得到 code=%d, loc=%s", rrLogs.Code, rrLogs.Header().Get("Location"))
	}

	// 3. 访问 / 显示登录输入框
	reqRoot, _ := http.NewRequest("GET", "/", nil)
	rrRoot := httptest.NewRecorder()
	mux.ServeHTTP(rrRoot, reqRoot)
	if rrRoot.Code != http.StatusOK || !strings.Contains(rrRoot.Body.String(), "验证身份") {
		t.Fatalf("首页应展示登录验证表单")
	}

	// 4. 输入错误验证码登录 -> 401
	formWrong := url.Values{"code": {"999999"}}
	reqPostWrong, _ := http.NewRequest("POST", "/", strings.NewReader(formWrong.Encode()))
	reqPostWrong.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrPostWrong := httptest.NewRecorder()
	mux.ServeHTTP(rrPostWrong, reqPostWrong)
	if rrPostWrong.Code != http.StatusUnauthorized {
		t.Fatalf("错误验证码应返回 401, 得到 %d", rrPostWrong.Code)
	}

	// 5. 输入正确验证码登录 -> 302 重定向到 /logs 并分配 cookie
	validCode, _ := generateTOTP(fixedSecret, time.Now())
	formValid := url.Values{"code": {validCode}}
	reqPostValid, _ := http.NewRequest("POST", "/", strings.NewReader(formValid.Encode()))
	reqPostValid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rrPostValid := httptest.NewRecorder()
	mux.ServeHTTP(rrPostValid, reqPostValid)

	if rrPostValid.Code != http.StatusFound || rrPostValid.Header().Get("Location") != "/logs" {
		t.Fatalf("正确验证码应重定向到 /logs, 得到 code=%d, loc=%s", rrPostValid.Code, rrPostValid.Header().Get("Location"))
	}

	cookies := rrPostValid.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("登录成功后未分配 session cookie")
	}
	if sessionCookie.MaxAge != 1800 {
		t.Fatalf("Session cookie 有效期应为 1800 秒 (30 分钟), 得到 %d", sessionCookie.MaxAge)
	}

	// 6. 携带有效 Cookie 访问 /logs -> 200
	reqLogsAuth, _ := http.NewRequest("GET", "/logs", nil)
	reqLogsAuth.AddCookie(sessionCookie)
	rrLogsAuth := httptest.NewRecorder()
	mux.ServeHTTP(rrLogsAuth, reqLogsAuth)
	if rrLogsAuth.Code != http.StatusOK {
		t.Fatalf("携带有效 cookie 访问 /logs 期望 200, 得到 %d", rrLogsAuth.Code)
	}
	if !strings.Contains(rrLogsAuth.Body.String(), "部署执行日志") {
		t.Fatalf("页面内容缺失部署日志标题")
	}

	// 7. 制造日志文件并测试详情页
	projLogDir := filepath.Join(tmpDir, "logs", "web-demo")
	_ = os.MkdirAll(projLogDir, 0o755)
	logFile := filepath.Join(projLogDir, "task-abc123.log")
	_ = os.WriteFile(logFile, []byte("\x1b[32m✓\x1b[0m 部署成功"), 0o644)

	// 未带 Cookie 访问详情页 -> 302 重定向
	reqDetailUnauth, _ := http.NewRequest("GET", "/logs/task-abc123", nil)
	rrDetailUnauth := httptest.NewRecorder()
	mux.ServeHTTP(rrDetailUnauth, reqDetailUnauth)
	if rrDetailUnauth.Code != http.StatusFound {
		t.Fatalf("未认证访问详情页应重定向")
	}

	// 带 Cookie 访问详情页 -> 200
	reqDetailAuth, _ := http.NewRequest("GET", "/logs/task-abc123", nil)
	reqDetailAuth.AddCookie(sessionCookie)
	rrDetailAuth := httptest.NewRecorder()
	mux.ServeHTTP(rrDetailAuth, reqDetailAuth)
	if rrDetailAuth.Code != http.StatusOK {
		t.Fatalf("携带 Cookie 访问详情页期望 200, 得到 %d", rrDetailAuth.Code)
	}
	if !strings.Contains(rrDetailAuth.Body.String(), "color:#3fb950") {
		t.Fatalf("ANSI 转义未被转为带颜色的 span: %s", rrDetailAuth.Body.String())
	}

	// 8. 退出登录 -> 清理 cookie 并重定向到 /
	reqLogout, _ := http.NewRequest("POST", "/logout", nil)
	reqLogout.AddCookie(sessionCookie)
	rrLogout := httptest.NewRecorder()
	mux.ServeHTTP(rrLogout, reqLogout)
	if rrLogout.Code != http.StatusFound || rrLogout.Header().Get("Location") != "/" {
		t.Fatalf("退出登录后应重定向到 /")
	}
}

func TestRenderAnsiToHTML(t *testing.T) {
	input := "  \x1b[32m✓\x1b[0m 部署成功: \x1b[96mcodia\x1b[0m \x1b[31m错误: 退出码 1\x1b[0m"
	got := RenderAnsiToHTML(input)
	if !strings.Contains(got, `<span style="color:#3fb950">✓</span>`) {
		t.Fatalf("绿色高亮不对: %s", got)
	}
	if !strings.Contains(got, `<span style="color:#38bdf8">codia</span>`) {
		t.Fatalf("天蓝色高亮不对: %s", got)
	}
	if !strings.Contains(got, `<span style="color:#f85149">错误: 退出码 1</span>`) {
		t.Fatalf("红色高亮不对: %s", got)
	}

	// XSS 安全校验
	xssInput := "<script>alert(1)</script>"
	xssGot := RenderAnsiToHTML(xssInput)
	if strings.Contains(xssGot, "<script>") {
		t.Fatalf("HTML 标签未被正确转义")
	}
}
