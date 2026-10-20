package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/state"
)

func newConsoleFixture(t *testing.T) (*Server, *http.ServeMux, *http.Cookie, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(root, "config.yaml"))
	projectRoot := filepath.Join(root, "app")
	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "deploy.yaml"), []byte("name: demo\nrelease:\n  provider: github\n  repository: owner/demo\nwebhook:\n  secret: never-render-this-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := config.SaveLocalProject("demo", projectRoot); err != nil {
		t.Fatal(err)
	}
	if err := state.WriteAtomic(projectRoot, &state.State{CurrentVersion: "v1.2.3", PreviousVersion: "v1.2.2", Type: "system", UnitName: "upy-demo.service", DeployedAt: "2026-10-20T14:32:08+08:00"}); err != nil {
		t.Fatal(err)
	}
	logDir := filepath.Join(root, "logs", "demo")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"archive-ok": "部署任务结束 (退出码: 0)\n", "archive-failed": "部署任务结束 (退出码: 17)\n", "archive-incomplete": "准备部署\n"} {
		if err := os.WriteFile(filepath.Join(logDir, name+".log"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	srv := NewServer(config.ServerConfig{}, nil)
	mux := http.NewServeMux()
	srv.registerRoutes(mux)
	cookie := &http.Cookie{Name: SessionCookieName, Value: srv.sessions.CreateSession()}
	return srv, mux, cookie, root
}
func consoleRequest(mux *http.ServeMux, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}
func TestConsoleProtectedRoutes(t *testing.T) {
	_, mux, cookie, _ := newConsoleFixture(t)
	for _, path := range []string{"/overview", "/projects", "/projects/demo", "/logs", "/logs/archive-ok", "/logs/archive-ok?format=json", "/logs/archive-ok?download=1"} {
		t.Run(path, func(t *testing.T) {
			unauth := consoleRequest(mux, path, nil)
			if unauth.Code != 302 || unauth.Header().Get("Location") != "/" {
				t.Fatalf("unprotected route %s: %d", path, unauth.Code)
			}
			got := consoleRequest(mux, path, cookie)
			if got.Code != 200 {
				t.Fatalf("%s: %d %s", path, got.Code, got.Body.String())
			}
			if got.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("sensitive response must not be cached")
			}
			if strings.Contains(got.Body.String(), "never-render-this-secret") {
				t.Fatal("project secret leaked")
			}
		})
	}
	if got := consoleRequest(mux, "/projects/missing", cookie); got.Code != 404 {
		t.Fatalf("unknown project: %d", got.Code)
	}
	for _, path := range []string{"/logs/missing", "/logs/%2e%2e%2fconfig", "/logs/%5cconfig"} {
		if got := consoleRequest(mux, path, cookie); got.Code == 200 {
			t.Fatalf("unexpected readable path %s", path)
		}
	}
	cookie.Value = "expired"
	if got := consoleRequest(mux, "/overview", cookie); got.Code != 302 {
		t.Fatal("expired session accepted")
	}
}
func TestConsoleUsesRealProjectState(t *testing.T) {
	srv, mux, cookie, _ := newConsoleFixture(t)
	projects, err := srv.collectProjects(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := projects[0]
	if p.Version != "v1.2.3" || p.Type != "system" || p.Unit != "upy-demo.service" || p.Status != "success" {
		t.Fatalf("unexpected project %+v", p)
	}
	response := consoleRequest(mux, "/projects/demo", cookie).Body.String()
	for _, want := range []string{"v1.2.3", "v1.2.2", "upy-demo.service", "owner/demo", "/logs?project=demo"} {
		if !strings.Contains(response, want) {
			t.Fatalf("missing %s", want)
		}
	}
	// 没有日志的项目仍应列出，不从任务记录推导注册项目总数。
	task, err := srv.taskMgr.CreateTask("demo", p.Root, "v1.3.0", false, "")
	if err != nil {
		t.Fatal(err)
	}
	projects, err = srv.collectProjects(collectLogItems(srv.taskMgr))
	if err != nil || projects[0].Status != "running" {
		t.Fatal("running task missing")
	}
	srv.taskMgr.FinishTask(task.ID, 1, nil)
	projects, _ = srv.collectProjects(collectLogItems(srv.taskMgr))
	if projects[0].Status != "failed" || projects[0].Version != "v1.2.3" {
		t.Fatal("failed task must not replace current version")
	}
	if got := consoleRequest(mux, "/projects?q=unmatched", cookie).Body.String(); strings.Contains(got, `href="/projects/demo"`) {
		t.Fatal("project search ignored")
	}
}
func TestArchivedLogStatusAndDetailAgree(t *testing.T) {
	_, mux, cookie, _ := newConsoleFixture(t)
	for id, want := range map[string]string{"archive-ok": "部署成功", "archive-failed": "部署失败", "archive-incomplete": "结果未知"} {
		got := consoleRequest(mux, "/logs/"+id+"?format=json", cookie)
		var data map[string]any
		if err := json.Unmarshal(got.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if data["status_label"] != want {
			t.Fatalf("%s status = %v", id, data)
		}
		if id == "archive-failed" && data["exit_label"] != "EXIT CODE 17" {
			t.Fatalf("lost historical exit code: %v", data)
		}
	}
	for filter, id := range map[string]string{"failed": "archive-failed", "unknown": "archive-incomplete", "success": "archive-ok"} {
		got := consoleRequest(mux, "/logs?status="+filter, cookie).Body.String()
		for _, candidate := range []string{"archive-ok", "archive-failed", "archive-incomplete"} {
			if strings.Contains(got, `href="/logs/`+candidate+`"`) != (candidate == id) {
				t.Fatalf("status filter %s incorrect", filter)
			}
		}
	}
}
func TestConsoleLogEscapingDownloadAndTruncation(t *testing.T) {
	srv, mux, cookie, root := newConsoleFixture(t)
	payload := strings.Repeat("long log line\n", 30000) + "<script>alert('xss')</script>\n部署任务结束 (退出码: 1)\n"
	path := filepath.Join(root, "logs", "demo", "large.log")
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	d, _, err := srv.consoleLogDetail("large")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Truncated || int64(len(d.Raw)) > maxConsoleLogBytes {
		t.Fatal("large log not bounded")
	}
	if strings.Contains(string(d.HTML), "<script>") {
		t.Fatal("raw log injected into HTML")
	}
	got := consoleRequest(mux, "/logs/large?download=1", cookie)
	if got.Body.String() != payload || !strings.Contains(got.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("download must contain full original log")
	}
	html := renderConsole(ConsolePage{Title: `<script>alert(1)</script>`, Screen: "project", Auth: true, Project: &ProjectView{Name: `<img src=x onerror=alert(1)>`, Root: `<script>`, Version: `<script>`}})
	if strings.Contains(html, `<img src=x`) || strings.Contains(html, `<script>alert(1)`) {
		t.Fatal("metadata not escaped")
	}
	// ANSI HTML 对日志片段转义；模板只信任该转换函数的输出。
	if RenderAnsiToHTML(`<svg onload=alert(1)>`) == string(template.HTML(`<svg onload=alert(1)>`)) {
		t.Fatal("log escaping missing")
	}
}
func TestConsoleLogPagination(t *testing.T) {
	_, mux, cookie, root := newConsoleFixture(t)
	for i := 0; i < 25; i++ {
		if err := os.WriteFile(filepath.Join(root, "logs", "demo", fmt.Sprintf("page-%02d.log", i)), []byte("部署任务结束 (退出码: 0)\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got := consoleRequest(mux, "/logs?project=demo&status=success&q=page-&page=1", cookie).Body.String()
	if n := strings.Count(got, `class="event"`); n != 20 {
		t.Fatalf("page size = %d", n)
	}
	if !strings.Contains(got, "下一页") || !strings.Contains(got, "project=demo") {
		t.Fatal("pagination lost filters")
	}
	got = consoleRequest(mux, "/logs?project=demo&status=success&q="+url.QueryEscape("page-")+"&page=999", cookie).Body.String()
	if n := strings.Count(got, `class="event"`); n != 5 {
		t.Fatalf("last page count = %d", n)
	}
}

func TestConsoleLiveLogLifecycle(t *testing.T) {
	srv, mux, cookie, root := newConsoleFixture(t)
	task, err := srv.taskMgr.CreateTask("demo", filepath.Join(root, "app"), "v2.0.0", false, "")
	if err != nil {
		t.Fatal(err)
	}
	initial := consoleRequest(mux, "/logs/"+task.ID+"?format=json", cookie)
	var data map[string]any
	if err := json.Unmarshal(initial.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data["live"] != true || !strings.Contains(data["html"].(string), "日志文件尚未生成") {
		t.Fatalf("missing running placeholder: %v", data)
	}
	if err := os.WriteFile(task.LogFile, []byte("live output\n"), 0600); err != nil {
		t.Fatal(err)
	}
	srv.taskMgr.FinishTask(task.ID, 0, nil)
	response := consoleRequest(mux, "/logs/"+task.ID+"?format=json", cookie)
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data["live"] != false || data["status_label"] != "部署成功" || data["html"] != "live output\n" {
		t.Fatalf("finished state missing: %v", data)
	}
	// 较新的 CLI 部署成功状态应覆盖旧任务失败，而不会丢失旧任务日志。
	projectRoot := filepath.Join(root, "app")
	if err := state.WriteAtomic(projectRoot, &state.State{CurrentVersion: "v3", Type: "system", DeployedAt: "2099-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	projects, err := srv.collectProjects([]LogItemView{{ID: "old", Project: "demo", Timestamp: "2026-01-01 00:00:00", Status: "failed"}})
	if err != nil || projects[0].Status != "success" || projects[0].Latest.ID != "old" {
		t.Fatalf("new CLI state ignored: %+v %v", projects, err)
	}
}
