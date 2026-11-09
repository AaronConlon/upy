package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSSE_ProtectedEndpoint(t *testing.T) {
	_, mux, _, _ := newConsoleFixture(t)
	// 未登录访问 /logs/archive-ok/stream 应 302 重定向到 /
	rec := consoleRequest(mux, "/logs/archive-ok/stream", nil)
	if rec.Code != http.StatusFound {
		t.Fatalf("未登录访问 SSE 应重定向，得到 status=%d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("重定向目标应为 /，得到 %q", loc)
	}
}

func TestSSE_NotFound(t *testing.T) {
	_, mux, cookie, _ := newConsoleFixture(t)
	rec := consoleRequest(mux, "/logs/non-existent-task/stream", cookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在的日志应返回 404，得到 status=%d", rec.Code)
	}
}

func TestSSE_CompletedTaskStream(t *testing.T) {
	_, mux, cookie, _ := newConsoleFixture(t)
	rec := consoleRequest(mux, "/logs/archive-ok/stream", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("归档日志 SSE 请求应返回 200，得到 status=%d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type 应为 text/event-stream，得到 %q", ct)
	}
	if buf := rec.Header().Get("X-Accel-Buffering"); buf != "no" {
		t.Fatalf("X-Accel-Buffering 应为 no，得到 %q", buf)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: log") {
		t.Fatalf("归档日志应发送 event: log，body=%q", body)
	}
	if !strings.Contains(body, "event: status") {
		t.Fatalf("归档日志应发送 event: status，body=%q", body)
	}
	if !strings.Contains(body, "event: end") {
		t.Fatalf("归档日志应发送 event: end，body=%q", body)
	}
}

func TestSSE_LastEventIDOffset(t *testing.T) {
	_, mux, cookie, root := newConsoleFixture(t)
	logFile := filepath.Join(root, "logs", "demo", "offset-test.log")
	initialContent := "line 1: hello\nline 2: world\n部署任务结束 (退出码: 0)\n"
	if err := os.WriteFile(logFile, []byte(initialContent), 0600); err != nil {
		t.Fatal(err)
	}

	// 指定 offset 为第 1 行之后
	line1Len := len("line 1: hello\n")
	rec := consoleRequest(mux, fmt.Sprintf("/logs/offset-test/stream?last_event_id=%d", line1Len), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("SSE 请求应返回 200，得到 %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "line 1: hello") {
		t.Fatalf("从 offset 读取时不应包含 line 1 内容，body=%q", body)
	}
	if !strings.Contains(body, "line 2: world") {
		t.Fatalf("从 offset 读取应包含 line 2 内容，body=%q", body)
	}
}

func TestSSE_LiveTaskStreaming(t *testing.T) {
	srv, mux, cookie, root := newConsoleFixture(t)
	taskLog := filepath.Join(root, "logs", "demo", "live-task.log")
	if err := os.WriteFile(taskLog, []byte("start deploying...\n"), 0600); err != nil {
		t.Fatal(err)
	}

	task, err := srv.taskMgr.CreateTask("demo", filepath.Join(root, "app"), "v2.0.0", false, taskLog)
	if err != nil {
		t.Fatal(err)
	}

	// 创建可取消的上下文模拟长连接
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/logs/"+task.ID+"/stream?last_event_id=0", nil).WithContext(ctx)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rec, req)
		close(done)
	}()

	// 稍作等待确保流建立
	time.Sleep(150 * time.Millisecond)

	// 追加新日志
	f, err := os.OpenFile(taskLog, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("step 1: build completed\n")
	_ = f.Close()

	// 等待流读取增量
	time.Sleep(250 * time.Millisecond)

	// 完成任务
	srv.taskMgr.FinishTask(task.ID, 0, nil)

	// 等待流捕获任务完成事件并主动退出
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("活跃任务结束后，SSE 流未在超时时间内自动结束")
	}

	body := rec.Body.String()
	if !strings.Contains(body, "step 1: build completed") {
		t.Fatalf("应捕获到追加的日志增量，body=%q", body)
	}
	if !strings.Contains(body, "event: status") {
		t.Fatalf("任务完成应发送 event: status，body=%q", body)
	}
	if !strings.Contains(body, "event: end") {
		t.Fatalf("任务完成应发送 event: end，body=%q", body)
	}
}
