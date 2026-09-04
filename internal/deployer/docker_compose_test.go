package deployer

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/AaronConlon/upy/internal/cmdrun"
	"github.com/AaronConlon/upy/internal/config"
)

// stubRun 替换 cmdrun.Run 记录调用序列; failOnSubstr 非空时, 参数命中即返回失败
func stubRun(t *testing.T, failOnSubstr string) *[]cmdrun.Call {
	t.Helper()
	var mu sync.Mutex
	calls := []cmdrun.Call{}
	cmdrun.Run = func(cmd string, args []string, opts cmdrun.Options) error {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, cmdrun.Call{Cmd: cmd, Args: args})
		if failOnSubstr != "" {
			for _, a := range args {
				if strings.Contains(a, failOnSubstr) {
					return errors.New("stub: 命令执行失败")
				}
			}
		}
		return nil
	}
	cmdrun.Output = func(cmd string, args []string, opts cmdrun.Options) (string, error) {
		return "codia\n", nil
	}
	t.Cleanup(func() {
		cmdrun.Run = cmdrun.RealRun
		cmdrun.Output = cmdrun.RealOutput
	})
	return &calls
}

// newComposeCtx 构造 compose 部署器测试上下文, 并在磁盘上创建两个版本的 compose 文件
func newComposeCtx(t *testing.T, prevVersion, curVersion string) (*DockerComposeDeployer, string) {
	t.Helper()
	root := t.TempDir()
	for _, v := range []string{prevVersion, curVersion} {
		if v == "" {
			continue
		}
		dir := filepath.Join(root, "releases", v)
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte("services: {}\n"), 0o644)
	}
	m := &config.Manifest{
		Name:    "codia",
		Version: curVersion,
		Type:    "docker",
		Docker:  &config.ManifestDocker{Mode: "compose", ComposeFile: "docker-compose.yml"},
	}
	ctx := &DeployContext{
		Root:            root,
		ReleaseDir:      filepath.Join(root, "releases", curVersion),
		Version:         curVersion,
		Manifest:        m,
		ProjectName:     m.Name,
		PreviousVersion: prevVersion,
	}
	return NewDockerCompose(ctx), root
}

func TestComposeDeactivateDownsPrevious(t *testing.T) {
	d, root := newComposeCtx(t, "0.3.16", "0.3.17")
	calls := stubRun(t, "")

	if err := d.Deactivate(); err != nil {
		t.Fatalf("Deactivate 不应失败: %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("期望恰好 1 次 docker 调用, 得到 %d", len(*calls))
	}
	call := (*calls)[0]
	prevFile := filepath.Join(root, "releases", "0.3.16", "docker-compose.yml")
	wanted := []string{"compose", "-f", prevFile, "down", "--remove-orphans"}
	if !reflect.DeepEqual(call.Args, wanted) {
		t.Fatalf("Deactivate 应对上一版本执行 down --remove-orphans\n期望: %v\n得到: %v", wanted, call.Args)
	}
}

func TestComposeArgsLoadsReleaseEnvBeforeSharedEnv(t *testing.T) {
	d, root := newComposeCtx(t, "", "0.3.17")
	releaseEnv := filepath.Join(root, "releases", "0.3.17", ".env")
	sharedEnv := filepath.Join(root, "shared", ".env")
	if err := os.MkdirAll(filepath.Dir(sharedEnv), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(releaseEnv, []byte("SURVEY_HOST_FONT_DIR=/release/fonts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sharedEnv, []byte("SURVEY_HOST_FONT_DIR=/shared/fonts\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	composeFile := filepath.Join(root, "releases", "0.3.17", "docker-compose.yml")
	wanted := []string{
		"-f", composeFile,
		"--env-file", releaseEnv,
		"--env-file", sharedEnv,
		"config",
	}
	if got := d.composeArgs("config"); !reflect.DeepEqual(got, wanted) {
		t.Fatalf("compose 环境文件顺序错误\n期望: %v\n得到: %v", wanted, got)
	}
}

func TestComposeDeactivateSkipsWithoutPrevious(t *testing.T) {
	d, _ := newComposeCtx(t, "", "0.3.17") // 首次部署
	calls := stubRun(t, "")

	if err := d.Deactivate(); err != nil {
		t.Fatalf("首次部署 Deactivate 应直接跳过: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("首次部署不应调用 docker, 得到 %d 次", len(*calls))
	}
}

func TestComposeDeactivateFailsWhenDownFails(t *testing.T) {
	d, _ := newComposeCtx(t, "0.3.16", "0.3.17")
	stubRun(t, "down")

	if err := d.Deactivate(); err == nil {
		t.Fatal("停旧失败时 Deactivate 应返回错误, 避免启动新栈造成冲突")
	}
}

func TestComposeDeactivateRecordsStoppedPrevious(t *testing.T) {
	d, _ := newComposeCtx(t, "0.3.16", "0.3.17")
	stubRun(t, "")
	cmdrun.Output = func(cmd string, args []string, opts cmdrun.Options) (string, error) {
		return "", nil // docker compose ps 没有 running service
	}

	if err := d.Deactivate(); err != nil {
		t.Fatalf("Deactivate 不应失败: %v", err)
	}
	if d.previousWasRunning {
		t.Fatal("无 running service 时应记录为停机")
	}
	if !d.previousStopped {
		t.Fatal("清理旧 compose 后应记录为已停止")
	}
}

func TestComposeRollbackStopsNewThenStartsPrevious(t *testing.T) {
	d, root := newComposeCtx(t, "0.3.16", "0.3.17")
	calls := stubRun(t, "")
	d.previousStopped = true
	d.previousWasRunning = true

	if err := d.Rollback(); err != nil {
		t.Fatalf("Rollback 不应失败: %v", err)
	}

	if len(*calls) != 2 {
		t.Fatalf("期望回滚 2 次 docker 调用 (down 新 + up 旧), 得到 %d", len(*calls))
	}

	curFile := filepath.Join(root, "releases", "0.3.17", "docker-compose.yml")
	prevFile := filepath.Join(root, "releases", "0.3.16", "docker-compose.yml")

	wantedDown := []string{"compose", "-f", curFile, "down", "--remove-orphans"}
	if !reflect.DeepEqual((*calls)[0].Args, wantedDown) {
		t.Fatalf("第一步应 down 新版本\n期望: %v\n得到: %v", wantedDown, (*calls)[0].Args)
	}
	wantedUp := []string{"compose", "-f", prevFile, "up", "-d"}
	if !reflect.DeepEqual((*calls)[1].Args, wantedUp) {
		t.Fatalf("第二步应 up -d 上一版本\n期望: %v\n得到: %v", wantedUp, (*calls)[1].Args)
	}
}

func TestComposeRollbackWithoutPrevious(t *testing.T) {
	d, _ := newComposeCtx(t, "", "0.3.17")
	calls := stubRun(t, "")

	if err := d.Rollback(); err != nil {
		t.Fatalf("无上一版本 Rollback 应直接返回: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("无上一版本不应调用 docker, 得到 %d 次", len(*calls))
	}
}

func TestComposeRollbackStillUpsPreviousWhenDownNewFails(t *testing.T) {
	d, root := newComposeCtx(t, "0.3.16", "0.3.17")
	calls := stubRun(t, "down") // down 新版本失败
	d.previousStopped = true
	d.previousWasRunning = true

	if err := d.Rollback(); err != nil {
		t.Fatalf("down 失败不应阻断回滚: %v", err)
	}
	prevFile := filepath.Join(root, "releases", "0.3.16", "docker-compose.yml")
	wantedUp := []string{"compose", "-f", prevFile, "up", "-d"}
	if len(*calls) != 2 || !reflect.DeepEqual((*calls)[1].Args, wantedUp) {
		t.Fatalf("down 新版本失败后仍应尝试 up -d 上一版本, calls=%v", *calls)
	}
}

func TestComposeRollbackKeepsStoppedPreviousStopped(t *testing.T) {
	d, root := newComposeCtx(t, "0.3.16", "0.3.17")
	calls := stubRun(t, "")
	d.previousStopped = true
	d.previousWasRunning = false

	if err := d.Rollback(); err != nil {
		t.Fatalf("Rollback 不应失败: %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("旧服务原本停机时仅应清理失败新版本，calls=%v", *calls)
	}
	curFile := filepath.Join(root, "releases", "0.3.17", "docker-compose.yml")
	wantedDown := []string{"compose", "-f", curFile, "down", "--remove-orphans"}
	if !reflect.DeepEqual((*calls)[0].Args, wantedDown) {
		t.Fatalf("应仅 down 失败新版本\n期望: %v\n得到: %v", wantedDown, (*calls)[0].Args)
	}
}

func TestComposeRollbackDoesNotTouchPreviousBeforeDeactivate(t *testing.T) {
	d, _ := newComposeCtx(t, "0.3.16", "0.3.17")
	calls := stubRun(t, "")

	if err := d.Rollback(); err != nil {
		t.Fatalf("Rollback 不应失败: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("旧服务未被停止时不应执行任何回滚命令，calls=%v", *calls)
	}
}
