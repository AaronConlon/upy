package log

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLogSetOutputAndRedact(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	RegisterSecret("secret-token-12345")

	Info("testing message with secret-token-12345 inside")
	out := buf.String()
	if strings.Contains(out, "secret-token-12345") {
		t.Fatalf("敏感值未被脱敏: %s", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("未包含脱敏星号: %s", out)
	}
}

func TestLogConcurrency(t *testing.T) {
	var buf bytes.Buffer
	SetOutput(&buf)
	defer ResetOutput()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			RegisterSecret("token")
			Step("step")
			Ok("ok")
			Info("info")
		}(i)
	}
	wg.Wait()
}

func TestStripAnsi(t *testing.T) {
	colored := "\x1b[32m✓\x1b[0m 部署成功: \x1b[96mtest\x1b[0m"
	got := StripAnsi(colored)
	if got != "✓ 部署成功: test" {
		t.Fatalf("StripAnsi 不符合预期: %q", got)
	}
}

func TestEnableFileLog(t *testing.T) {
	tmpDir := t.TempDir()
	closer := EnableFileLog(tmpDir, []string{"upy", "deploy", "demo-app"})
	Step("执行第一步")
	Ok("执行完成")
	closer()

	logContent, err := os.ReadFile(filepath.Join(tmpDir, "upy.log"))
	if err != nil {
		t.Fatalf("读取 upy.log 失败: %v", err)
	}
	content := string(logContent)
	if !strings.Contains(content, ">>> 启动: upy deploy demo-app") {
		t.Fatalf("缺少启动标记: %s", content)
	}
	if !strings.Contains(content, "-> 执行第一步") {
		t.Fatalf("缺少步骤内容: %s", content)
	}
	if !strings.Contains(content, "✓ 执行完成") {
		t.Fatalf("缺少完成内容: %s", content)
	}
	if !strings.Contains(content, "<<< 退出") {
		t.Fatalf("缺少退出标记: %s", content)
	}
}
