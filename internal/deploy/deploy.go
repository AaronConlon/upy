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
	"github.com/AaronConlon/upy/internal/cleaner"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/crypto"
	"github.com/AaronConlon/upy/internal/deployer"
	"github.com/AaronConlon/upy/internal/i18n"
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
	lock, err := projectlock.Acquire(root, i18n.T("部署"))
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	if _, err := os.Stat(zipPath); err != nil {
		return nil, fmt.Errorf(i18n.T("未找到 bundle 文件: %s"), zipPath)
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

	// 检查 bundle 是否加密并根据 deploy.yaml 配置解密
	targetZip := zipPath
	projCfg, _ := config.ReadProjectConfig(root)
	cryptoPassword := projCfg.CryptoPassword()

	isEnc, err := crypto.IsEncryptedFile(zipPath)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("检查 bundle 文件失败: %w"), err)
	}

	if isEnc {
		if cryptoPassword == "" {
			return nil, i18n.Error("检测到 bundle 已加密，但 deploy.yaml 中未配置解密密码 (crypto.password)")
		}
		log.Step(i18n.T("正在解密 bundle..."))
		decryptedZip := filepath.Join(releasesDir, fmt.Sprintf(".decrypted-%d.zip", time.Now().UnixNano()))
		defer os.Remove(decryptedZip)
		if err := crypto.DecryptFile(zipPath, decryptedZip, cryptoPassword); err != nil {
			return nil, err
		}
		log.Ok(i18n.T("bundle 解密完成"))
		targetZip = decryptedZip
	} else if cryptoPassword != "" {
		log.Info(i18n.T("bundle 未加密，直接解压部署"))
	}

	log.Step(i18n.T("正在解压 bundle..."))
	if err := bundle.SafeExtract(targetZip, tmp); err != nil {
		return nil, err
	}
	log.Ok(i18n.T("bundle 解压完成"))

	manifest, err := bundle.ReadManifest(tmp)
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("bundle: %s v%s (%s)", manifest.Name, stripV(manifest.Version), manifest.Type))

	// 平台兼容性
	to, ta := targetSpec(manifest)
	if !platform.IsCompatible(to, ta) {
		return nil, fmt.Errorf(i18n.T("bundle 目标平台 %s/%s 与当前主机 (%s/%s) 不兼容，拒绝部署"),
			def(to, i18n.T("未声明")), def(ta, i18n.T("未声明")), platform.CurrentOS(), platform.CurrentArch())
	}

	st, err := state.Read(root)
	if err != nil {
		return nil, err
	}

	// 相同版本跳过
	if st.CurrentVersion != "" && st.CurrentVersion == manifest.Version && !force {
		log.Info(fmt.Sprintf(i18n.T("当前已是 %s，跳过部署（--force 可强制重新部署）"), manifest.Version))
		elapsedStr := ui.FormatDuration(time.Since(startedAt))
		log.Info(i18n.T("⏱ 部署耗时: ") + log.Green(log.Bold(elapsedStr)))
		return &Result{Skipped: true, Version: manifest.Version, Project: manifest.Name, Type: manifest.Type, Mode: dockerMode(manifest)}, nil
	}

	releaseDir := filepath.Join(releasesDir, manifest.Version)
	log.Step(i18n.T("版本目录: ") + log.Path(releaseDir))
	var backupReleaseDir string
	if _, err := os.Stat(releaseDir); err == nil {
		// SYS-005: 同版本 force 重发时保留旧版本目录备份，避免在 Deactivate 前就地删除正在运行的产物
		backupReleaseDir = filepath.Join(releasesDir, fmt.Sprintf(".old-%s-%d", manifest.Version, time.Now().UnixNano()))
		if err := moveDir(releaseDir, backupReleaseDir); err != nil {
			return nil, fmt.Errorf(i18n.T("暂存既有版本目录失败: %w"), err)
		}
	}
	log.Step(i18n.T("正在安装版本 ") + manifest.Version + "...")
	if err := moveDir(tmp, releaseDir); err != nil {
		if backupReleaseDir != "" {
			_ = moveDir(backupReleaseDir, releaseDir)
		}
		return nil, err
	}
	cleanupTmp = false
	log.Ok(i18n.T("版本已安装到 releases/") + manifest.Version)

	// 规范化版本目录权限: 确保目录为 0755, 文件为 0644/0755, 供 Nginx 等 Web 服务器正常访问
	if err := EnsurePermissions(releaseDir); err != nil {
		log.Warn(i18n.T("设置版本目录权限失败: ") + err.Error())
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
		if backupReleaseDir != "" {
			_ = os.RemoveAll(releaseDir)
			_ = moveDir(backupReleaseDir, releaseDir)
		}
		return nil, err
	}

	// Prepare 阶段：校验可执行文件、环境、权限、既有归属等。若 Prepare 失败，绝不能调用 Rollback 触碰或停掉原服务！
	if err := d.Prepare(); err != nil {
		if backupReleaseDir != "" {
			_ = os.RemoveAll(releaseDir)
			_ = moveDir(backupReleaseDir, releaseDir)
		}
		return nil, err
	}

	// 激活与健康检查阶段：停掉上一版本并启动新版本。若失败，执行回滚
	if err := runActiveDeployer(d); err != nil {
		log.Warn(i18n.T("部署失败，正在尝试回滚..."))
		if backupReleaseDir != "" {
			// SYS-15: 必须在调用 Rollback() 重新拉起旧服务前，先将原版本的二进制和配置还原到 releaseDir！
			_ = os.RemoveAll(releaseDir)
			_ = moveDir(backupReleaseDir, releaseDir)
			backupReleaseDir = ""
		}
		if rbErr := d.Rollback(); rbErr != nil {
			log.Warn(i18n.T("回滚出错: ") + rbErr.Error())
		}
		return nil, err
	}

	if backupReleaseDir != "" {
		_ = os.RemoveAll(backupReleaseDir)
	}

	// docker / system 类型额外切换 current 软链
	if manifest.Type == "docker" || manifest.Type == "system" || manifest.Type == "service" {
		log.Step(i18n.T("正在切换 current -> releases/") + manifest.Version + "...")
		if err := deployer.AtomicSymlink(releaseDir, filepath.Join(root, "current")); err != nil {
			return nil, err
		}
	}

	unitName := ""
	if sysD, ok := d.(*deployer.SystemDeployer); ok {
		unitName = sysD.UnitName()
	}
	newState := &state.State{
		CurrentVersion:  manifest.Version,
		PreviousVersion: st.CurrentVersion,
		DeployedAt:      time.Now().Format(time.RFC3339),
		Type:            manifest.Type,
		UnitName:        unitName,
		Status:          "active",
	}
	if err := state.WriteAtomic(root, newState); err != nil {
		return nil, err
	}
	log.Ok(i18n.T("部署完成: ") + manifest.Name + " " + manifest.Version)

	// 先清理旧版本目录与旧 Bundle 缓存, 再输出最终目录结构, 最后打印耗时
	cleanupOldReleases(root, newState)
	cleanupOldBundles(root)
	ui.PrintProjectLayout(root)

	elapsedStr := ui.FormatDuration(time.Since(startedAt))
	log.Info(i18n.T("⏱ 部署耗时: ") + log.Green(log.Bold(elapsedStr)))

	return &Result{Skipped: false, Version: manifest.Version, Project: manifest.Name, Type: manifest.Type, Mode: dockerMode(manifest)}, nil
}

func runActiveDeployer(d deployer.Deployer) error {
	// 新版本准备成功后, 先停掉上一版本再启动新版本, 避免端口/容器名冲突
	if err := d.Deactivate(); err != nil {
		return err
	}
	if err := d.Activate(); err != nil {
		return err
	}
	return d.HealthCheck()
}

// cleanupOldReleases 保留当前版本与最近一个上一版本 (SYS-004), 其余全部清理
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
		if strings.HasPrefix(e.Name(), ".tmp-deploy-") || strings.HasPrefix(e.Name(), ".old-") {
			_ = os.RemoveAll(filepath.Join(releasesDir, e.Name()))
			continue
		}
		if e.Name() == st.CurrentVersion || (st.PreviousVersion != "" && e.Name() == st.PreviousVersion) {
			continue
		}
		log.Info(i18n.T("🗑 清理旧版本目录 ") + e.Name())
		os.RemoveAll(filepath.Join(releasesDir, e.Name()))
	}
}

// cleanupOldBundles 按保留策略清理多余的 bundle 缓存 (默认最多保留 3 个，支持 0~100)
func cleanupOldBundles(root string) {
	bundlesKeep := 3
	if cfg, err := config.ReadProjectConfig(root); err == nil && cfg != nil {
		bundlesKeep = cfg.BundlesKeep()
	}
	_, deleted, err := cleaner.PruneBundles(root, bundlesKeep)
	if err != nil {
		log.Warn(i18n.T("清理旧 Bundle 缓存出错: ") + err.Error())
	}
	for _, name := range deleted {
		log.Info(i18n.T("🗑 清理旧 Bundle 缓存 ") + name)
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
		return fmt.Errorf(i18n.T("移动目录失败，且跨分区复制失败: %w"), err)
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
		Elapsed: ui.FormatDuration(time.Since(startedAt)),
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
