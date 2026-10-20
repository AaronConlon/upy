package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogCommand(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(tmpDir, "config.yaml"))

	logsDir := filepath.Join(tmpDir, "logs")
	_ = os.MkdirAll(logsDir, 0o755)

	// 1. 写测试 server.log
	serverLog := filepath.Join(logsDir, "server.log")
	_ = os.WriteFile(serverLog, []byte("[2026-10-20] line 1\n[2026-10-20] line 2\n[2026-10-20] line 3\n"), 0o644)

	// 2. 写测试 upy.log
	upyLog := filepath.Join(logsDir, "upy.log")
	_ = os.WriteFile(upyLog, []byte("[2026-10-20] cli step 1\n[2026-10-20] cli step 2\n"), 0o644)

	// 3. 写测试项目日志
	projDir := filepath.Join(logsDir, "demo-proj")
	_ = os.MkdirAll(projDir, 0o755)
	_ = os.WriteFile(filepath.Join(projDir, "task-001.log"), []byte("deploying demo-proj...\n"), 0o644)

	// 测试 list 模式
	if err := Log(LogArgs{List: true}); err != nil {
		t.Fatalf("Log --list 失败: %v", err)
	}

	// 测试查看 server 日志
	if err := Log(LogArgs{Server: true, Tail: 2}); err != nil {
		t.Fatalf("Log server 失败: %v", err)
	}

	// 测试查看 upy 日志
	if err := Log(LogArgs{Target: "upy", Tail: 1}); err != nil {
		t.Fatalf("Log upy 失败: %v", err)
	}

	// 测试查看项目日志
	if err := Log(LogArgs{Target: "demo-proj"}); err != nil {
		t.Fatalf("Log demo-proj 失败: %v", err)
	}

	// 测试清空日志
	if err := Log(LogArgs{Target: "upy", Clear: true}); err != nil {
		t.Fatalf("Log --clear 失败: %v", err)
	}
	content, _ := os.ReadFile(upyLog)
	if len(content) != 0 {
		t.Fatalf("清空后日志文件应为空，得到 %d 字节", len(content))
	}
}

func TestReadLastNLines(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")
	data := "line1\nline2\nline3\nline4\nline5\n"
	_ = os.WriteFile(tmpFile, []byte(data), 0o644)

	lines, offset, err := readLastNLines(tmpFile, 2)
	if err != nil {
		t.Fatalf("readLastNLines 失败: %v", err)
	}
	if len(lines) != 2 || lines[0] != "line4" || lines[1] != "line5" {
		t.Fatalf("行数或内容不符合预期: %v", lines)
	}
	if offset != int64(len(data)) {
		t.Fatalf("偏移量不匹配: %d vs %d", offset, len(data))
	}
}
