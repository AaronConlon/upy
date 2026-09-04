// upy update: 自更新. 内置仓库 AaronConlon/upy, 按 platform/arch 选资产
package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/AaronConlon/upy/internal/github"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/platform"
	"github.com/AaronConlon/upy/internal/semver"
	"github.com/AaronConlon/upy/internal/version"
)

// UpdateCmd 自更新: 支持最新版本或指定版本 (upy update [tag])
func UpdateCmd(target string, force bool) error {
	target = strings.TrimSpace(target)

	var release *github.GHRelease
	var err error

	if target == "" || strings.EqualFold(target, "latest") {
		release, err = github.GetLatest(selfRepo)
		if err != nil {
			return err
		}
		if !force && !semver.Newer(release.Tag, version.Value) {
			log.Info(fmt.Sprintf("当前已是最新版本 (%s)。可使用 --force 重新安装。", version.String()))
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
			return nil
		}
		log.Info(fmt.Sprintf("准备切换到版本: %s (当前版本: %s)", release.Tag, version.String()))
	}

	assetName := "upy-" + platform.CurrentOS() + "-" + platform.CurrentArch()
	asset, err := github.FindAsset(release, assetName, "", "")
	if err != nil {
		return err
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("无法定位 upy 二进制路径，自更新仅支持编译后的 upy 二进制")
	}

	tmp, err := os.CreateTemp("", "upy-update-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	os.Remove(tmpPath) // 需要干净路径, 让下载创建

	log.Step(fmt.Sprintf("⬇️  正在下载 %s（%s）...", assetName, humanSize(asset.Size)))
	if err := github.DownloadAsset(selfRepo, asset, tmpPath); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, self); err != nil {
		return fmt.Errorf("无法替换二进制文件 %s: %v (若安装在系统受限目录如 /usr/local/bin，请使用 sudo upy update)", self, err)
	}
	if err := os.Chmod(self, 0o755); err != nil {
		return err
	}

	log.Ok(fmt.Sprintf("已成功更新 %s 到 %s", self, release.Tag))
	log.Info("请重新运行 upy 以使用新版本。")
	return nil
}

