//go:build !windows

package platform

import "syscall"

// SetDefaultUmask 将当前进程的 umask 设置为 0022，避免外部严格 umask (如 0027/0077)
// 导致新建目录和文件缺少其他用户（如 Nginx / www-data）的读取与目录执行权限。
func SetDefaultUmask() {
	syscall.Umask(0o022)
}
