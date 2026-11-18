package updatecheck

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/github"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/semver"
)

// DefaultCheckInterval 默认缓存有效期（12小时），避免频繁请求 GitHub API
const DefaultCheckInterval = 12 * time.Hour

// DefaultRepo upy 官方仓库
const DefaultRepo = "AaronConlon/upy"

// CacheData 本地更新检查缓存
type CacheData struct {
	LastCheckedAt int64  `json:"last_checked_at"`
	LatestVersion string `json:"latest_version"`
}

// Checker 低优先级更新检测器
type Checker struct {
	enabled        bool
	currentVersion string
	repo           string
	cachePath      string
	newVersion     string
	resultChan     chan string
}

// Start 启动低优先级版本检测 (异步非阻塞)
func Start(userCfg *config.UserConfig, currentVersion, repo, cachePath string) *Checker {
	if userCfg != nil && !userCfg.Update.IsCheckEnabled() {
		return &Checker{enabled: false}
	}
	if repo == "" {
		repo = DefaultRepo
	}
	if cachePath == "" {
		cachePath = filepath.Join(config.UserDataDir(), "update_check.json")
	}

	c := &Checker{
		enabled:        true,
		currentVersion: currentVersion,
		repo:           repo,
		cachePath:      cachePath,
	}

	// 1. 读取本地缓存
	cache := loadCache(cachePath)

	// 如果缓存有效（在 12 小时内检查过）
	if cache.LastCheckedAt > 0 && time.Since(time.Unix(cache.LastCheckedAt, 0)) < DefaultCheckInterval {
		if semver.Newer(cache.LatestVersion, currentVersion) {
			c.newVersion = cache.LatestVersion
		}
		// 缓存新鲜，无需发起网络请求
		return c
	}

	// 如果缓存过时但有版本记录，且该版本比当前新，可暂存作为备选
	if cache.LatestVersion != "" && semver.Newer(cache.LatestVersion, currentVersion) {
		c.newVersion = cache.LatestVersion
	}

	// 2. 异步后台拉取最新 tag (低优先级，4 秒超时)
	c.resultChan = make(chan string, 1)
	go func() {
		tag := github.PeekLatestTagWithTimeout(repo, 4*time.Second)
		if tag != "" {
			_ = saveCache(cachePath, CacheData{
				LastCheckedAt: time.Now().Unix(),
				LatestVersion: tag,
			})
			if semver.Newer(tag, currentVersion) {
				c.resultChan <- tag
				return
			}
		} else {
			// 网络失败时记录时间戳退避，避免离线时频繁重试
			_ = saveCache(cachePath, CacheData{
				LastCheckedAt: time.Now().Unix(),
				LatestVersion: cache.LatestVersion,
			})
		}
		c.resultChan <- ""
	}()

	return c
}

// Notify 在指令最终输出后输出配色提示文案 (非阻塞或毫秒级等待)
func (c *Checker) Notify(w io.Writer) {
	if c == nil || !c.enabled {
		return
	}
	if w == nil {
		w = os.Stderr
	}

	// 如果后台协程还在运行，最多等待一小段极短时间（首次无缓存时最多等 1.2s），后续直接走本地缓存零延迟
	if c.resultChan != nil {
		select {
		case tag := <-c.resultChan:
			if tag != "" && semver.Newer(tag, c.currentVersion) {
				c.newVersion = tag
			}
		case <-time.After(1200 * time.Millisecond):
			// 超时放行，沿用缓存新版本 (如果有的话)
		}
	}

	if c.newVersion == "" || !semver.Newer(c.newVersion, c.currentVersion) {
		return
	}

	prompt := RenderPrompt(c.currentVersion, c.newVersion)
	fmt.Fprintln(w, prompt)
}

// RenderPrompt 生成良好配色的终端提示框
func RenderPrompt(current, latest string) string {
	normCurrent := current
	if !strings.HasPrefix(normCurrent, "v") {
		normCurrent = "v" + normCurrent
	}
	normLatest := latest
	if !strings.HasPrefix(normLatest, "v") {
		normLatest = "v" + normLatest
	}

	line1 := fmt.Sprintf(i18n.T("  🔔 发现新版本: %s %s%s%s"),
		log.Bold("upy"),
		log.Dim(normCurrent),
		log.Dim(" → "),
		log.Green(log.Bold(normLatest)),
	)
	line2 := fmt.Sprintf(i18n.T("  运行 %s 即可升级至最新版本"),
		log.Sky(log.Bold("upy update")),
	)

	w1 := stringWidth(line1)
	w2 := stringWidth(line2)
	innerWidth := max(w1, w2) + 2

	pad1 := strings.Repeat(" ", innerWidth-w1)
	pad2 := strings.Repeat(" ", innerWidth-w2)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(log.Yellow("  ╭" + strings.Repeat("─", innerWidth+2) + "╮\n"))
	sb.WriteString(log.Yellow("  │ ") + line1 + pad1 + log.Yellow(" │\n"))
	sb.WriteString(log.Yellow("  │ ") + line2 + pad2 + log.Yellow(" │\n"))
	sb.WriteString(log.Yellow("  ╰" + strings.Repeat("─", innerWidth+2) + "╯"))
	return sb.String()
}

func loadCache(path string) CacheData {
	data, err := os.ReadFile(path)
	if err != nil {
		return CacheData{}
	}
	var c CacheData
	_ = json.Unmarshal(data, &c)
	return c
}

func saveCache(path string, c CacheData) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// stringWidth 计算字符串在终端中的可视列宽 (考虑 ANSI 转义字符与全角/Emoji 字符宽度)
func stringWidth(s string) int {
	clean := log.StripAnsi(s)
	w := 0
	for _, r := range clean {
		if isWideRune(r) {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func isWideRune(r rune) bool {
	if r == '🔔' || r == '🎉' || r == '🚀' || r == '📦' || r == '📁' || r == '🏷' || r == '🧩' || r == '📋' || r == '🚨' || r == 0x26a0 {
		return true
	}
	if (r >= 0x2e80 && r <= 0x9fff) ||
		(r >= 0xac00 && r <= 0xd7af) ||
		(r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xff01 && r <= 0xff60) ||
		(r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1f9ff) {
		return true
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
