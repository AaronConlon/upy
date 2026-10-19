package config_test

import (
	"testing"

	"github.com/AaronConlon/upy/internal/config"
)

func TestParseManifest_System(t *testing.T) {
	yamlData := `
name: my-api
version: 1.0.0
type: system
target:
  os: linux
  arch: amd64
system:
  command: ["./bin/my-api", "--port", "8080"]
  healthcheck:
    url: http://127.0.0.1:8080/health
    timeout: 15s
`
	m, err := config.ParseManifest([]byte(yamlData))
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}
	if m.Name != "my-api" || m.Version != "1.0.0" || m.Type != "system" {
		t.Errorf("Unexpected manifest base fields: %+v", m)
	}
	if m.System == nil {
		t.Fatal("Expected m.System to be non-nil")
	}
	cmds, err := config.ParseCommand(m.System.Command)
	if err != nil {
		t.Fatalf("ParseCommand failed: %v", err)
	}
	if len(cmds) != 3 || cmds[0] != "./bin/my-api" || cmds[1] != "--port" || cmds[2] != "8080" {
		t.Errorf("Unexpected command: %v", cmds)
	}
	if m.Healthcheck == nil || m.Healthcheck.URL != "http://127.0.0.1:8080/health" || m.Healthcheck.Timeout != 15 {
		t.Errorf("Unexpected healthcheck: %+v", m.Healthcheck)
	}
}

func TestParseManifest_ServiceAlias(t *testing.T) {
	yamlData := `
name: my-service
version: 2.0.0
type: service
system:
  command: "./bin/svc -d"
`
	m, err := config.ParseManifest([]byte(yamlData))
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}
	if m.Type != "system" {
		t.Errorf("Expected canonical type 'system', got %s", m.Type)
	}
	cmds, err := config.ParseCommand(m.System.Command)
	if err != nil {
		t.Fatalf("ParseCommand failed: %v", err)
	}
	if len(cmds) != 2 || cmds[0] != "./bin/svc" || cmds[1] != "-d" {
		t.Errorf("Unexpected command: %v", cmds)
	}
}

func TestParseManifest_SystemValidation(t *testing.T) {
	// Missing system block
	missingSystem := `
name: bad-service
version: 1.0.0
type: system
`
	if _, err := config.ParseManifest([]byte(missingSystem)); err == nil {
		t.Error("Expected error when system block is missing, got nil")
	}

	// Conflicting static block
	conflict := `
name: bad-service
version: 1.0.0
type: system
system:
  command: ["./bin/app"]
static:
  directory: dist
`
	if _, err := config.ParseManifest([]byte(conflict)); err == nil {
		t.Error("Expected error when static block conflicts with system, got nil")
	}
}
