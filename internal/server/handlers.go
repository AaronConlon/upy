package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/version"
)

type WebhookRequest struct {
	Target string `json:"target"`
	Force  bool   `json:"force"`
}

type JSONResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
 s.registerPWARoutes(mux)
	// API 接口
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/webhook/{project}", s.handleWebhook)
	mux.HandleFunc("GET /api/v1/tasks/{id}", s.handleGetTask)
	mux.HandleFunc("GET /api/v1/tasks/{id}/logs", s.handleGetTaskLogs)
	mux.HandleFunc("GET /api/v1/projects", s.handleListProjects)

	// Web 认证与两步验证 (TOTP 2FA) 路由
	mux.HandleFunc("GET /{$}", s.handleRootGet)
	mux.HandleFunc("POST /{$}", s.handleRootPost)
	mux.HandleFunc("POST /logout", s.handleLogout)

	// 受保护的 Web 日志路由 (Session 校验，失效重定向到 /)
	mux.HandleFunc("GET /overview", s.requireAuth(s.handleOverviewPage))
	mux.HandleFunc("GET /projects", s.requireAuth(s.handleProjectsPage))
	mux.HandleFunc("GET /projects/{name}", s.requireAuth(s.handleProjectPage))
	mux.HandleFunc("GET /logs", s.requireAuth(s.handleConsoleLogsPage))
	mux.HandleFunc("GET /logs/{id}", s.requireAuth(s.handleConsoleLogDetail))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(s.startedAt).Round(time.Second).String()
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    0,
		"status":  "ok",
		"version": version.String(),
		"uptime":  uptime,
	})
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	projectName := strings.TrimSpace(r.PathValue("project"))
	if projectName == "" {
		writeJSON(w, http.StatusBadRequest, JSONResponse{Code: 400, Error: "缺少项目名称"})
		return
	}

	// 1. 查找项目是否存在
	localProjects, err := config.ListLocalProjects()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, JSONResponse{Code: 500, Error: "读取本地项目列表失败: " + err.Error()})
		return
	}

	var targetProject *config.LocalProject
	for _, lp := range localProjects {
		if lp.Name == projectName {
			targetProject = &lp
			break
		}
	}
	if targetProject == nil {
		writeJSON(w, http.StatusNotFound, JSONResponse{Code: 404, Error: fmt.Sprintf("找不到本地已登记项目: %q，请先在服务器上手动部署一次", projectName)})
		return
	}

	// 2. 读取项目专属配置进行鉴权
	projCfg, err := config.ReadProjectConfig(targetProject.Root)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, JSONResponse{Code: 500, Error: "读取项目配置失败: " + err.Error()})
		return
	}

	inboundSecret := ""
	if projCfg != nil {
		inboundSecret = projCfg.InboundSecret()
	}

	if authErr := Authenticate(r, s.cfg.Token, inboundSecret); authErr != nil {
		writeJSON(w, http.StatusUnauthorized, JSONResponse{Code: 401, Error: authErr.Error()})
		return
	}

	// 3. 检查是否有该项目的正在进行的任务
	if activeTask, busy := s.taskMgr.GetActiveTask(projectName); busy {
		writeJSON(w, http.StatusConflict, map[string]any{
			"code":           409,
			"error":          "项目当前已有正在执行的部署任务",
			"active_task_id": activeTask.ID,
			"log_url":        s.taskLogURL(activeTask.ID),
		})
		return
	}

	// 4. 解析参数 (Query / JSON Body)
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	force := r.URL.Query().Get("force") == "true"

	if r.Header.Get("Content-Type") == "application/json" && r.Body != nil {
		var reqBody WebhookRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			if target == "" && reqBody.Target != "" {
				target = strings.TrimSpace(reqBody.Target)
			}
			if reqBody.Force {
				force = true
			}
		}
	}
	if target == "" {
		target = "latest"
	}

	// 5. 创建任务
	task, err := s.taskMgr.CreateTask(projectName, targetProject.Root, target, force, "")
	if err != nil {
		writeJSON(w, http.StatusConflict, JSONResponse{Code: 409, Error: err.Error()})
		return
	}

	// 6. 异步拉起子进程执行任务 (使用独立上下文避免请求结束后被取消)
	go func() {
		taskCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		s.worker.RunDeploy(taskCtx, task)
	}()

	// 7. 立即返回 202 Accepted
	writeJSON(w, http.StatusAccepted, map[string]any{
		"code":       0,
		"message":    "部署任务已创建并开始执行",
		"task_id":    task.ID,
		"project":    task.Project,
		"target":     task.Target,
		"force":      task.Force,
		"status":     task.Status,
		"created_at": task.StartedAt.Format(time.RFC3339),
		"log_url":    s.taskLogURL(task.ID),
	})
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.PathValue("id"))
	if taskID == "" {
		writeJSON(w, http.StatusBadRequest, JSONResponse{Code: 400, Error: "缺少任务 ID"})
		return
	}

	task, ok := s.taskMgr.Get(taskID)
	if !ok {
		writeJSON(w, http.StatusNotFound, JSONResponse{Code: 404, Error: "未找到该任务"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"task": task,
	})
}

func (s *Server) handleGetTaskLogs(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.PathValue("id"))
	if taskID == "" {
		http.Error(w, "缺少任务 ID", http.StatusBadRequest)
		return
	}

	var logPath string
	if task, ok := s.taskMgr.Get(taskID); ok && task.LogFile != "" {
		logPath = task.LogFile
	} else {
		// 尝试在 ~/.upy/logs/*/<taskID>.log 中定位
		logPath = findLogPathByID(taskID)
	}

	if logPath == "" {
		http.Error(w, "未找到对应的任务日志文件", http.StatusNotFound)
		return
	}

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "日志文件尚未生成或已被清理", http.StatusNotFound)
			return
		}
		http.Error(w, "读取日志文件失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if authErr := Authenticate(r, s.cfg.Token, ""); authErr != nil {
		writeJSON(w, http.StatusUnauthorized, JSONResponse{Code: 401, Error: authErr.Error()})
		return
	}

	localProjects, err := config.ListLocalProjects()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, JSONResponse{Code: 500, Error: err.Error()})
		return
	}

	type projectView struct {
		Name        string `json:"name"`
		Root        string `json:"root"`
		IsDeploying bool   `json:"is_deploying"`
	}

	views := make([]projectView, 0, len(localProjects))
	for _, lp := range localProjects {
		_, busy := s.taskMgr.GetActiveTask(lp.Name)
		views = append(views, projectView{
			Name:        lp.Name,
			Root:        lp.Root,
			IsDeploying: busy,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":     0,
		"projects": views,
	})
}

func (s *Server) taskLogURL(taskID string) string {
	base := strings.TrimRight(s.cfg.PublicURL, "/")
	if base == "" {
		host := s.cfg.EffectiveListen()
		if strings.HasPrefix(host, ":") {
			host = "127.0.0.1" + host
		}
		base = "http://" + host
	}
	return base + "/api/v1/tasks/" + taskID + "/logs"
}

func findLogPathByID(taskID string) string {
	if !validLogID(taskID) {
		return ""
	}
	logsRoot := filepath.Join(config.UserDataDir(), "logs")
	entries, err := os.ReadDir(logsRoot)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		target := filepath.Join(logsRoot, e.Name(), taskID+".log")
		if _, err := os.Stat(target); err == nil {
			return target
		}
	}
	return ""
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		token := GetSessionToken(r)
		if !s.sessions.Validate(token) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func (s *Server) handleRootGet(w http.ResponseWriter, r *http.Request) {
	token := GetSessionToken(r)
	if s.sessions.Validate(token) {
		http.Redirect(w, r, "/overview", http.StatusFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	ip := ClientIP(r)
	if s.loginLimiter != nil && s.loginLimiter.IsBlocked(ip) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, RenderLoginPage("尝试次数过多，已禁止访问", true))
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, RenderLoginPage(""))
}

func (s *Server) handleRootPost(w http.ResponseWriter, r *http.Request) {
	ip := ClientIP(r)
	if s.loginLimiter != nil && s.loginLimiter.IsBlocked(ip) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, RenderLoginPage("尝试次数过多，已禁止访问", true))
		return
	}

	_ = r.ParseForm()
	code := strings.TrimSpace(r.FormValue("code"))

	if s.totpMgr == nil || !s.totpMgr.Validate(code) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if s.loginLimiter != nil && !s.loginLimiter.RecordFailure(ip) {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, RenderLoginPage("尝试次数过多，已禁止访问", true))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, RenderLoginPage("动态验证码错误或已失效，请重新输入"))
		return
	}

	if s.loginLimiter != nil {
		s.loginLimiter.RecordSuccess(ip)
	}

	token := s.sessions.CreateSession()
	SetSessionCookie(w, token)
	http.Redirect(w, r, "/overview", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := GetSessionToken(r)
	s.sessions.Revoke(token)
	ClearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func collectLogItems(taskMgr *TaskManager) []LogItemView {
	seen := make(map[string]bool)
	var items []LogItemView

	for _, t := range taskMgr.List() {
		seen[t.ID] = true
		items = append(items, LogItemView{
			ID:        t.ID,
			Project:   t.Project,
			Timestamp: t.StartedAt.Format("2006-01-02 15:04:05"),
			Status:    string(t.Status),
			ExitCode:  t.ExitCode,
			Duration:  t.Duration,
			Target:    t.Target,
		})
	}

	logsRoot := filepath.Join(config.UserDataDir(), "logs")
	entries, err := os.ReadDir(logsRoot)
	if err == nil {
		for _, pe := range entries {
			if !pe.IsDir() {
				continue
			}
			projName := pe.Name()
			projLogDir := filepath.Join(logsRoot, projName)
			logEntries, lerr := os.ReadDir(projLogDir)
			if lerr != nil {
				continue
			}
			for _, le := range logEntries {
				if le.IsDir() || !strings.HasSuffix(le.Name(), ".log") {
					continue
				}
				taskID := strings.TrimSuffix(le.Name(), ".log")
				if seen[taskID] {
					continue
				}
				seen[taskID] = true

				info, _ := le.Info()
				modTime := time.Now()
				if info != nil {
					modTime = info.ModTime()
				}

				logPath := filepath.Join(projLogDir, le.Name())
				status, exitCode := archivedLogStatus(logPath)

				items = append(items, LogItemView{
					ID:        taskID,
					Project:   projName,
					Timestamp: modTime.Format("2006-01-02 15:04:05"),
					Status:    status,
					ExitCode:  exitCode,
					Duration:  "-",
				})
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Timestamp == items[j].Timestamp {
			return items[i].ID > items[j].ID
		}
		return items[i].Timestamp > items[j].Timestamp
	})
	return items
}

func readTail(path string, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return "", err
	}

	size := stat.Size()
	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
	}
	buf := make([]byte, size-offset)
	_, err = f.ReadAt(buf, offset)
	return string(buf), err
}

// 历史文件缺少明确结束标记时结果未知，不能将中断的部署推断为成功。
var archivedExitPattern = regexp.MustCompile(`部署任务结束 \(退出码: (-?[0-9]+)\)`)

func archivedLogStatus(path string) (string, int) {
	tail, err := readTail(path, 4096)
	if err != nil {
		return "unknown", 0
	}
	matches := archivedExitPattern.FindAllStringSubmatch(ansiRegex.ReplaceAllString(tail, ""), -1)
	if len(matches) == 0 {
		return "unknown", 0
	}
	code, err := strconv.Atoi(matches[len(matches)-1][1])
	if err != nil {
		return "unknown", 0
	}
	if code != 0 {
		return "failed", code
	}
	return "success", 0
}
