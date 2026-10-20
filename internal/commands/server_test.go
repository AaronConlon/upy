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
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("enrollment rotated secret")
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
	var data struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(first, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), data.Secret) || strings.Contains(string(logged), data.Secret) {
		t.Fatal("plaintext secret leaked")
	}
}
