package commands

import (
	"path/filepath"
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
