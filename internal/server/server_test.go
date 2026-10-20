package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
)

func TestAuth(t *testing.T) {
	globalToken := "global-secret-123"
	projectSecret := "proj-secret-456"

	// 1. 无任何 secret
	req, _ := http.NewRequest("POST", "/webhook", nil)
	if err := Authenticate(req, "", ""); err != ErrNoSecret {
		t.Fatalf("期望 ErrNoSecret, 得到 %v", err)
	}

	// 2. Bearer Token 匹配全局 Token
	req, _ = http.NewRequest("POST", "/webhook", nil)
	req.Header.Set("Authorization", "Bearer global-secret-123")
	if err := Authenticate(req, globalToken, projectSecret); err != nil {
		t.Fatalf("Bearer 全局 token 鉴权失败: %v", err)
	}

	// 3. X-Upy-Token 匹配项目专属 Secret
	req, _ = http.NewRequest("POST", "/webhook", nil)
	req.Header.Set("X-Upy-Token", "proj-secret-456")
	if err := Authenticate(req, globalToken, projectSecret); err != nil {
		t.Fatalf("X-Upy-Token 项目 secret 鉴权失败: %v", err)
	}

	// 4. Token 错误
	req, _ = http.NewRequest("POST", "/webhook", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	if err := Authenticate(req, globalToken, projectSecret); err != ErrUnauthorized {
		t.Fatalf("期望 ErrUnauthorized, 得到 %v", err)
	}

	// 5. GitHub HMAC 签名匹配
	body := []byte(`{"action":"release","release":{"tag_name":"v1.0.0"}}`)
	mac := hmac.New(sha256.New, []byte(projectSecret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, _ = http.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	if err := Authenticate(req, globalToken, projectSecret); err != nil {
		t.Fatalf("GitHub HMAC 鉴权失败: %v", err)
	}

	// 6. GitHub HMAC 签名错误
	req, _ = http.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalidhex")
	if err := Authenticate(req, globalToken, projectSecret); err != ErrUnauthorized {
		t.Fatalf("期望 ErrUnauthorized, 得到 %v", err)
	}
}

func TestTaskManager(t *testing.T) {
	tm := NewTaskManager()

	task, err := tm.CreateTask("app-test", "/tmp/app", "latest", false, "/tmp/app.log")
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if task.Status != StatusRunning {
		t.Fatalf("期望状态 running, 得到 %s", task.Status)
	}

	// 并发同一项目报错
	_, err = tm.CreateTask("app-test", "/tmp/app", "latest", false, "/tmp/app2.log")
	if err == nil {
		t.Fatalf("同一项目重叠应报错 ErrProjectBusy")
	}

	// 查活跃任务
	active, ok := tm.GetActiveTask("app-test")
	if !ok || active.ID != task.ID {
		t.Fatalf("获取活跃任务失败: %+v", active)
	}

	// 结束任务
	tm.FinishTask(task.ID, 0, nil)
	finished, ok := tm.Get(task.ID)
	if !ok || finished.Status != StatusSuccess {
		t.Fatalf("期望任务成功, 得到: %+v", finished)
	}

	// 结束后可以再次创建
	task2, err := tm.CreateTask("app-test", "/tmp/app", "v2.0.0", true, "/tmp/app3.log")
	if err != nil {
		t.Fatalf("结束后重新创建失败: %v", err)
	}
	if task2.Target != "v2.0.0" || !task2.Force {
		t.Fatalf("任务参数不对: %+v", task2)
	}
}

func TestTaskLoggerAndPrune(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(tmpDir, "config.yaml"))

	log.RegisterSecret("my-secret-token")

	logger, err := NewTaskLogger("test-proj", "task-001", nil)
	if err != nil {
		t.Fatalf("创建 TaskLogger 失败: %v", err)
	}

	_, err = logger.Write([]byte("start deploy with my-secret-token ...\n"))
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	_ = logger.Close()

	// 校验文件内容已脱敏
	content, err := os.ReadFile(logger.Path())
	if err != nil {
		t.Fatalf("读取日志文件失败: %v", err)
	}
	if strings.Contains(string(content), "my-secret-token") {
		t.Fatalf("日志中包含未脱敏 secret: %s", string(content))
	}
	if !strings.Contains(string(content), "***") {
		t.Fatalf("日志未包含脱敏 ***: %s", string(content))
	}

	// 制造多个历史日志测试 PruneProjectLogs
	for i := 2; i <= 5; i++ {
		l, _ := NewTaskLogger("test-proj", fmt.Sprintf("task-00%d", i), nil)
		_, _ = l.Write([]byte("line\n"))
		_ = l.Close()
		time.Sleep(10 * time.Millisecond) // 保证时间戳递增
	}

	// 保留最新的 2 份
	deleted, err := PruneProjectLogs("test-proj", 2)
	if err != nil {
		t.Fatalf("PruneProjectLogs 失败: %v", err)
	}
	if len(deleted) != 3 {
		t.Fatalf("期望删除 3 份旧日志, 实际删除 %d: %v", len(deleted), deleted)
	}
}

func TestServerEndpoints(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(tmpDir, "config.yaml"))

	// 注册本地项目
	projRoot := filepath.Join(tmpDir, "my-service")
	_ = os.MkdirAll(projRoot, 0o755)
	deployYAML := `name: my-service
release:
  provider: github
  repository: acme/service
webhook:
  secret: "proj-secret"
`
	_ = os.WriteFile(filepath.Join(projRoot, "deploy.yaml"), []byte(deployYAML), 0o644)
	if err := config.SaveLocalProject("my-service", projRoot); err != nil {
		t.Fatalf("SaveLocalProject 失败: %v", err)
	}

	var serverBuf bytes.Buffer
	srv := NewServer(config.ServerConfig{
		Listen: "127.0.0.1:0",
		Token:  "global-sec",
	}, &serverBuf)

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// 1. 测试 Health
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Health 响应码不对: %d", rr.Code)
	}

	// 2. 测试 Webhook 触发 (未鉴权 -> 401)
	req, _ = http.NewRequest("POST", "/api/v1/webhook/my-service", nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("未鉴权应返回 401, 得到 %d", rr.Code)
	}

	// 3. 测试 Webhook 触发不存在项目 -> 404
	req, _ = http.NewRequest("POST", "/api/v1/webhook/not-found", nil)
	req.Header.Set("Authorization", "Bearer global-sec")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("未登记项目应返回 404, 得到 %d", rr.Code)
	}

	// 4. 测试 Webhook 触发成功 -> 202
	reqBody := `{"target":"v1.2.3","force":true}`
	req, _ = http.NewRequest("POST", "/api/v1/webhook/my-service", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer proj-secret")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("合法触发应返回 202, 得到 %d: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Code   int    `json:"code"`
		TaskID string `json:"task_id"`
		LogURL string `json:"log_url"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.TaskID == "" || resp.Status != "running" {
		t.Fatalf("响应数据异常: %+v", resp)
	}

	// 5. 立即查询任务状态
	req, _ = http.NewRequest("GET", "/api/v1/tasks/"+resp.TaskID, nil)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("查询任务状态失败: %d", rr.Code)
	}

	// 6. 重复触发该项目 -> 409 Conflict
	req, _ = http.NewRequest("POST", "/api/v1/webhook/my-service", nil)
	req.Header.Set("Authorization", "Bearer proj-secret")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("重复触发应返回 409 Conflict, 得到 %d", rr.Code)
	}

	// 7. 测试查询项目列表 (需鉴权)
	req, _ = http.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer global-sec")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("查询项目列表失败: %d", rr.Code)
	}
}

func TestPIDFileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(tmpDir, "config.yaml"))

	if err := WritePID(12345); err != nil {
		t.Fatalf("WritePID 失败: %v", err)
	}

	pid, err := ReadPID()
	if err != nil {
		t.Fatalf("ReadPID 失败: %v", err)
	}
	if pid != 12345 {
		t.Fatalf("PID 不匹配: %d", pid)
	}

	if err := RemovePID(); err != nil {
		t.Fatalf("RemovePID 失败: %v", err)
	}

	_, err = ReadPID()
	if err == nil {
		t.Fatalf("删除后 ReadPID 应报错")
	}
}
