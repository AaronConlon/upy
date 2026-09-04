// GitHub Releases 客户端
// 认证: 优先 ~/.uply/config.yaml 的 github.token, 其次 DEPLOY_GITHUB_TOKEN。私有资产走 API 端点。
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AaronConlon/uply/internal/config"
	"github.com/AaronConlon/uply/internal/log"
)

const apiBase = "https://api.github.com"

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

func token() (string, error) {
	t, err := config.ResolveGitHubToken()
	if err != nil {
		return "", err
	}
	log.RegisterSecret(t)
	return t, nil
}

func newRequest(method, url, token string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "uply-cli")
	return req, nil
}

var client = &http.Client{Timeout: 60 * time.Second}

func ghGet(url, token string, allow404 bool) (*http.Response, error) {
	req, err := newRequest(http.MethodGet, url, token)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("仓库访问失败: %v", log.Redact(err.Error()))
	}
	if resp.StatusCode == 404 || resp.StatusCode == 403 {
		if allow404 {
			return resp, nil
		}
		repo := repoFromPath(url)
		return nil, fmt.Errorf("仓库 %s 不可访问（请检查 token 权限）。", repo)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API 错误 (%d): %s", resp.StatusCode, strings.TrimPrefix(url, apiBase))
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
	tok, err := token()
	if err != nil {
		return nil, err
	}
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
	tok, err := token()
	if err != nil {
		return nil, err
	}
	resp, err := ghGet(apiBase+"/repos/"+repo+"/releases/latest", tok, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("仓库 %s 没有已发布版本。", repo)
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
	tok, err := token()
	if err != nil {
		return nil, err
	}
	url := apiBase + "/repos/" + repo + "/releases/tags/" + tag
	resp, err := ghGet(url, tok, true)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("找不到版本 %s。", tag)
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
		return nil, fmt.Errorf("版本 %s 中不存在资产 \"%s\"。", release.Tag, name)
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
		return nil, fmt.Errorf("版本 %s 中找不到与项目相关的 bundle 资产（可用资产: %s）", release.Tag, listAssets(release))
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

// DownloadAsset 下载资产到 dest (走 API 资产端点, 私有仓库必需)
func DownloadAsset(repo string, asset *GHAsset, dest string) error {
	tok, err := token()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/repos/%s/releases/assets/%d", apiBase, repo, asset.ID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "uply-cli")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %v", log.Redact(err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("资产 %s 下载失败 (%d)", asset.Name, resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("下载失败: %v", log.Redact(err.Error()))
	}
	return nil
}

// PeekLatestTag 不强制 token 的 latest tag 探测 (version 自更新检查, 失败返回空)
func PeekLatestTag(repo string) string {
	req, err := http.NewRequest(http.MethodGet, apiBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "uply-cli")
	if t, err := config.LookupGitHubToken(); err == nil && t != "" {
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
