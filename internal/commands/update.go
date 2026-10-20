// upy update: 自更新. 内置仓库 AaronConlon/upy, 按 platform/arch 选资产
package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AaronConlon/upy/internal/checksum"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/github"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/platform"
	"github.com/AaronConlon/upy/internal/semver"
	"github.com/AaronConlon/upy/internal/ui"
	"github.com/AaronConlon/upy/internal/version"
)

// UpdateCmd 自更新: 支持最新版本或指定版本 (upy update [tag])
func UpdateCmd(target string, force bool) error {
	startedAt := time.Now()
	target = strings.TrimSpace(target)
	userCfg, err := config.LoadUserConfig()
	if err != nil {
		return err
	}
	proxyEnabled := userCfg.Update.ProxyEnabled
	if proxyEnabled {
		log.Info("upy 自更新下载加速已启用: https://ghfast.top")
	}
	download := func(asset *github.GHAsset, dest string) error {
		if proxyEnabled {
			return github.DownloadSelfUpdateAsset(asset, dest)
		}
		return github.DownloadAsset(selfRepo, asset, dest)
	}

	var release *github.GHRelease

	if target == "" || strings.EqualFold(target, "latest") {
		release, err = github.GetLatest(selfRepo)
		if err != nil {
			return err
		}
		if !force && !semver.Newer(release.Tag, version.Value) {
			log.Info(fmt.Sprintf("当前已是最新版本 (%s)。可使用 --force 重新安装。", version.String()))
			reportCompletionInstall()
			log.Info("⏱ 耗时: " + log.Green(log.Bold(ui.FormatDuration(time.Since(startedAt)))))
			return nil
		}
		log.Info(fmt.Sprintf("发现新版本: %s (当前版本: %s)", release.Tag, version.String()))
	} else {
		// 指定版本更新 / 回退
		tag := target
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		release, err = github.GetByTag(selfRepo, tag)
		if err != nil {
			// 若加 v 失败，尝试原始 tag
			if r2, err2 := github.GetByTag(selfRepo, target); err2 == nil {
				release = r2
				err = nil
			} else {
				return err
			}
		}
		if !force && release.Tag == version.Value {
			log.Info(fmt.Sprintf("当前已是目标版本 (%s)。可使用 --force 重新安装。", version.String()))
			reportCompletionInstall()
			log.Info("⏱ 耗时: " + log.Green(log.Bold(ui.FormatDuration(time.Since(startedAt)))))
			return nil
		}
		log.Info(fmt.Sprintf("准备切换到版本: %s (当前版本: %s)", release.Tag, version.String()))
	}

	assetName := "upy-" + platform.CurrentOS() + "-" + platform.CurrentArch()
	asset, err := github.FindAsset(release, assetName, "", "")
	if err != nil {
		return err
	}
	sumsAsset, err := github.FindAsset(release, "SHA256SUMS", "", "")
	if err != nil {
		return fmt.Errorf("版本 %s 缺少 SHA256SUMS，拒绝安装未校验的二进制: %w", release.Tag, err)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位 upy 二进制路径，自更新仅支持编译后的 upy 二进制")
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("无法解析 upy 二进制实际路径: %w", err)
	}

	selfDir := filepath.Dir(self)
	// 优先在目标二进制所在目录下创建临时文件：保证与目标在同一文件系统，避免跨设备 rename 报错 (EXDEV)
	tmp, err := os.CreateTemp(selfDir, ".upy-update-")
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("无法在目录 %s 创建临时更新文件: 权限不足 (若安装在系统受限目录如 /usr/local/bin，请使用 sudo upy update)", selfDir)
		}
		// 其他错误回退到系统临时目录
		tmp, err = os.CreateTemp("", "upy-update-")
		if err != nil {
			return err
		}
	}
	tmpPath := tmp.Name()
	tmp.Close()
	os.Remove(tmpPath) // 需要干净路径, 让下载创建
	sumsPath := tmpPath + ".SHA256SUMS"
	defer os.Remove(tmpPath)
	defer os.Remove(sumsPath)

	log.Step(fmt.Sprintf("⬇️  正在下载 %s（%s）...", assetName, humanSize(asset.Size)))
	if err := download(asset, tmpPath); err != nil {
		return err
	}
	log.Ok(fmt.Sprintf("%s 下载完成", assetName))
	log.Step("正在校验 SHA-256...")
	if err := download(sumsAsset, sumsPath); err != nil {
		return fmt.Errorf("无法下载 SHA256SUMS，拒绝安装未校验的二进制: %w", err)
	}
	if err := checksum.VerifySHA256SUMS(tmpPath, sumsPath, assetName); err != nil {
		return err
	}
	log.Ok("SHA-256 校验通过")
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}

	if err := replaceExecutable(tmpPath, self); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("无法替换二进制文件 %s: 权限不足 (若安装在系统受限目录如 /usr/local/bin，请使用 sudo upy update)", self)
		}
		return fmt.Errorf("无法替换二进制文件 %s: %w", self, err)
	}
	_ = os.Chmod(self, 0o755)

	log.Ok(fmt.Sprintf("已成功更新 %s 到 %s", self, release.Tag))
	// 已替换为新二进制，交由新版本写入补全规则，并在异常时回退到本进程同步
	completion := exec.Command(self, "__install-completion")
	completion.Stdout = os.Stdout
	completion.Stderr = os.Stderr
	if err := completion.Run(); err != nil {
		log.Warn("通过新版本注册补全异常，使用当前进程同步: " + err.Error())
		reportCompletionInstall()
	}
	log.Info("⏱ 耗时: " + log.Green(log.Bold(ui.FormatDuration(time.Since(startedAt)))))
	log.Info("请重新运行 upy 以使用新版本。")
	return nil
}

func reportCompletionInstall() {
	result, err := InstallShellCompletion()
	if err != nil {
		log.Warn("Tab 命令补全同步失败: " + err.Error())
		return
	}
	if !result.Supported {
		log.Info("检测到当前 shell: " + result.Shell + "（暂不支持自动安装 Tab 补全）")
		return
	}
	log.Info("检测到当前 shell: " + result.Shell)
	if len(result.ScriptPaths) > 1 {
		log.Ok("已同步 Tab 命令补全 (" + strings.Join(result.ScriptPaths, ", ") + ")（新开终端后生效）")
	} else {
		log.Ok("已同步 Tab 命令补全: " + result.ScriptPath + "（新开终端后生效）")
	}
}

// replaceExecutable 替换目标二进制文件，妥善处理同分区原子 rename、跨分区 EXDEV 以及 Linux 运行中二进制 ETXTBSY 保护
func replaceExecutable(src, dst string) error {
	// 1. 优先尝试原子 rename（同分区/文件系统下最安全且原子）
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// 2. 若 rename 失败（如跨设备/分区 EXDEV），尝试在目标目录创建临时文件并 rename
	dstDir := filepath.Dir(dst)
	dstTmp, err := os.CreateTemp(dstDir, ".upy-update-")
	if err == nil {
		dstTmpPath := dstTmp.Name()
		defer os.Remove(dstTmpPath)
		if err := copyExecutable(src, dstTmp); err == nil {
			_ = dstTmp.Close()
			_ = os.Chmod(dstTmpPath, 0o755)
			if err := os.Rename(dstTmpPath, dst); err == nil {
				return nil
			}
		} else {
			_ = dstTmp.Close()
		}
	}

	// 3. 跨分区终极回退（无法在目标目录建立同分区临时文件时）：
	// 在 Linux 下直接写入运行中的目标文件会触发 ETXTBSY (text file busy)，
	// 因此先将原目标文件重命名为 .old 腾出位置，再写入新文件
	backup := dst + ".old-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := os.Rename(dst, backup); err == nil {
		defer os.Remove(backup)
		if err := copyFileDirect(src, dst, 0o755); err != nil {
			_ = os.Rename(backup, dst) // 失败则尝试回滚
			return err
		}
		return nil
	}

	// 4. 若备份重命名也失败，尝试直接覆盖
	return copyFileDirect(src, dst, 0o755)
}

func copyExecutable(src string, dst *os.File) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(dst, in)
	return err
}

func copyFileDirect(src, dst string, mode os.FileMode) error {
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
	return os.Chmod(dst, mode)
}
