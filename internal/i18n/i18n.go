// Package i18n provides CLI messages without translating user data or protocol fields.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
)

//go:embed en.json
var catalogs embed.FS
var english map[string]string
var chinese atomic.Bool

func init() {
	data, _ := catalogs.ReadFile("en.json")
	if err := json.Unmarshal(data, &english); err != nil {
		panic(err)
	}
}

// Normalize accepts common aliases and keeps the persisted language canonical.
func Normalize(language string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "en", "en-us", "en-gb":
		return "en", nil
	case "zh", "zh-cn", "zh-hans":
		return "zh-CN", nil
	default:
		return "", fmt.Errorf("unsupported language %q; use en or zh-CN", language)
	}
}
func SetLanguage(language string) error {
	value, err := Normalize(language)
	if err != nil {
		return err
	}
	chinese.Store(value == "zh-CN")
	return nil
}
func Language() string {
	if chinese.Load() {
		return "zh-CN"
	}
	return "en"
}

// T translates a source message before formatting so paths and user input remain untouched.
func T(source string) string {
	if chinese.Load() {
		return source
	}
	if translated, ok := english[source]; ok {
		return translated
	}
	return source
}

// MessageError keeps sentinel identity stable while rendering in the active language.
type messageError struct{ source string }

func (e *messageError) Error() string { return T(e.source) }
func Error(source string) error       { return &messageError{source: source} }
