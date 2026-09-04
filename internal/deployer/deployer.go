// Deployer 接口 + 共享辅助 (健康轮询 / 名称归一化 / 原子符号链接 / shared env 路径)
package deployer

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
)

// DeployContext 部署上下文
type DeployContext struct {
	Root            string // 项目根
	ReleaseDir      string // releases/<version>
	Version         string
	Manifest        *config.Manifest
	ProjectName     string
	PreviousVersion string
}

// Deployer 部署器接口
type Deployer interface {
	// Prepare 准备阶段 (compose: 构建镜像; static: 校验 bundle)
	Prepare() error
	// Deactivate 新版本准备成功后、启动新版本前, 停掉上一版本
	// (compose: 对上一版本执行 down --remove-orphans 释放端口/容器名; 其他类型可为空实现)
	Deactivate() error
	Activate() error
	HealthCheck() error
	Rollback() error
}

// Slug 容器/镜像名归一化: 小写, 非 [a-z0-9.-] 替换为 "-", 首尾去 "-"
func Slug(s string) string {
	re := regexp.MustCompile(`[^a-z0-9.-]+`)
	v := re.ReplaceAllString(strings.ToLower(s), "-")
	v = strings.Trim(v, "-")
	if v == "" {
		return "app"
	}
	return v
}

// SharedEnvPath 返回 shared/.env 路径
func SharedEnvPath(root string) string {
	return filepath.Join(root, "shared", ".env")
}

// SharedEnvFile 返回存在的 shared/.env 路径, 否则空字符串
func SharedEnvFile(root string) string {
	p := SharedEnvPath(root)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// PollHealth 每 1s 轮询 url, 2xx 视为成功, 超时返回错误
func PollHealth(url string, timeoutSec int) error {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	lastErr := ""
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Sprintf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err.Error()
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("健康检查超时（%ds）: %s", timeoutSec, log.Redact(lastErr))
}

// AtomicSymlink 原子切换符号链接 (临时 symlink + rename)
// 默认生成相对路径链接 (从 link 所在目录到 target), 便于容器挂载/目录迁移
func AtomicSymlink(target, linkPath string) error {
	linkTarget := target
	if rel, err := filepath.Rel(filepath.Dir(linkPath), target); err == nil {
		linkTarget = rel
	}
	tmp := linkPath + ".upy-tmp-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := os.Symlink(linkTarget, tmp); err != nil {
		return fmt.Errorf("无法创建符号链接: %v", log.Redact(err.Error()))
	}
	if err := os.Rename(tmp, linkPath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("无法激活符号链接: %v", log.Redact(err.Error()))
	}
	return nil
}
