// GitHub Releases 客户端
// 认证: 优先 ~/.upy/config.yaml 的 github.token, 其次 DEPLOY_GITHUB_TOKEN。私有资产走 API 端点。
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/i18n"
	"github.com/AaronConlon/upy/internal/log"
)

var apiBase = "https://api.github.com"

// GHAsset Release 资产
type GHAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// GHRelease Release 信息
type GHRelease struct {
	Tag        string    `json:"tag_name"`
	Name       string    `json:"name"`
	Prerelease bool      `json:"prerelease"`
	Draft      bool      `json:"draft"`
	Assets     []GHAsset `json:"assets"`
}

func token(repo string) string {
	t, _ := config.LookupGitHubTokenForRepo(repo)
	if t != "" {
		log.RegisterSecret(t)
	}
	return t
}

func newRequest(method, url, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "upy-cli")
	return req, nil
}

// envProxy 优先读取 HTTP_PROXY / HTTPS_PROXY，其次支持标准库默认忽略的 ALL_PROXY / all_proxy (包括 socks5://)
func envProxy(req *http.Request) (*url.URL, error) {
	if u, err := http.ProxyFromEnvironment(req); err == nil && u != nil {
		return u, nil
	}
	all := strings.TrimSpace(os.Getenv("ALL_PROXY"))
	if all == "" {
		all = strings.TrimSpace(os.Getenv("all_proxy"))
	}
	if all == "" {
		return nil, nil
	}
	if isLoopback(req.URL.Host) || !useProxyForHost(req.URL.Host) {
		return nil, nil
	}
	if !strings.Contains(all, "://") {
		all = "http://" + all
	}
	return url.Parse(all)
}

func isLoopback(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	if h == "localhost" || h == "127.0.0.1" || h == "::1" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func useProxyForHost(host string) bool {
	noProxy := strings.TrimSpace(os.Getenv("NO_PROXY"))
	if noProxy == "" {
		noProxy = strings.TrimSpace(os.Getenv("no_proxy"))
	}
	if noProxy == "*" {
		return false
	}
	if noProxy == "" {
		return true
	}
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.ToLower(strings.TrimSpace(h))
	for _, p := range strings.Split(noProxy, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if p == "*" || p == h || strings.HasSuffix(h, "."+strings.TrimPrefix(p, ".")) {
			return false
		}
	}
	return true
}

func newTransport() *http.Transport {
	var t *http.Transport
	if dt, ok := http.DefaultTransport.(*http.Transport); ok {
		t = dt.Clone()
	} else {
		t = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
	t.Proxy = envProxy
	return t
}

var client = &http.Client{
	Timeout:   60 * time.Second,
	Transport: newTransport(),
}

// downloadClient 用于下载大资产，不设整体超时以避免大文件下载中断，通过响应头超时保证不挂死
var downloadClient = &http.Client{
	Transport: func() *http.Transport {
		t := newTransport()
		t.ResponseHeaderTimeout = 60 * time.Second
		return t
	}(),
}

func ghGet(url, token string, allow404 bool) (*http.Response, error) {
	req, err := newRequest(http.MethodGet, url, token)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("仓库访问失败: %v"), log.Redact(err.Error()))
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 404 {
		if allow404 && resp.StatusCode == 404 {
			return resp, nil
		}
		repo := repoFromPath(url)
		// 若没有 token 或 token 失效，给出针对性的友好提示
		if _, err := config.ResolveGitHubTokenForRepo(repo); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf(i18n.T("仓库 %s 无法访问 (HTTP %d，请检查 token 权限)。"), repo, resp.StatusCode)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(i18n.T("GitHub API 错误 (%d): %s"), resp.StatusCode, strings.TrimPrefix(url, apiBase))
	}
	return resp, nil
}

func repoFromPath(url string) string {
	// /repos/owner/repo/releases?per_page=100
	rest := strings.TrimPrefix(url, apiBase+"/repos/")
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return "?"
}

// ListReleases 列出 repo 的 releases (排除 draft)
func ListReleases(repo string) ([]GHRelease, error) {
	tok := token(repo)
	resp, err := ghGet(apiBase+"/repos/"+repo+"/releases?per_page=100", tok, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rels []GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&rels); err != nil {
		return nil, err
	}
	out := rels[:0]
	for _, r := range rels {
		if !r.Draft {
			out = append(out, r)
		}
	}
	return out, nil
}

// GetLatest 取最新正式 release (GitHub 的 latest)
func GetLatest(repo string) (*GHRelease, error) {
	tok := token(repo)
	resp, err := ghGet(apiBase+"/repos/"+repo+"/releases/latest", tok, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		owner := config.ParseRepoOwner(repo)
		if tok == "" {
			return nil, fmt.Errorf(i18n.T("仓库 %s 不可访问或没有发布版本。若为私有仓库，请通过 upy init %s <token> 添加访问权限。"), repo, owner)
		}
		return nil, fmt.Errorf(i18n.T("仓库 %s 没有已发布的正式版本（请检查该 token 是否具有目标仓库权限）。"), repo)
	}
	defer resp.Body.Close()
	var r GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// GetByTag 按 tag 精确取 release
func GetByTag(repo, tag string) (*GHRelease, error) {
	tok := token(repo)
	url := apiBase + "/repos/" + repo + "/releases/tags/" + tag
	resp, err := ghGet(url, tok, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		owner := config.ParseRepoOwner(repo)
		if tok == "" {
			return nil, fmt.Errorf(i18n.T("仓库 %s 找不到版本 %s。若为私有仓库，请通过 upy init %s <token> 添加访问权限。"), repo, tag, owner)
		}
		return nil, fmt.Errorf(i18n.T("仓库 %s 找不到版本 %s（请检查版本号或 token 权限）。"), repo, tag)
	}
	defer resp.Body.Close()
	var r GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// FindAsset 定位 bundle 资产:
//  1. 精确匹配配置名 (如 bundle.zip) → 直接返回
//  2. 未命中且提供了项目名线索时, 模糊匹配: 资产名含项目名 + 含版本号,
//     支持 "项目名-版本tag-日期.zip" 这类动态命名
//
// projectName 与 repoShort 可传空, 传空则跳过模糊匹配 (如自更新资产名固定)
func FindAsset(release *GHRelease, name, projectName, repoShort string) (*GHAsset, error) {
	for i := range release.Assets {
		if release.Assets[i].Name == name {
			return &release.Assets[i], nil
		}
	}

	names := make([]string, 0, 2)
	if pn := strings.ToLower(strings.TrimSpace(projectName)); pn != "" {
		names = append(names, pn)
	}
	if rs := strings.ToLower(strings.TrimSpace(repoShort)); rs != "" {
		names = append(names, rs)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf(i18n.T("版本 %s 中不存在资产 \"%s\"。"), release.Tag, name)
	}

	normTag := strings.TrimPrefix(release.Tag, "v")
	var candidates []*GHAsset
	for i := range release.Assets {
		a := &release.Assets[i]
		n := strings.ToLower(a.Name)
		if !strings.HasSuffix(n, ".zip") {
			continue
		}
		hasName := false
		for _, nm := range names {
			if strings.Contains(n, nm) {
				hasName = true
				break
			}
		}
		if !hasName || !strings.Contains(n, normTag) {
			continue
		}
		candidates = append(candidates, a)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf(i18n.T("版本 %s 中找不到与项目相关的 bundle 资产（可用资产: %s）"), release.Tag, listAssets(release))
	}
	// 多个候选时优先匹配含完整 "v<tag>" 的 (如 v0.5.1)
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c.Name), strings.ToLower(release.Tag)) {
			return c, nil
		}
	}
	return candidates[0], nil
}

func listAssets(release *GHRelease) string {
	names := make([]string, 0, len(release.Assets))
	for _, a := range release.Assets {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

const (
	maxDownloadRetries = 3
	readIdleTimeout    = 30 * time.Second
)

type progressReader struct {
	reader      io.Reader
	total       int64
	initial     int64
	current     int64
	startTime   time.Time
	lastUpdate  time.Time
	idleTimer   *time.Timer
	idleTimeout time.Duration
	isTTY       bool
}

func newProgressReader(r io.Reader, total, initial int64, idleTimer *time.Timer, idleTimeout time.Duration) *progressReader {
	return &progressReader{
		reader:      r,
		total:       total,
		initial:     initial,
		current:     initial,
		startTime:   time.Now(),
		lastUpdate:  time.Now(),
		idleTimer:   idleTimer,
		idleTimeout: idleTimeout,
		isTTY:       log.UseColor,
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.current += int64(n)
		if pr.idleTimer != nil {
			pr.idleTimer.Reset(pr.idleTimeout)
		}
		pr.render()
	}
	return n, err
}

func (pr *progressReader) render() {
	if !pr.isTTY {
		return
	}
	now := time.Now()
	// 控制刷新频率为 100ms 一次，避免刷屏卡顿；若已全部读完则立即刷新
	if now.Sub(pr.lastUpdate) < 100*time.Millisecond && (pr.total <= 0 || pr.current < pr.total) {
		return
	}
	pr.lastUpdate = now

	elapsed := now.Sub(pr.startTime).Seconds()
	if elapsed <= 0 {
		elapsed = 0.001
	}
	// 计算当前轮次的实时传输速度，避免加上续传的历史字节
	readInThisRound := pr.current - pr.initial
	speed := float64(readInThisRound) / elapsed

	var line string
	if pr.total > 0 {
		percent := float64(pr.current) / float64(pr.total) * 100
		if percent > 100 {
			percent = 100
		}
		barWidth := 20
		filled := int(float64(barWidth) * float64(pr.current) / float64(pr.total))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("=", filled)
		if filled < barWidth {
			bar += ">" + strings.Repeat(" ", barWidth-filled-1)
		}
		line = fmt.Sprintf("\r    [%s] %5.1f%% %s/%s (%s/s)",
			bar, percent, log.HumanSize(pr.current), log.HumanSize(pr.total), log.HumanSize(int64(speed)))
	} else {
		line = fmt.Sprintf("\r    %s (%s/s)", log.HumanSize(pr.current), log.HumanSize(int64(speed)))
	}

	os.Stderr.WriteString(line)
}

func (pr *progressReader) finish() {
	if pr.isTTY {
		os.Stderr.WriteString("\r\x1b[2K\r")
	}
}

// DownloadAsset 下载资产到 dest (走 API 资产端点，失败时自动降级公开直链)
func DownloadAsset(repo string, asset *GHAsset, dest string) error {
	tok := token(repo)
	if tok != "" {
		// 具有 token，优先走 GitHub API 资产端点 (私有仓库必需)
		url := fmt.Sprintf("%s/repos/%s/releases/assets/%d", apiBase, repo, asset.ID)
		err := downloadToFile(url, tok, asset, dest)
		if err == nil {
			return nil
		}
		// 若 API 端点下载失败且存在公开下载直链，自动降级重试公开直链 (与 install.sh 保持一致)
		if asset.URL != "" {
			log.Info(fmt.Sprintf(i18n.T("API 资产端点下载失败 (%v)，正在降级尝试公开直链..."), log.Redact(err.Error())))
			if err2 := downloadToFile(asset.URL, "", asset, dest); err2 == nil {
				return nil
			}
		}
		return err
	}

	// 无 token，公开仓库直接通过 Browser Download URL 下载
	if asset.URL == "" {
		return fmt.Errorf(i18n.T("资产 %s 无公开下载地址，请配置 GitHub token"), asset.Name)
	}
	return downloadToFile(asset.URL, "", asset, dest)
}

// DownloadSelfUpdateAsset 仅转发 upy 公共 Release 资产，不携带 GitHub Token。
// 保留现有下载重试、断点续传和大小检查，安装端仍执行 SHA-256 校验。
func DownloadSelfUpdateAsset(asset *GHAsset, dest string) error {
	proxyURL, err := selfUpdateProxyURL(asset.URL)
	if err != nil {
		return err
	}
	return downloadToFile(proxyURL, "", asset, dest)
}

func selfUpdateProxyURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/AaronConlon/upy/releases/download/") {
		return "", i18n.Error("自更新加速只支持 AaronConlon/upy 的 GitHub Release 公开下载地址")
	}
	return "https://ghfast.top/" + raw, nil
}

func downloadToFile(url, token string, asset *GHAsset, dest string) error {
	tmpDest := fmt.Sprintf("%s.tmp-%d", dest, time.Now().UnixNano())
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpDest)
		}
	}()

	var lastErr error
	for attempt := 1; attempt <= maxDownloadRetries+1; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt-1) * time.Second
			time.Sleep(backoff)
		}

		var existingSize int64
		if fi, err := os.Stat(tmpDest); err == nil {
			existingSize = fi.Size()
		}

		if asset.Size > 0 && existingSize == asset.Size {
			if err := os.Rename(tmpDest, dest); err != nil {
				return err
			}
			success = true
			return nil
		}

		ctx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			return err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Accept", "application/octet-stream")
		}
		if existingSize > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		}
		req.Header.Set("User-Agent", "upy-cli")

		idleTimer := time.AfterFunc(readIdleTimeout, func() {
			cancel()
		})

		resp, err := downloadClient.Do(req)
		if err != nil {
			idleTimer.Stop()
			cancel()
			lastErr = err
			if attempt <= maxDownloadRetries {
				log.Warn(fmt.Sprintf(i18n.T("下载连接中断 (%v)，正在尝试断点续传 (第 %d/%d 次)..."),
					log.Redact(err.Error()), attempt, maxDownloadRetries))
				continue
			}
			return fmt.Errorf(i18n.T("下载失败: %v"), log.Redact(err.Error()))
		}

		isPartial := resp.StatusCode == http.StatusPartialContent
		isOK := resp.StatusCode == http.StatusOK
		isRangeInvalid := resp.StatusCode == http.StatusRequestedRangeNotSatisfiable

		if isRangeInvalid {
			resp.Body.Close()
			idleTimer.Stop()
			cancel()
			_ = os.Truncate(tmpDest, 0)
			if attempt <= maxDownloadRetries {
				log.Warn(fmt.Sprintf(i18n.T("Range 请求无效 (HTTP 416)，已重置为从头下载 (第 %d/%d 次)..."), attempt, maxDownloadRetries))
				continue
			}
			return fmt.Errorf(i18n.T("资产 %s 下载失败 (HTTP 416)"), asset.Name)
		}

		if !isPartial && !isOK {
			resp.Body.Close()
			idleTimer.Stop()
			cancel()
			lastErr = fmt.Errorf(i18n.T("资产 %s 下载失败 (%d)"), asset.Name, resp.StatusCode)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return lastErr
			}
			if attempt <= maxDownloadRetries {
				log.Warn(fmt.Sprintf(i18n.T("服务端异常 (%d)，准备重试 (第 %d/%d 次)..."), resp.StatusCode, attempt, maxDownloadRetries))
				continue
			}
			return lastErr
		}

		var out *os.File
		var currentOffset int64
		if isPartial {
			out, err = os.OpenFile(tmpDest, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
			currentOffset = existingSize
		} else {
			out, err = os.Create(tmpDest)
			currentOffset = 0
		}
		if err != nil {
			resp.Body.Close()
			idleTimer.Stop()
			cancel()
			return err
		}

		totalSize := asset.Size
		if totalSize <= 0 {
			if isPartial {
				if cr := resp.Header.Get("Content-Range"); cr != "" {
					if slashIdx := strings.LastIndex(cr, "/"); slashIdx != -1 {
						if tot, parseErr := strconv.ParseInt(strings.TrimSpace(cr[slashIdx+1:]), 10, 64); parseErr == nil {
							totalSize = tot
						}
					}
				}
			} else if resp.ContentLength > 0 {
				totalSize = resp.ContentLength
			}
		}

		pr := newProgressReader(resp.Body, totalSize, currentOffset, idleTimer, readIdleTimeout)
		_, copyErr := io.Copy(out, pr)
		pr.finish()
		idleTimer.Stop()
		out.Close()
		resp.Body.Close()
		cancel()

		if copyErr != nil {
			lastErr = copyErr
			if attempt <= maxDownloadRetries {
				log.Warn(fmt.Sprintf(i18n.T("传输中断 (%v)，正在尝试断点续传 (第 %d/%d 次)..."),
					log.Redact(copyErr.Error()), attempt, maxDownloadRetries))
				continue
			}
			return fmt.Errorf(i18n.T("下载失败: %v"), log.Redact(copyErr.Error()))
		}

		fi, statErr := os.Stat(tmpDest)
		if statErr != nil {
			return statErr
		}
		if asset.Size > 0 && fi.Size() != asset.Size {
			lastErr = fmt.Errorf(i18n.T("资产 %s 下载不完整: 预期 %d 字节，实际得到 %d 字节"), asset.Name, asset.Size, fi.Size())
			if attempt <= maxDownloadRetries {
				log.Warn(fmt.Sprintf(i18n.T("下载大小不一致 (已下载 %s / 预期 %s)，正在重试续传 (第 %d/%d 次)..."),
					log.HumanSize(fi.Size()), log.HumanSize(asset.Size), attempt, maxDownloadRetries))
				continue
			}
			return lastErr
		}

		if err := os.Rename(tmpDest, dest); err != nil {
			return err
		}
		success = true
		return nil
	}

	return lastErr
}

// PeekLatestTag 不强制 token 的 latest tag 探测 (version 自更新检查, 失败返回空)
func PeekLatestTag(repo string) string {
	req, err := http.NewRequest(http.MethodGet, apiBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "upy-cli")
	if t, err := config.LookupGitHubTokenForRepo(repo); err == nil && t != "" {
		log.RegisterSecret(t)
		req.Header.Set("Authorization", "Bearer "+t)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var r GHRelease
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return ""
	}
	return r.Tag
}
