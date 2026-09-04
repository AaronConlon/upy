// upy version: 打印版本号并探测自更新 (失败静默)
package commands

import (
	"fmt"

	"github.com/AaronConlon/upy/internal/github"
	"github.com/AaronConlon/upy/internal/log"
	"github.com/AaronConlon/upy/internal/semver"
	"github.com/AaronConlon/upy/internal/version"
)

// VersionCmd 打印版本并检查更新
func VersionCmd() error {
	fmt.Println("upy " + version.String())

	tag := github.PeekLatestTag(selfRepo)
	if tag == "" {
		return nil
	}
	if semver.Newer(tag, version.Value) {
		log.Info("发现新版本: " + tag + "，可执行 upy update 升级")
	} else {
		log.Info("已是最新版本。")
	}
	return nil
}
