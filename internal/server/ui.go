package server

import (
	"encoding/base64"
	"fmt"
	"html"
	"regexp"
	"strings"
)

var ansiRegex = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// RenderAnsiToHTML 将终端 ANSI 颜色转义字符解析为安全的带颜色的 HTML 标签
func RenderAnsiToHTML(raw string) string {
	escaped := html.EscapeString(raw)

	// 状态机替换
	openSpans := 0
	out := ansiRegex.ReplaceAllStringFunc(escaped, func(m string) string {
		sub := ansiRegex.FindStringSubmatch(m)
		if len(sub) < 2 || sub[1] == "0" || sub[1] == "" {
			res := strings.Repeat("</span>", openSpans)
			openSpans = 0
			return res
		}

		codes := strings.Split(sub[1], ";")
		var styles []string
		for _, c := range codes {
			switch c {
			case "1":
				styles = append(styles, "font-weight:bold")
			case "2":
				styles = append(styles, "opacity:0.65")
			case "31": // Red
				styles = append(styles, "color:#f85149")
			case "32": // Green
				styles = append(styles, "color:#3fb950")
			case "33": // Yellow
				styles = append(styles, "color:#d29922")
			case "34": // Blue
				styles = append(styles, "color:#58a6ff")
			case "35": // Magenta
				styles = append(styles, "color:#bc8cff")
			case "36", "96": // Cyan / Bright Cyan
				styles = append(styles, "color:#38bdf8")
			}
		}

		if len(styles) > 0 {
			openSpans++
			return fmt.Sprintf(`<span style="%s">`, strings.Join(styles, ";"))
		}
		return ""
	})

	if openSpans > 0 {
		out += strings.Repeat("</span>", openSpans)
	}
	return out
}

const baseCSS = `
:root {
	--bg: #0d1117;
	--card-bg: #161b22;
	--border: #30363d;
	--text: #c9d1d9;
	--text-dim: #8b949e;
	--accent: #38bdf8;
	--green: #3fb950;
	--red: #f85149;
	--yellow: #d29922;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
	background: var(--bg);
	color: var(--text);
	font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
	min-height: 100vh;
	display: flex;
	flex-direction: column;
}
a { color: var(--accent); text-decoration: none; }
a:hover { text-decoration: underline; }
.header {
	border-bottom: 1px solid var(--border);
	padding: 16px 24px;
	display: flex;
	justify-content: space-between;
	align-items: center;
	background: rgba(22, 27, 34, 0.8);
	backdrop-filter: blur(8px);
}
.brand {
	display: flex;
	align-items: center;
	gap: 10px;
	font-size: 18px;
	font-weight: 700;
	color: #fff;
	text-decoration: none;
}
.brand .tag {
	font-size: 11px;
	background: #1f6feb26;
	color: var(--accent);
	border: 1px solid #38bdf840;
	padding: 2px 8px;
	border-radius: 12px;
	font-weight: normal;
}
.container {
	max-width: 900px;
	width: 100%;
	margin: 40px auto;
	padding: 0 20px;
}
.card {
	background: var(--card-bg);
	border: 1px solid var(--border);
	border-radius: 12px;
	padding: 32px;
	box-shadow: 0 10px 25px rgba(0,0,0,0.5);
}
.btn {
	background: #238636;
	color: #fff;
	border: none;
	padding: 10px 20px;
	border-radius: 6px;
	font-size: 15px;
	font-weight: 600;
	cursor: pointer;
	transition: background 0.2s;
	display: inline-flex;
	align-items: center;
	justify-content: center;
	gap: 6px;
}
.btn:hover { background: #2ea043; text-decoration: none; }
.btn-outline {
	background: transparent;
	border: 1px solid var(--border);
	color: var(--text);
}
.btn-outline:hover { background: #21262d; }
.badge {
	font-size: 12px;
	padding: 3px 8px;
	border-radius: 6px;
	font-weight: 600;
	display: inline-block;
}
.badge-success { background: #23863626; color: var(--green); border: 1px solid #3fb95040; }
.badge-failed { background: #da363326; color: var(--red); border: 1px solid #f8514940; }
.badge-running { background: #1f6feb26; color: var(--accent); border: 1px solid #38bdf840; }
`

// RenderAuthPage 渲染 TOTP 2FA 绑定页面 (/auth)
func RenderAuthPage(qrPNG []byte, secret string, views, maxViews int) string {
	qrBase64 := base64.StdEncoding.EncodeToString(qrPNG)
	remaining := maxViews - views
	if remaining < 0 {
		remaining = 0
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>两步验证 (2FA) 绑定 — upy</title>
	<style>
		%s
		.qr-wrapper {
			text-align: center;
			margin: 24px 0;
		}
		.qr-box {
			background: #fff;
			padding: 16px;
			display: inline-block;
			border-radius: 12px;
			box-shadow: 0 4px 12px rgba(0,0,0,0.4);
		}
		.qr-box img { display: block; width: 200px; height: 200px; }
		.secret-key {
			background: #0d1117;
			border: 1px solid var(--border);
			padding: 12px 16px;
			border-radius: 8px;
			font-family: monospace;
			font-size: 16px;
			color: var(--accent);
			text-align: center;
			letter-spacing: 2px;
			margin: 16px 0;
			user-select: all;
		}
		.quota-box {
			background: #d299221a;
			border: 1px solid #d2992240;
			color: #e3b341;
			padding: 10px 14px;
			border-radius: 8px;
			font-size: 13px;
			margin-bottom: 20px;
			display: flex;
			align-items: center;
			justify-content: space-between;
		}
	</style>
</head>
<body>
	<header class="header">
		<a href="/" class="brand">🚀 upy <span class="tag">Web Server</span></a>
	</header>
	<div class="container" style="max-width: 520px;">
		<div class="card">
			<h2 style="margin-bottom: 8px;">🔐 两步验证 (2FA) 绑定</h2>
			<p style="color: var(--text-dim); font-size: 14px;">请使用 Google Authenticator、Microsoft Authenticator 或 1Password 等扫码绑定。</p>

			<div class="qr-wrapper">
				<div class="qr-box">
					<img src="data:image/png;base64,%s" alt="2FA QR Code">
				</div>
			</div>

			<p style="font-size: 13px; color: var(--text-dim); text-align: center;">若无法扫码，可手动添加密钥：</p>
			<div class="secret-key">%s</div>

			<div class="quota-box">
				<span>⚠️ 安全保护配额</span>
				<span>剩余扫码机会: <strong>%d / %d</strong> 次</span>
			</div>

			<div style="text-align: center; margin-top: 24px;">
				<a href="/" class="btn" style="width: 100%%;">已完成绑定，前往登录验证 -></a>
			</div>
		</div>
	</div>
</body>
</html>`, baseCSS, qrBase64, secret, remaining, maxViews)
}

// RenderLoginPage 渲染 TOTP 动态验证码输入页面 (/)
func RenderLoginPage(errMsg string) string {
	errHTML := ""
	if errMsg != "" {
		errHTML = fmt.Sprintf(`<div style="background:#da363326; border:1px solid #f8514940; color:var(--red); padding:10px 14px; border-radius:8px; font-size:14px; margin-bottom:18px;">❌ %s</div>`, html.EscapeString(errMsg))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>安全登录 — upy</title>
	<style>
		%s
		.code-input {
			width: 100%%;
			background: #0d1117;
			border: 2px solid var(--border);
			color: #fff;
			font-family: monospace;
			font-size: 32px;
			letter-spacing: 12px;
			text-align: center;
			padding: 12px;
			border-radius: 8px;
			outline: none;
			transition: border-color 0.2s;
			margin: 20px 0;
		}
		.code-input:focus {
			border-color: var(--accent);
		}
	</style>
</head>
<body>
	<header class="header">
		<a href="/" class="brand">🚀 upy <span class="tag">Console</span></a>
	</header>
	<div class="container" style="max-width: 440px; margin-top: 60px;">
		<div class="card">
			<h2 style="margin-bottom: 8px; text-align: center;">验证身份</h2>
			<p style="color: var(--text-dim); font-size: 14px; text-align: center;">请输入认证器中的 6 位动态验证码</p>

			<form method="POST" action="/" style="margin-top: 20px;">
				%s
				<input type="text" name="code" class="code-input" maxlength="6" pattern="[0-9]{6}" inputmode="numeric" placeholder="000000" autocomplete="one-time-code" autofocus required>
				<button type="submit" class="btn" style="width: 100%%; padding: 12px;">验证并进入系统</button>
			</form>
			<div style="margin-top: 20px; text-align: center; font-size: 12px; color: var(--text-dim);">
				Session 有效期为 30 分钟，过期需重新验证
			</div>
		</div>
	</div>
</body>
</html>`, baseCSS, errHTML)
}

type LogItemView struct {
	ID        string
	Project   string
	Timestamp string
	Status    string
	ExitCode  int
	Duration  string
	Size      string
}

// RenderLogsPage 渲染日志列表页面 (/logs)
func RenderLogsPage(items []LogItemView, activeProjects []string) string {
	var rows strings.Builder
	if len(items) == 0 {
		rows.WriteString(`<tr><td colspan="6" style="text-align:center; padding: 40px; color: var(--text-dim);">暂无任何部署日志记录。当 CI/CD 触发 Webhook 或手动部署后将自动展示于此。</td></tr>`)
	} else {
		for _, item := range items {
			badgeClass := "badge-success"
			statusText := "成功"
			if item.Status == "failed" || item.ExitCode != 0 {
				badgeClass = "badge-failed"
				statusText = "失败"
			} else if item.Status == "running" {
				badgeClass = "badge-running"
				statusText = "运行中"
			}

			rows.WriteString(fmt.Sprintf(`
			<tr>
				<td><span style="font-weight: 600; color: #fff; background: #21262d; padding: 3px 8px; border-radius: 4px; font-size: 13px;">📦 %s</span></td>
				<td><a href="/logs/%s" style="font-family: monospace;">%s</a></td>
				<td><span class="badge %s">%s</span></td>
				<td style="color: var(--text-dim); font-size: 13px;">%s</td>
				<td style="color: var(--text-dim); font-size: 13px;">%s</td>
				<td><a href="/logs/%s" class="btn btn-outline" style="padding: 4px 10px; font-size: 12px;">查看日志 &rarr;</a></td>
			</tr>`,
				html.EscapeString(item.Project),
				html.EscapeString(item.ID),
				html.EscapeString(item.ID),
				badgeClass, statusText,
				html.EscapeString(item.Timestamp),
				html.EscapeString(item.Duration),
				html.EscapeString(item.ID),
			))
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>部署日志 — upy</title>
	<style>
		%s
		table {
			width: 100%%;
			border-collapse: collapse;
			margin-top: 16px;
		}
		th, td {
			padding: 12px 14px;
			text-align: left;
			border-bottom: 1px solid var(--border);
		}
		th {
			font-size: 12px;
			color: var(--text-dim);
			text-transform: uppercase;
			font-weight: 600;
			background: #161b22;
		}
		tr:hover td {
			background: #1c2128;
		}
	</style>
</head>
<body>
	<header class="header">
		<a href="/logs" class="brand">🚀 upy <span class="tag">Console</span></a>
		<div style="display: flex; gap: 12px; align-items: center;">
			<form method="POST" action="/logout">
				<button type="submit" class="btn btn-outline" style="padding: 6px 12px; font-size: 13px;">退出登录</button>
			</form>
		</div>
	</header>
	<div class="container" style="max-width: 1000px;">
		<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
			<div>
				<h2>📋 部署执行日志</h2>
				<p style="color: var(--text-dim); font-size: 14px; margin-top: 4px;">按项目清晰归档与回溯最近的部署记录</p>
			</div>
			<a href="/logs" class="btn btn-outline" style="padding: 8px 14px; font-size: 13px;">🔄 刷新列表</a>
		</div>

		<div class="card" style="padding: 0; overflow: hidden;">
			<table>
				<thead>
					<tr>
						<th>所属项目</th>
						<th>任务 ID</th>
						<th>执行状态</th>
						<th>开始时间</th>
						<th>耗时</th>
						<th>操作</th>
					</tr>
				</thead>
				<tbody>
					%s
				</tbody>
			</table>
		</div>
	</div>
</body>
</html>`, baseCSS, rows.String())
}

// RenderLogDetailPage 渲染单次任务日志详情页面 (/logs/{id})
func RenderLogDetailPage(taskID, project, status string, exitCode int, duration, logHTML, rawText string) string {
	badgeClass := "badge-success"
	statusText := "成功"
	if status == "failed" || exitCode != 0 {
		badgeClass = "badge-failed"
		statusText = fmt.Sprintf("失败 (退出码 %d)", exitCode)
	} else if status == "running" {
		badgeClass = "badge-running"
		statusText = "执行中..."
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>%s 日志详情 — upy</title>
	<style>
		%s
		.terminal {
			background: #090d13;
			border: 1px solid var(--border);
			border-radius: 8px;
			overflow: hidden;
			margin-top: 20px;
			box-shadow: 0 8px 24px rgba(0,0,0,0.6);
		}
		.terminal-bar {
			background: #161b22;
			border-bottom: 1px solid var(--border);
			padding: 10px 16px;
			display: flex;
			align-items: center;
			justify-content: space-between;
		}
		.traffic-lights {
			display: flex;
			gap: 6px;
		}
		.light { width: 10px; height: 10px; border-radius: 50%%; }
		.light-red { background: #f85149; }
		.light-yellow { background: #d29922; }
		.light-green { background: #3fb950; }
		.terminal-body {
			padding: 16px;
			font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
			font-size: 13px;
			line-height: 1.6;
			color: #c9d1d9;
			white-space: pre-wrap;
			word-break: break-all;
			max-height: 75vh;
			overflow-y: auto;
		}
	</style>
</head>
<body>
	<header class="header">
		<a href="/logs" class="brand">🚀 upy <span class="tag">Console</span></a>
		<div style="display: flex; gap: 12px; align-items: center;">
			<a href="/logs" class="btn btn-outline" style="padding: 6px 12px; font-size: 13px;">&larr; 返回列表</a>
		</div>
	</header>
	<div class="container" style="max-width: 1000px;">
		<div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 12px;">
			<div>
				<div style="display: flex; align-items: center; gap: 10px; margin-bottom: 6px;">
					<span style="font-weight: 700; color: #fff; font-size: 18px;">项目: %s</span>
					<span class="badge %s">%s</span>
				</div>
				<div style="font-family: monospace; font-size: 13px; color: var(--text-dim);">
					任务 ID: %s &bull; 耗时: %s
				</div>
			</div>
			<div style="display: flex; gap: 8px;">
				<button class="btn btn-outline" onclick="location.reload()" style="padding: 6px 12px; font-size: 13px;">🔄 刷新日志</button>
				<button class="btn btn-outline" onclick="copyLog()" style="padding: 6px 12px; font-size: 13px;">📋 复制日志</button>
			</div>
		</div>

		<div class="terminal">
			<div class="terminal-bar">
				<div class="traffic-lights">
					<div class="light light-red"></div>
					<div class="light light-yellow"></div>
					<div class="light light-green"></div>
				</div>
				<span style="font-size: 12px; color: var(--text-dim); font-family: monospace;">bash - upy deploy output</span>
				<span style="width: 40px;"></span>
			</div>
			<div class="terminal-body" id="term-body">%s</div>
		</div>
	</div>
	<script>
		function copyLog() {
			const text = document.getElementById('term-body').innerText;
			navigator.clipboard.writeText(text).then(() => {
				alert('日志已复制到剪贴板！');
			});
		}
		// 自动滚动到最底部
		const term = document.getElementById('term-body');
		term.scrollTop = term.scrollHeight;
	</script>
</body>
</html>`,
		html.EscapeString(taskID),
		baseCSS,
		html.EscapeString(project),
		badgeClass, statusText,
		html.EscapeString(taskID),
		html.EscapeString(duration),
		logHTML,
	)
}
