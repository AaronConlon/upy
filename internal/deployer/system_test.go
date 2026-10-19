package deployer

import (
	"fmt"
	"os"
	userpkg "os/user"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/AaronConlon/upy/internal/cmdrun"
	"github.com/AaronConlon/upy/internal/config"
)

func stubSystemctl(t *testing.T) *[]cmdrun.Call {
	t.Helper()
	var mu sync.Mutex
	calls := []cmdrun.Call{}
	cmdrun.Run = func(cmd string, args []string, opts cmdrun.Options) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, cmdrun.Call{Cmd: cmd, Args: args})
		return nil
	}
	cmdrun.Output = func(cmd string, args []string, opts cmdrun.Options) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, cmdrun.Call{Cmd: cmd, Args: args})
		return "1234", nil
	}
	t.Cleanup(func() {
		cmdrun.Run = cmdrun.RealRun
		cmdrun.Output = cmdrun.RealOutput
	})
	return &calls
}

func newSystemCtx(t *testing.T, projectName, version string, command []string) (*SystemDeployer, string, string) {
	t.Helper()
	root := t.TempDir()
	sysDir := t.TempDir()
	t.Setenv("UPY_SYSTEMD_DIR", sysDir)
	curUser, err := userpkg.Current()
	if err == nil && curUser != nil {
		t.Setenv("UPY_SERVICE_USER", curUser.Username)
	}

	releaseDir := filepath.Join(root, "releases", version)
	os.MkdirAll(filepath.Join(releaseDir, "bin"), 0o755)

	// 创建可执行文件
	binaryPath := filepath.Join(releaseDir, "bin", "my-api")
	os.WriteFile(binaryPath, []byte("#!/bin/sh\necho ok\n"), 0o755)

	manifest := &config.Manifest{
		Name:    projectName,
		Version: version,
		Type:    "system",
		System: &config.ManifestSystem{
			Command: command,
		},
	}
	ctx := &DeployContext{
		Root:        root,
		ReleaseDir:  releaseDir,
		Version:     version,
		Manifest:    manifest,
		ProjectName: projectName,
	}

	deployer := NewSystem(ctx)
	return deployer, root, sysDir
}

func TestSystemDeployer_Prepare(t *testing.T) {
	deployer, root, _ := newSystemCtx(t, "my-api", "1.0.0", []string{"./bin/my-api", "--port", "8080"})

	if err := deployer.Prepare(); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	// 验证 shared/data 目录已创建
	dataDir := filepath.Join(root, "shared", "data")
	if info, err := os.Stat(dataDir); err != nil || !info.IsDir() {
		t.Errorf("Expected shared/data directory to be created, got err: %v", err)
	}
}

func TestSystemDeployer_MissingUser(t *testing.T) {
	deployer, _, _ := newSystemCtx(t, "my-api", "1.0.0", []string{"./bin/my-api"})
	deployer.user = "non-existent-user-xyz-999"
	if err := deployer.Prepare(); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("Expected error for missing user, got: %v", err)
	}
}

func TestSystemDeployer_Prepare_MissingBinary(t *testing.T) {
	deployer, _, _ := newSystemCtx(t, "my-api", "1.0.0", []string{"./bin/not-exist"})

	if err := deployer.Prepare(); err == nil {
		t.Error("Expected Prepare to fail on non-existent binary, but it succeeded")
	}
}

func TestSystemDeployer_OwnershipProtection(t *testing.T) {
	deployer, root, sysDir := newSystemCtx(t, "my-api", "1.0.0", []string{"./bin/my-api"})

	unitFile := filepath.Join(sysDir, "upy-my-api.service")
	// 场景 1: 非 upy 纳管的既有服务文件
	os.WriteFile(unitFile, []byte("[Unit]\nDescription=foreign service\n"), 0o644)
	if err := deployer.Prepare(); err == nil || !strings.Contains(err.Error(), "非 upy 纳管") {
		t.Fatalf("Expected error for non-upy unit, got: %v", err)
	}

	// 场景 2: 属于另一路径的同名项目
	otherUnit := fmt.Sprintf("# Managed by upy - DO NOT EDIT MANUALLY\n# X-Upy-Root: /other/path\n[Unit]\nDescription=my-api managed by upy\n")
	os.WriteFile(unitFile, []byte(otherUnit), 0o644)
	if err := deployer.Prepare(); err == nil || !strings.Contains(err.Error(), "拒绝冲突覆盖") {
		t.Fatalf("Expected error for cross-project unit conflict, got: %v", err)
	}

	// 场景 3: 属于当前根目录的服务文件
	curUnit := fmt.Sprintf("# Managed by upy - DO NOT EDIT MANUALLY\n# X-Upy-Root: %s\n[Unit]\nDescription=my-api managed by upy\n", root)
	os.WriteFile(unitFile, []byte(curUnit), 0o644)
	if err := deployer.Prepare(); err != nil {
		t.Fatalf("Expected Prepare to succeed for owned unit, got: %v", err)
	}
}

func TestSystemDeployer_Activate(t *testing.T) {
	calls := stubSystemctl(t)
	deployer, root, sysDir := newSystemCtx(t, "my-api", "1.1.0", []string{"./bin/my-api", "run", "--rate=%50", "$FOO"})

	if err := deployer.Activate(); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	unitFile := filepath.Join(sysDir, "upy-my-api.service")
	content, err := os.ReadFile(unitFile)
	if err != nil {
		t.Fatalf("Failed to read generated unit file: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "# X-Upy-Root: "+root) {
		t.Errorf("Unit missing X-Upy-Root: %s", s)
	}
	if !strings.Contains(s, "Description=my-api managed by upy") {
		t.Errorf("Unit missing description: %s", s)
	}
	// 验证参数中的 % 转义为 %%，参数中的 $ 转义为 $$
	if !strings.Contains(s, "--rate=%%50") {
		t.Errorf("Unit missing escaped %% argument: %s", s)
	}
	if !strings.Contains(s, "$$FOO") {
		t.Errorf("Unit missing escaped $$ argument: %s", s)
	}
	if !strings.Contains(s, "Restart=on-failure") {
		t.Errorf("Unit missing Restart=on-failure: %s", s)
	}
	if !strings.Contains(s, "KillSignal=SIGTERM") {
		t.Errorf("Unit missing KillSignal=SIGTERM: %s", s)
	}

	// 验证 systemctl 调用
	foundReload := false
	foundStart := false
	for _, c := range *calls {
		if c.Cmd == "systemctl" && len(c.Args) > 0 {
			if c.Args[0] == "daemon-reload" {
				foundReload = true
			}
			if c.Args[0] == "start" && c.Args[1] == "upy-my-api.service" {
				foundStart = true
			}
		}
	}
	if !foundReload || !foundStart {
		t.Errorf("Expected daemon-reload and start calls, got: %+v", *calls)
	}
}

func TestSystemDeployer_DeactivateAndRollback(t *testing.T) {
	calls := stubSystemctl(t)
	deployer, root, sysDir := newSystemCtx(t, "my-api", "1.1.0", []string{"./bin/my-api"})
	deployer.ctx.PreviousVersion = "1.0.0"

	// 先在磁盘放置旧版本的 unit
	unitFile := filepath.Join(sysDir, "upy-my-api.service")
	oldContent := []byte(fmt.Sprintf("# Managed by upy - DO NOT EDIT MANUALLY\n# X-Upy-Root: %s\n[Unit]\nDescription=my-api managed by upy\n", root))
	os.WriteFile(unitFile, oldContent, 0o644)

	// 模拟 deactivate
	if err := deployer.Deactivate(); err != nil {
		t.Fatalf("Deactivate failed: %v", err)
	}
	if string(deployer.backupUnit) != string(oldContent) {
		t.Fatalf("Expected backupUnit to match old content, got: %s", string(deployer.backupUnit))
	}
	// 验证磁盘上的 .deploy.unit.bak 已生成
	bakFile := filepath.Join(root, ".deploy.unit.bak")
	bakData, err := os.ReadFile(bakFile)
	if err != nil || string(bakData) != string(oldContent) {
		t.Fatalf("Expected .deploy.unit.bak to match old content")
	}

	// 模拟 activate 生成新版本
	if err := deployer.Activate(); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	// 触发回滚
	if err := deployer.Rollback(); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// 检查 unit 是否被还原为 oldContent
	restored, err := os.ReadFile(unitFile)
	if err != nil {
		t.Fatalf("Failed to read restored unit file: %v", err)
	}
	if string(restored) != string(oldContent) {
		t.Errorf("Rollback failed to restore old unit. Expected %s, got %s", oldContent, restored)
	}

	// 验证回滚是否调用了 stop 和 reset-failed 确保新服务彻底关闭
	foundStop := false
	foundResetFailed := false
	for _, c := range *calls {
		if c.Cmd == "systemctl" && len(c.Args) > 0 {
			if c.Args[0] == "stop" && c.Args[1] == "upy-my-api.service" {
				foundStop = true
			}
			if c.Args[0] == "reset-failed" && c.Args[1] == "upy-my-api.service" {
				foundResetFailed = true
			}
		}
	}
	if !foundStop || !foundResetFailed {
		t.Errorf("Expected stop and reset-failed calls during rollback, got: %+v", *calls)
	}
}

func TestSystemDeployer_FirstDeployRollback(t *testing.T) {
	calls := stubSystemctl(t)
	deployer, _, sysDir := newSystemCtx(t, "my-api", "1.0.0", []string{"./bin/my-api"})

	// 首次部署先激活
	if err := deployer.Activate(); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	unitFile := filepath.Join(sysDir, "upy-my-api.service")
	if _, err := os.Stat(unitFile); err != nil {
		t.Fatalf("Expected unit file to exist after Activate")
	}

	if err := deployer.Rollback(); err != nil {
		t.Fatalf("Rollback on first deploy failed: %v", err)
	}

	// 验证 unit 文件已被清理
	if _, err := os.Stat(unitFile); !os.IsNotExist(err) {
		t.Errorf("Expected unit file to be deleted on first deploy rollback, but it exists")
	}

	foundDisable := false
	for _, c := range *calls {
		if c.Cmd == "systemctl" && len(c.Args) > 0 && c.Args[0] == "disable" {
			foundDisable = true
			break
		}
	}
	if !foundDisable {
		t.Errorf("Expected disable call on first deploy rollback, got: %+v", *calls)
	}
}

func TestSystemDeployer_ServerConfigOverrides(t *testing.T) {
	root := t.TempDir()
	sysDir := t.TempDir()
	t.Setenv("UPY_SYSTEMD_DIR", sysDir)

	deployYaml := `name: my-api
release:
  provider: github
  repository: owner/repo
service:
  user: custom-user
  group: custom-group
  unit: custom-api.service
`
	os.WriteFile(filepath.Join(root, "deploy.yaml"), []byte(deployYaml), 0o644)

	releaseDir := filepath.Join(root, "releases", "1.0.0")
	os.MkdirAll(filepath.Join(releaseDir, "bin"), 0o755)
	os.WriteFile(filepath.Join(releaseDir, "bin", "my-api"), []byte("#!/bin/sh\n"), 0o755)

	manifest := &config.Manifest{
		Name:    "my-api",
		Version: "1.0.0",
		Type:    "system",
		System: &config.ManifestSystem{
			Command: []string{"./bin/my-api"},
		},
	}
	ctx := &DeployContext{
		Root:        root,
		ReleaseDir:  releaseDir,
		Version:     "1.0.0",
		Manifest:    manifest,
		ProjectName: "my-api",
	}

	deployer := NewSystem(ctx)
	if deployer.UnitName() != "custom-api.service" {
		t.Errorf("Expected UnitName to be custom-api.service, got %s", deployer.UnitName())
	}
	if deployer.user != "custom-user" || deployer.group != "custom-group" {
		t.Errorf("Expected custom-user/custom-group, got %s/%s", deployer.user, deployer.group)
	}
}
