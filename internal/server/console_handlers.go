package server

import (
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/state"
	"github.com/AaronConlon/upy/internal/version"
)

func (s *Server) consolePage(screen, title, active string) ConsolePage {
	return ConsolePage{Title: title, Screen: screen, Active: active, Auth: true}
}
func writeConsole(w http.ResponseWriter, p ConsolePage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, renderConsole(p))
}

// 只从注册表解析项目目录，不把 URL 中的项目名称当作文件路径。
func (s *Server) collectProjects(logs []LogItemView) ([]ProjectView, error) {
	locals, err := config.ListLocalProjects()
	if err != nil {
		return nil, err
	}
	projects := make([]ProjectView, 0, len(locals))
	for _, lp := range locals {
		p := ProjectView{Name: lp.Name, Root: lp.Root, Status: "unknown"}
		st, err := state.Read(lp.Root)
		if err != nil {
			p.Warning = "部署状态无法读取，部分信息暂不可用。"
		} else {
			p.Version, p.PreviousVersion, p.Type, p.Unit, p.DeployedAt = st.CurrentVersion, st.PreviousVersion, st.Type, st.UnitName, st.DeployedAt
			if p.Version != "" {
				p.Status = "success"
			}
			if st.Status == "failed" || st.Status == "degraded" {
				p.Status = "failed"
			}
		}
		cfg, err := config.ReadProjectConfig(lp.Root)
		if err != nil {
			if p.Warning != "" {
				p.Warning += " "
			}
			p.Warning += "项目配置无法读取，仓库等信息暂不可用。"
		} else {
			if cfg.Release != nil {
				p.Repository = cfg.Release.Repository
			}
			if p.Unit == "" && cfg.Service != nil {
				p.Unit = cfg.Service.Unit
			}
		}
		for _, item := range logs {
			if item.Project == p.Name {
				p.Logs = append(p.Logs, item)
			}
		}
		if len(p.Logs) > 0 {
			latest := p.Logs[0]
			p.Latest = &latest
			// CLI 直接部署可能晚于最后一份 server 任务日志；此时以较新的状态文件为准。
			deployedAt, stateErr := time.Parse(time.RFC3339, p.DeployedAt)
			loggedAt, logErr := time.ParseInLocation("2006-01-02 15:04:05", latest.Timestamp, time.Local)
			if latest.Status == "running" || latest.Status == "queued" || stateErr != nil || logErr != nil || !deployedAt.After(loggedAt) {
				p.Status = latest.Status
			}
		}
		projects = append(projects, p)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}
func (s *Server) handleOverviewPage(w http.ResponseWriter, r *http.Request) {
	p := s.consolePage("overview", "服务总览", "overview")
	logs := collectLogItems(s.taskMgr)
	projects, err := s.collectProjects(logs)
	if err != nil {
		p.Warning = "项目注册表无法读取，请检查服务器配置。"
	}
	p.Projects = projects
	for _, project := range projects {
		if project.Status == "success" {
			p.SuccessCount++
		}
		if statusClass(project.Status) == "failed" {
			p.FailedCount++
		}
	}
	if len(logs) > 5 {
		logs = logs[:5]
	}
	p.Logs = logs
	p.Host, _ = os.Hostname()
	if p.Host == "" {
		p.Host = "upy server"
	}
	p.Platform = strings.ToUpper(runtime.GOOS + " / " + runtime.GOARCH)
	p.Uptime = time.Since(s.startedAt).Round(time.Second).String()
	p.Listen = s.Addr()
	p.ServerVersion = version.String()
	writeConsole(w, p)
}
func (s *Server) handleProjectsPage(w http.ResponseWriter, r *http.Request) {
	p := s.consolePage("projects", "项目", "projects")
	p.Query = strings.TrimSpace(r.URL.Query().Get("q"))
	projects, err := s.collectProjects(collectLogItems(s.taskMgr))
	if err != nil {
		p.Warning = "项目注册表无法读取，请检查服务器配置。"
	}
	for _, project := range projects {
		if matchesQuery(p.Query, project.Name, project.Root) {
			p.Projects = append(p.Projects, project)
		}
	}
	writeConsole(w, p)
}
func (s *Server) handleProjectPage(w http.ResponseWriter, r *http.Request) {
	projects, err := s.collectProjects(collectLogItems(s.taskMgr))
	if err != nil {
		http.Error(w, "项目注册表无法读取", http.StatusInternalServerError)
		return
	}
	for _, project := range projects {
		if project.Name == r.PathValue("name") {
			p := s.consolePage("project", project.Name, "projects")
			p.Back = "/projects"
			if len(project.Logs) > 5 {
				project.Logs = project.Logs[:5]
			}
			p.Project = &project
			writeConsole(w, p)
			return
		}
	}
	http.NotFound(w, r)
}
func matchesQuery(query string, values ...string) bool {
	query = strings.ToLower(query)
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}
func (s *Server) handleConsoleLogsPage(w http.ResponseWriter, r *http.Request) {
	p := s.consolePage("logs", "部署执行日志", "logs")
	q := r.URL.Query()
	p.Query = strings.TrimSpace(q.Get("q"))
	p.ProjectFilter = q.Get("project")
	p.StatusFilter = q.Get("status")
	logs := collectLogItems(s.taskMgr)
	projects, err := s.collectProjects(logs)
	if err != nil {
		p.Warning = "项目注册表无法读取；仍可查看已归档日志。"
	}
	// 已从注册表移除的项目仍能在筛选器中选择。
	names := map[string]bool{}
	for _, project := range projects {
		names[project.Name] = true
	}
	for _, item := range logs {
		names[item.Project] = true
	}
	if p.ProjectFilter != "" {
		names[p.ProjectFilter] = true
	}
	for name := range names {
		p.Projects = append(p.Projects, ProjectView{Name: name})
	}
	sort.Slice(p.Projects, func(i, j int) bool { return p.Projects[i].Name < p.Projects[j].Name })
	var filtered []LogItemView
	for _, item := range logs {
		if p.ProjectFilter != "" && item.Project != p.ProjectFilter {
			continue
		}
		if p.StatusFilter != "" && statusClass(item.Status) != p.StatusFilter {
			continue
		}
		if !matchesQuery(p.Query, item.ID, item.Project, item.Target) {
			continue
		}
		filtered = append(filtered, item)
	}
	p.Total = len(filtered)
	p.Pages = (p.Total + 19) / 20
	if p.Pages < 1 {
		p.Pages = 1
	}
	p.Page, _ = strconv.Atoi(q.Get("page"))
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Page > p.Pages {
		p.Page = p.Pages
	}
	start := (p.Page - 1) * 20
	end := start + 20
	if end > len(filtered) {
		end = len(filtered)
	}
	p.Logs = filtered[start:end]
	pageURL := func(page int) string {
		v := url.Values{}
		if p.Query != "" {
			v.Set("q", p.Query)
		}
		if p.ProjectFilter != "" {
			v.Set("project", p.ProjectFilter)
		}
		if p.StatusFilter != "" {
			v.Set("status", p.StatusFilter)
		}
		v.Set("page", strconv.Itoa(page))
		return "/logs?" + v.Encode()
	}
	if p.Page > 1 {
		p.PrevURL = pageURL(p.Page - 1)
	}
	if p.Page < p.Pages {
		p.NextURL = pageURL(p.Page + 1)
	}
	writeConsole(w, p)
}

const maxConsoleLogBytes int64 = 256 * 1024

// 大日志页面只读取末尾，完整文件通过同一 Session 保护的下载入口传输。
func readConsoleLog(path string) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", false, err
	}
	offset := st.Size() - maxConsoleLogBytes
	truncated := offset > 0
	if offset < 0 {
		offset = 0
	}
	if _, err = f.Seek(offset, io.SeekStart); err != nil {
		return "", false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxConsoleLogBytes))
	if truncated {
		if i := strings.IndexByte(string(data), '\n'); i >= 0 {
			data = data[i+1:]
		}
	}
	return strings.ToValidUTF8(string(data), "�"), truncated, err
}
func (s *Server) consoleLogDetail(id string) (LogDetailView, string, error) {
	d := LogDetailView{ID: id, Status: "unknown"}
	var path string
	switch id {
	case "server":
		d.Project = "server"
		d.Status = "stream"
		d.Live = true
		path = ServerLogPath()
	case "upy":
		d.Project = "upy"
		d.Status = "stream"
		d.Live = true
		path = filepath.Join(config.UserDataDir(), "logs", "upy.log")
	default:
		if task, ok := s.taskMgr.Get(id); ok {
			d.Project = task.Project
			d.Status = string(task.Status)
			d.ExitCode = task.ExitCode
			d.Duration = task.Duration
			path = task.LogFile
			d.Live = task.Status == StatusRunning || task.Status == StatusQueued
		} else {
			path = findLogPathByID(id)
			if path != "" {
				d.Project = filepath.Base(filepath.Dir(path))
				d.Status, d.ExitCode = archivedLogStatus(path)
			}
		}
	}
	if path == "" {
		return d, "", os.ErrNotExist
	}
	raw, truncated, err := readConsoleLog(path)
	if os.IsNotExist(err) && (d.Live || id == "upy") {
		raw = "日志文件尚未生成。\n"
		err = nil
	}
	if err != nil {
		return d, path, err
	}
	d.Raw = raw
	d.Truncated = truncated
	d.HTML = template.HTML(RenderAnsiToHTML(raw))
	return d, path, nil
}
func (s *Server) handleConsoleLogDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validLogID(id) {
		http.NotFound(w, r)
		return
	}
	d, path, err := s.consoleLogDetail(id)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
		} else {
			http.Error(w, "读取日志失败", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("download") == "1" {
		f, err := os.Open(path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			http.Error(w, "读取日志失败", 500)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": id + ".log"}))
		http.ServeContent(w, r, id+".log", st.ModTime(), f)
		return
	}
	if r.URL.Query().Get("format") == "json" {
		writeJSON(w, 200, map[string]any{"html": string(d.HTML), "status_class": statusClass(d.Status), "status_label": statusLabel(d.Status), "status_html": string(consoleIcon(statusIcon(d.Status))) + template.HTMLEscapeString(statusLabel(d.Status)), "duration": d.Duration, "exit_label": d.ExitLabel(), "live": d.Live, "truncated": d.Truncated})
		return
	}
	p := s.consolePage("detail", "部署日志", "logs")
	p.Back = "/logs"
	p.Detail = &d
	writeConsole(w, p)
}
func validLogID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\\x00")
}
