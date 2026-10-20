package i18n

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestLanguageAndArguments(t *testing.T) {
	defer SetLanguage("en")
	if Language() != "en" {
		t.Fatal("default locale must be English")
	}
	for _, tc := range []struct{ input, want string }{{"", "en"}, {"en-US", "en"}, {"zh", "zh-CN"}, {"zh-CN", "zh-CN"}, {"zh-Hans", "zh-CN"}} {
		got, err := Normalize(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("%q: %q %v", tc.input, got, err)
		}
	}
	if err := SetLanguage("fr"); err == nil {
		t.Fatal("unsupported locale accepted")
	}
	project := "中文项目"
	root := "/srv/项目"
	got := fmt.Sprintf(T("项目 %s 备份成功 (%s, 耗时 %s)"), project, root, "3s")
	if !strings.Contains(got, "Backup succeeded") || !strings.Contains(got, project) || !strings.Contains(got, root) {
		t.Fatalf("translated user input: %s", got)
	}
	sentinel := Error("项目已被锁定")
	if sentinel.Error() != "Project is locked" {
		t.Fatal(sentinel)
	}
	if err := SetLanguage("zh-CN"); err != nil {
		t.Fatal(err)
	}
	if T("当前版本") != "当前版本" || sentinel.Error() != "项目已被锁定" {
		t.Fatal("Chinese locale not applied")
	}
	if !errors.Is(fmt.Errorf("wrapped: %w", sentinel), sentinel) {
		t.Fatal("sentinel identity lost")
	}
}
func TestCatalogFormatArguments(t *testing.T) {
	// Translate format templates, never their arguments; retain verb types, ordering, and wrapping.
	directives := regexp.MustCompile(`%[-+# 0]*(?:[0-9]+)?(?:\.[0-9]+)?[a-zA-Z%]`)
	for source, translated := range english {
		if strings.TrimSpace(translated) == "" {
			t.Errorf("empty translation: %q", source)
		}
		if !reflect.DeepEqual(directives.FindAllString(source, -1), directives.FindAllString(translated, -1)) {
			t.Errorf("format verbs differ: %q -> %q", source, translated)
		}
	}
}
