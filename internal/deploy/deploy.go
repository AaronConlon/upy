// DeployBundle 编排: 单一部署流程, Release 与 Bundle 模式统一入口
package deploy

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/bundle"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/deployer"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/notify"
	"github.com/AaronConlon/upy/internal/platform"
	"github.com/AaronConlon/upy/internal/projectlock"
	"github.com/AaronConlon/upy/internal/state"
	"github.com/AaronConlon/upy/internal/ui"
)

// Options 部署选项
type Options struct {
	Force     bool
	Root      string
	StartedAt time.Time
}

// Result 部署结果
type Result struct {
	Skipped bool
	Version string
	Project string
	Type    string
	Mode    string
}

// DeployBundle 部署 bundle.zip
func DeployBundle(zipPath string, opts Options) (*Result, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	res, err := deployBundle(zipPath, root, opts.Force, startedAt)
	notifyDeploy(res, err, root, startedAt)
	return res, err
}

func deployBundle(zipPath, root string, force bool, startedAt time.Time) (*Result, error) {
	lock, err := projectlock.Acquire(root, "部署")
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	if _, err := os.Stat(zipPath); err != nil {
		return nil, fmt.Errorf("未找到 bundle 文件: %s", zipPath)
	}

	// 保证 releases 目录就绪；临时解压目录建在 releasesDir 下，保证与目标目录在同一分区/设备，避免跨设备 rename 报错 (EXDEV)
	releasesDir := filepath.Join(root, "releases")
	if err := os.MkdirAll(releasesDir, 0o755); err != nil {
		return nil, err
	}
	_ = os.Chmod(releasesDir, 0o755)
	_ = os.Chmod(root, 0o755)

	tmp, err := os.MkdirTemp(releasesDir, ".tmp-deploy-")
	if err != nil {
		// 若 releasesDir 下创建临时目录失败，回退到系统临时目录
		tmp, err = os.MkdirTemp("", "upy-deploy-")
		if err != nil {
			return nil, err
		}
	}
	_ = os.Chmod(tmp, 0o755)
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			os.RemoveAll(tmp)
		}
	}()

	log.Step("正在解压 bundle...")
	if err := bundle.SafeExtract(zipPath, tmp); err != nil {
		return nil, err
	}
	log.Ok("bundle 解压完成")

	manifest, err := bundle.ReadManifest(tmp)
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("bundle: %s v%s (%s)", manifest.Name, stripV(manifest.Version), manifest.Type))

	// 平台兼容性
	to, ta := targetSpec(manifest)
	if !platform.IsCompatible(to, ta) {
		return nil, fmt.Errorf("bundle 目标平台 %s/%s 与当前主机 (%s/%s) 不兼容，拒绝部署",
			def(to, "未声明"), def(ta, "未声明"), platform.CurrentOS(), platform.CurrentArch())
	}

	st, err := state.Read(root)
	if err != nil {
		return nil, err
	}

	// 相同版本跳过
	if st.CurrentVersion != "" && st.CurrentVersion == manifest.Version && !force {
		log.Info(fmt.Sprintf("当前已是 %s，跳过部署（--force 可强制重新部署）", manifest.Version))
		elapsed := time.Since(startedAt).Seconds()
		log.Info("⏱ 部署耗时: " + log.Green(log.Bold(fmt.Sprintf("%.1fs", elapsed))))
		return &Result{Skipped: true, Version: manifest.Version, Project: manifest.Name, Type: manifest.Type, Mode: dockerMode(manifest)}, nil
	}

	releaseDir := filepath.Join(releasesDir, manifest.Version)
	log.Step("版本目录: " + log.Path(releaseDir))
	if _, err := os.Stat(releaseDir); err == nil {
		if err := os.RemoveAll(releaseDir); err != nil {
			return nil, err
		}
	}
	log.Step("正在安装版本 " + manifest.Version + "...")
	if err := moveDir(tmp, releaseDir); err != nil {
		return nil, err
	}
	cleanupTmp = false
	log.Ok("版本已安装到 releases/" + manifest.Version)

	// 规范化版本目录权限: 确保目录为 0755, 文件为 0644/0755, 供 Nginx 等 Web 服务器正常访问
	if err := EnsurePermissions(releaseDir); err != nil {
		log.Warn("设置版本目录权限失败: " + err.Error())
	}

	ctx := &deployer.DeployContext{
		Root:            root,
		ReleaseDir:      releaseDir,
		Version:         manifest.Version,
		Manifest:        manifest,
		ProjectName:     manifest.Name,
		PreviousVersion: st.CurrentVersion,
	}
	d, err := deployer.CreateDeployer(ctx)
	if err != nil {
		return nil, err
	}

	// Prepare / Activate / HealthCheck, 失败回滚
	if err := runDeployer(d); err != nil {
		log.Warn("部署失败，正在尝试回滚...")
		if rbErr := d.Rollback(); rbErr != nil {
			log.Warn("回滚出错: " + rbErr.Error())
		}
		return nil, err
	}

	// docker 类型额外切换 current
	if manifest.Type == "docker" {
		log.Step("正在切换 current -> releases/" + manifest.Version + "...")
		if err := deployer.AtomicSymlink(releaseDir, filepath.Join(root, "current")); err != nil {
			return nil, err
		}
	}

	newState := &state.State{
		CurrentVersion:  manifest.Version,
		PreviousVersion: st.CurrentVersion,
		DeployedAt:      time.Now().Format(time.RFC3339),
	}
	if err := state.WriteAtomic(root, newState); err != nil {
		return nil, err
	}
	log.Ok("部署完成: " + manifest.Name + " " + manifest.Version)

	// 先清理旧版本目录, 再输出最终目录结构, 最后打印耗时
	cleanupOldReleases(root, newState)
	ui.PrintProjectLayout(root)

	elapsed := time.Since(startedAt).Seconds()
	log.Info("⏱ 部署耗时: " + log.Green(log.Bold(fmt.Sprintf("%.1fs", elapsed))))

	return &Result{Skipped: false, Version: manifest.Version, Project: manifest.Name, Type: manifest.Type, Mode: dockerMode(manifest)}, nil
}

func runDeployer(d deployer.Deployer) error {
	if err := d.Prepare(); err != nil {
		return err
	}
	// 新版本构建成功后, 先停掉上一版本再启动新版本, 避免端口/容器名冲突
	if err := d.Deactivate(); err != nil {
		return err
	}
	if err := d.Activate(); err != nil {
		return err
	}
	return d.HealthCheck()
}

// cleanupOldReleases 只保留当前版本, 其余全部清理, 不留数据
func cleanupOldReleases(root string, st *state.State) {
	releasesDir := filepath.Join(root, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".tmp-deploy-") {
			_ = os.RemoveAll(filepath.Join(releasesDir, e.Name()))
			continue
		}
		if e.Name() == st.CurrentVersion {
			continue
		}
		log.Info("🗑 清理旧版本目录 " + e.Name())
		os.RemoveAll(filepath.Join(releasesDir, e.Name()))
	}
}

// moveDir 尝试将 src 移动到 dst。优先使用原子重命名 (os.Rename)；若跨设备/分区 (EXDEV) 则回退到递归复制 + 删除
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// 跨文件系统 (EXDEV) 或其他 rename 失败时，回退到 copy + remove
	if err := copyDir(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return fmt.Errorf("移动目录失败，且跨分区复制失败: %w", err)
	}
	_ = os.RemoveAll(src)
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, target)
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func stripV(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func targetSpec(m *config.Manifest) (string, string) {
	if m.Target == nil {
		return "", ""
	}
	return m.Target.OS, m.Target.Arch
}

func def(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func notifyDeploy(res *Result, err error, root string, startedAt time.Time) {
	evt := notify.Event{
		Root:    root,
		Elapsed: fmt.Sprintf("%.1fs", time.Since(startedAt).Seconds()),
	}
	if res != nil {
		evt.Version = res.Version
		evt.Project = res.Project
		evt.Type = res.Type
		evt.Mode = res.Mode
	}
	if err != nil {
		evt.Kind = notify.KindFailure
		evt.Error = err.Error()
		notify.Send(evt)
		return
	}
	if res == nil || res.Skipped {
		return
	}
	evt.Kind = notify.KindSuccess
	notify.Send(evt)
}

func dockerMode(m *config.Manifest) string {
	if m == nil || m.Docker == nil {
		return ""
	}
	return m.Docker.Mode
}

// EnsurePermissions 递归确保目录为 0755，普通文件至少为 0644 (若含可执行位则设为 0755)。
// 忽略符号链接，避免操作系统变更符号链接目标文件的权限或跨系统报错。
func EnsurePermissions(rootPath string) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if info.IsDir() {
			return os.Chmod(path, 0o755)
		}
		if info.Mode().Perm()&0o111 != 0 {
			return os.Chmod(path, 0o755)
		}
		return os.Chmod(path, 0o644)
	})
}
