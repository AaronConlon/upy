package deployer

import "github.com/AaronConlon/upy/internal/i18n"

// Legacy message assertions exercise the supported Chinese locale explicitly.
func init() { _ = i18n.SetLanguage("zh-CN") }
