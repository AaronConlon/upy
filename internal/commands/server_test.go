package commands

import (
	"bytes"
	"encoding/json"
	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerStatusStopped(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(tmpDir, "config.yaml"))

	err := Server(ServerArgs{
		Action: "status",
	})
	if err != nil {
		t.Fatalf("Server status stopped should not fail: %v", err)
	}
}

func TestServerStopWhenNotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(tmpDir, "config.yaml"))

	err := Server(ServerArgs{
		Action: "stop",
	})
	if err == nil {
		t.Fatalf("Server stop when not running should error")
	}
}

func TestServerUnknownAction(t *testing.T) {
	err := Server(ServerArgs{
		Action: "foo",
	})
	if err == nil {
		t.Fatalf("Unknown action should error")
	}
}

func TestServerTerminalEnrollment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("UPY_CONFIG", filepath.Join(root, "config.yaml"))
	var out bytes.Buffer
	closeLog := log.EnableFileLog(filepath.Join(root, "logs"), []string{"upy", "server", "auth"})
	if err := renderServerAuth(config.ServerConfig{}, &out); err != nil {
		t.Fatal(err)
	}
	closeLog()
	first, err := os.ReadFile(filepath.Join(root, "totp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := renderServerAuth(config.ServerConfig{}, io.Discard); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(root, "totp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d1, d2 struct {
		Secret string `json:"secret"`
		Views  int    `json:"views"`
	}
	if err := json.Unmarshal(first, &d1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second, &d2); err != nil {
		t.Fatal(err)
	}
	if d1.Secret == "" || d1.Secret != d2.Secret {
		t.Fatal("enrollment rotated secret")
	}
	if d1.Views != 1 || d2.Views != 2 {
		t.Fatalf("expected views to increment from 1 to 2, got d1=%d, d2=%d", d1.Views, d2.Views)
	}
	if !strings.Contains(out.String(), "剩余展示次数：4 / 5 次") {
		t.Fatalf("expected remaining views in output, got: %s", out.String())
	}

	// 剩余查看次数耗尽后禁止渲染
	for i := 3; i <= 5; i++ {
		if err := renderServerAuth(config.ServerConfig{}, io.Discard); err != nil {
			t.Fatalf("call %d should succeed: %v", i, err)
		}
	}
	if err := renderServerAuth(config.ServerConfig{}, io.Discard); err == nil {
		t.Fatal("call 6 should have exceeded view quota, but got nil error")
	} else if !strings.Contains(err.Error(), "上限") {
		t.Fatalf("expected quota exceeded error, got: %v", err)
	}
	if !strings.Contains(out.String(), "█") {
		t.Fatal("terminal QR missing")
	}
	logged, err := os.ReadFile(filepath.Join(root, "logs", "upy.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logged), "█") || strings.Contains(string(logged), "otpauth://") {
		t.Fatal("QR leaked into runtime log")
	}
	if strings.Contains(out.String(), d1.Secret) || strings.Contains(string(logged), d1.Secret) {
		t.Fatal("plaintext secret leaked")
	}
}
