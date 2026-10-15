//go:build windows

package platform

// SetDefaultUmask 在 Windows 下为 no-op。
func SetDefaultUmask() {}
