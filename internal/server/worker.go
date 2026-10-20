package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/AaronConlon/upy/internal/log"
)

type Worker struct {
	taskMgr       *TaskManager
	serverOut     io.Writer
	maxLogHistory int
	customBinary  string // 测试时可注入 mock 二进制
}

func NewWorker(taskMgr *TaskManager, serverOut io.Writer, maxLogHistory int) *Worker {
	if maxLogHistory <= 0 {
		maxLogHistory = 30
	}
	return &Worker{
		taskMgr:       taskMgr,
		serverOut:     serverOut,
		maxLogHistory: maxLogHistory,
	}
}

func (w *Worker) RunDeploy(ctx context.Context, task *Task) {
	logger, err := NewTaskLogger(task.Project, task.ID, w.serverOut)
	if err != nil {
		w.taskMgr.FinishTask(task.ID, 1, fmt.Errorf("创建日志分流器失败: %w", err))
		return
	}
	defer logger.Close()

	exe := w.customBinary
	if exe == "" {
		if curExe, err := os.Executable(); err == nil && curExe != "" {
			exe = curExe
		} else {
			exe = "upy"
		}
	}

	var args []string
	if task.Target != "" && task.Target != "latest" {
		args = []string{"release", task.Target, "--root", task.Root}
	} else {
		args = []string{"deploy", task.Project}
	}
	if task.Force {
		args = append(args, "--force")
	}

	header := fmt.Sprintf("[%s] 开始执行部署任务 (TaskID: %s, 项目: %s, 目标: %s)\n",
		time.Now().Format("2006-01-02 15:04:05"), task.ID, task.Project, task.Target)
	_, _ = logger.Write([]byte(header))

	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout = logger
	cmd.Stderr = logger
	// 注入终端彩色支持，保留 ANSI 颜色高亮
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	footer := fmt.Sprintf("\n[%s] 部署任务结束 (退出码: %d)\n",
		time.Now().Format("2006-01-02 15:04:05"), exitCode)
	_, _ = logger.Write([]byte(footer))

	w.taskMgr.FinishTask(task.ID, exitCode, runErr)

	// 任务结束后自动轮转清理历史日志
	if _, pruneErr := PruneProjectLogs(task.Project, w.maxLogHistory); pruneErr != nil && w.serverOut != nil {
		fmt.Fprintf(w.serverOut, "[upy-server] 轮转清理 %s 历史日志失败: %v\n", task.Project, pruneErr)
	}

	if w.serverOut != nil {
		statusStr := log.Green("成功")
		if exitCode != 0 || runErr != nil {
			statusStr = log.Red("失败")
		}
		fmt.Fprintf(w.serverOut, "[upy-server] 任务 %s 执行完毕: %s\n", task.ID, statusStr)
	}
}
