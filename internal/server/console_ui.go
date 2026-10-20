package server

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/url"
	"strings"
)

// 所有资源编译进二进制，不依赖外部字体、CDN 或运行目录。
//
//go:embed web/* web/icons/* web/pwa/*
var consoleAssets embed.FS

var consoleTemplate = template.Must(template.New("console").Funcs(template.FuncMap{
	"icon":           consoleIcon,
	"projectURL":     func(name string) string { return "/projects/" + url.PathEscape(name) },
	"logURL":         func(id string) string { return "/logs/" + url.PathEscape(id) },
	"projectLogsURL": func(name string) string { return "/logs?project=" + url.QueryEscape(name) },
	"statusClass":    statusClass, "statusLabel": statusLabel, "statusIcon": statusIcon,
	"fallback": func(s string) string {
		if s == "" {
			return "—"
		}
		return s
	},
	"modeLabel": modeLabel, "styles": consoleStyles, "script": consoleScript,
}).ParseFS(consoleAssets, "web/console.html"))

func consoleIcon(name string) template.HTML {
	// 图标名只来自代码/模板；文件内容是随包提供的 Lucide SVG。
	data, err := consoleAssets.ReadFile("web/icons/" + name + ".svg")
	if err != nil {
		return ""
	}
	svg := strings.Replace(string(data), "<svg", `<svg class="icon" aria-hidden="true"`, 1)
	svg = strings.Replace(svg, `stroke-width="2"`, `stroke-width="1.5"`, 1)
	return template.HTML(svg)
}

type LogItemView struct {
	ID, Project, Timestamp, Status, Duration, Size, Target string
	ExitCode                                               int
}

type ProjectView struct {
	Name, Root, Version, PreviousVersion, Type, Repository, Unit, DeployedAt, Status, Warning string
	Latest                                                                                    *LogItemView
	Logs                                                                                      []LogItemView
}

type ConsolePage struct {
	Title, Screen, Active, Back, Warning          string
	Host, Platform, Uptime, Listen, ServerVersion string
	Projects                                      []ProjectView
	Project                                       *ProjectView
	Logs                                          []LogItemView
	SuccessCount, FailedCount                     int
	Query, ProjectFilter, StatusFilter            string
	Page, Pages, Total                            int
	PrevURL, NextURL                              string
	Detail                                        *LogDetailView
	Auth                                          bool
	Error                                         string
}

type LogDetailView struct {
	ID, Project, Status, Duration, Raw string
	ExitCode                           int
	// 仅接受 RenderAnsiToHTML 转义后的文本，不能放入任意用户 HTML。
	HTML      template.HTML
	Truncated bool
	Live      bool
}

func renderConsole(p ConsolePage) string {
	var b bytes.Buffer
	if err := consoleTemplate.ExecuteTemplate(&b, "console.html", p); err != nil {
		return "<!doctype html><html lang=\"zh-CN\"><meta charset=\"utf-8\"><title>upy</title><p>页面渲染失败</p></html>"
	}
	return b.String()
}

func statusClass(s string) string {
	switch s {
	case "success":
		return "success"
	case "failed", "timeout":
		return "failed"
	case "running", "queued", "stream":
		return "running"
	default:
		return "unknown"
	}
}
func statusLabel(s string) string {
	switch s {
	case "success":
		return "部署成功"
	case "failed":
		return "部署失败"
	case "timeout":
		return "任务超时"
	case "running":
		return "执行中"
	case "queued":
		return "排队中"
	case "archived":
		return "已归档"
	case "stream":
		return "持续记录"
	default:
		return "结果未知"
	}
}
func statusIcon(s string) string {
	switch statusClass(s) {
	case "success":
		return "circle-check"
	case "failed":
		return "circle-x"
	case "running":
		return "clock-3"
	default:
		return "circle"
	}
}
func modeLabel(s string) string {
	switch s {
	case "static":
		return "Static"
	case "system", "service":
		return "Systemd"
	case "docker":
		return "Docker"
	default:
		return "类型未知"
	}
}

func RenderLoginPage(errMsg string) string {
	return renderConsole(ConsolePage{Title: "安全登录", Screen: "login", Error: errMsg})
}
func RenderLogsPage(items []LogItemView, activeProjects []string) string {
	p := ConsolePage{Title: "部署执行日志", Screen: "logs", Active: "logs", Auth: true, Logs: items, Total: len(items), Page: 1, Pages: 1}
	for _, name := range activeProjects {
		p.Projects = append(p.Projects, ProjectView{Name: name})
	}
	return renderConsole(p)
}
func RenderLogDetailPage(taskID, project, status string, exitCode int, duration, logHTML, rawText string) string {
	return renderConsole(ConsolePage{Title: "部署日志", Screen: "detail", Active: "logs", Back: "/logs", Auth: true, Detail: &LogDetailView{ID: taskID, Project: project, Status: status, ExitCode: exitCode, Duration: duration, HTML: template.HTML(logHTML), Raw: rawText, Live: status == "running"}})
}

// CSS 内联避免登录页加载资源时额外暴露文件端点。
func consoleStyles() template.CSS {
	tokens, _ := consoleAssets.ReadFile("web/tokens.css")
	css, _ := consoleAssets.ReadFile("web/styles.css")
	return template.CSS(string(tokens) + "\n" + string(css))
}
func consoleScript() template.JS {
	js, _ := consoleAssets.ReadFile("web/app.js")
	return template.JS(js)
}

// template 中格式化退出码保持数值，不把未知历史日志推断为成功。
func (d LogDetailView) ExitLabel() string {
	if d.Status == "success" || d.Status == "failed" || d.Status == "timeout" {
		return fmt.Sprintf("EXIT CODE %d", d.ExitCode)
	}
	return ""
}
