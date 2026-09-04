// 部署通知: 用户家目录配置里的 Bark 渠道
// 通知失败只记警告, 不改变部署结果。
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
)

// Kind 通知类型
type Kind string

const (
	KindSuccess Kind = "success"
	KindFailure Kind = "failure"
)

// Event 一次部署通知
type Event struct {
	Kind    Kind
	Project string
	Version string
	Root    string
	Type    string
	Mode    string
	Elapsed string
	Error   string
}

var httpClient = &http.Client{Timeout: 8 * time.Second}

func nlJoin(parts []string) string {
	return strings.Join(parts, string(rune(10)))
}

// Send 按用户配置发送通知。未配置渠道时静默返回。
func Send(evt Event) {
	cfg, err := config.LoadUserConfig()
	if err != nil {
		log.Warn("读取通知配置失败: " + err.Error())
		return
	}
	barks := cfg.ActiveBarks()
	if len(barks) == 0 {
		return
	}

	title, body, groupSuffix, level := formatEvent(evt)
	sent := 0
	for _, b := range barks {
		if err := sendBark(b, title, body, groupSuffix, level); err != nil {
			log.Warn("通知 " + b.Label() + " 发送失败: " + log.Redact(err.Error()))
			continue
		}
		sent++
	}
	if sent > 0 {
		log.Info(fmt.Sprintf("已发送 %d 条部署通知", sent))
	}
}

func formatEvent(evt Event) (title, body, groupSuffix, level string) {
	name := strings.TrimSpace(evt.Project)
	if name == "" {
		name = "upy"
	}
	ver := strings.TrimSpace(evt.Version)
	if ver == "" {
		ver = "未知版本"
	}
	kindLabel := deployKindLabel(evt.Type, evt.Mode)
	lines := []string{
		"项目: " + name,
		"版本: " + ver,
		"类型: " + kindLabel,
		"目录: " + evt.Root,
	}
	if evt.Elapsed != "" {
		lines = append(lines, "耗时: "+evt.Elapsed)
	}
	if evt.Kind == KindFailure {
		if msg := strings.TrimSpace(evt.Error); msg != "" {
			lines = append(lines, "原因: "+msg)
		}
		return "Uply 部署失败 · " + name, nlJoin(lines), "failure", "timeSensitive"
	}
	return "Uply 部署成功 · " + name, nlJoin(lines), "success", "active"
}

func deployKindLabel(typ, mode string) string {
	switch typ {
	case "static":
		return "静态站点"
	case "docker":
		if mode == "compose" {
			return "Docker Compose 集成项目"
		}
		return "Docker 容器"
	default:
		if typ == "" {
			return "未知"
		}
		return typ
	}
}

type barkPayload struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	Group     string `json:"group,omitempty"`
	Sound     string `json:"sound,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Level     string `json:"level,omitempty"`
	IsArchive string `json:"isArchive,omitempty"`
}

func sendBark(b config.UserBark, title, body, groupSuffix, defaultLevel string) error {
	url := b.Endpoint() + "/" + b.Key()
	group := strings.TrimSpace(b.Group)
	if group == "" {
		group = "upy." + groupSuffix
	}
	level := strings.TrimSpace(b.Level)
	if level == "" {
		level = defaultLevel
	}
	payload := barkPayload{
		Title:     title,
		Body:      body,
		Group:     group,
		Sound:     strings.TrimSpace(b.Sound),
		Icon:      strings.TrimSpace(b.Icon),
		Level:     level,
		IsArchive: "1",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "upy-cli")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
