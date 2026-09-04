// upy update: 自更新. 内置仓库 AaronConlon/uply, 按 platform/arch 选资产
package commands

import (
	"fmt"
	"os"

	"github.com/AaronConlon/upy/internal/github"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/platform"
	"github.com/AaronConlon/upy/internal/semver"
	"github.com/AaronConlon/upy/internal/version"
)

// UpdateCmd 自更新
func UpdateCmd() error {
	latest, err := github.GetLatest(selfRepo)
	if err != nil {
		return err
	}
	if !semver.Newer(latest.Tag, version.Value) {
		log.Info("已是最新版本 (v" + version.Value + ")。")
		return nil
	}
	log.Info("发现新版本: " + latest.Tag)

	assetName := "upy-" + platform.CurrentOS() + "-" + platform.CurrentArch()
	asset, err := github.FindAsset(latest, assetName, "", "")
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

	log.Step("⬇️  正在下载 " + assetName + "（" + humanSize(asset.Size) + "）...")
	if err := github.DownloadAsset(selfRepo, asset, tmpPath); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, self); err != nil {
		return fmt.Errorf("无法替换二进制文件 %s: %v", self, err)
	}
	if err := os.Chmod(self, 0o755); err != nil {
		return err
	}

	log.Ok("已更新 " + self + " 到 " + latest.Tag)
	log.Info("请重新运行 upy 以使用新版本。")
	return nil
}
