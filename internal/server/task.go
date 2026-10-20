package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/AaronConlon/upy/internal/config"
)

var (
	ErrProjectBusy = errors.New("该项目当前已有正在执行的部署任务")
	ErrTaskNotFound = errors.New("任务未找到")
)

type TaskStatus string

const (
	StatusQueued  TaskStatus = "queued"
	StatusRunning TaskStatus = "running"
	StatusSuccess TaskStatus = "success"
	StatusFailed  TaskStatus = "failed"
	StatusTimeout TaskStatus = "timeout"
)

type Task struct {
	ID         string     `json:"id"`
	Project    string     `json:"project"`
	Root       string     `json:"root"`
	Target     string     `json:"target"`
	Force      bool       `json:"force"`
	Status     TaskStatus `json:"status"`
	ExitCode   int        `json:"exit_code"`
	LogFile    string     `json:"log_file"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Duration   string     `json:"duration,omitempty"`
	Error      string     `json:"error,omitempty"`
}

type TaskManager struct {
	mu             sync.RWMutex
	tasks          map[string]*Task
	activeProjects map[string]string // projectName -> taskID
}

func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks:          make(map[string]*Task),
		activeProjects: make(map[string]string),
	}
}

func (tm *TaskManager) CreateTask(project, root, target string, force bool, logFile string) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if activeID, busy := tm.activeProjects[project]; busy {
		return nil, fmt.Errorf("%w (活跃任务 ID: %s)", ErrProjectBusy, activeID)
	}

	taskID := fmt.Sprintf("%s-%s-%s", project, time.Now().Format("20060102-150405"), randomSuffix(4))
	if logFile == "" {
		logFile = filepath.Join(config.UserDataDir(), "logs", project, taskID+".log")
	}
	t := &Task{
		ID:        taskID,
		Project:   project,
		Root:      root,
		Target:    target,
		Force:     force,
		Status:    StatusRunning,
		LogFile:   logFile,
		StartedAt: time.Now(),
	}

	tm.tasks[taskID] = t
	tm.activeProjects[project] = taskID
	return t, nil
}

func (tm *TaskManager) FinishTask(id string, exitCode int, err error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.tasks[id]
	if !ok {
		return
	}

	now := time.Now()
	t.FinishedAt = &now
	t.Duration = now.Sub(t.StartedAt).Round(time.Millisecond).String()
	t.ExitCode = exitCode

	if err != nil {
		t.Status = StatusFailed
		t.Error = err.Error()
	} else if exitCode != 0 {
		t.Status = StatusFailed
		t.Error = fmt.Sprintf("进程非零退出码 (%d)", exitCode)
	} else {
		t.Status = StatusSuccess
	}

	delete(tm.activeProjects, t.Project)
}

func (tm *TaskManager) Get(id string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	t, ok := tm.tasks[id]
	if !ok {
		return nil, false
	}
	// 返回浅拷贝避免外部读写竞争
	cp := *t
	return &cp, true
}

func (tm *TaskManager) GetActiveTask(project string) (*Task, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	taskID, ok := tm.activeProjects[project]
	if !ok {
		return nil, false
	}
	t, ok := tm.tasks[taskID]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

func (tm *TaskManager) List() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	res := make([]*Task, 0, len(tm.tasks))
	for _, t := range tm.tasks {
		cp := *t
		res = append(res, &cp)
	}
	return res
}

func randomSuffix(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%04x", time.Now().UnixNano()%0xffff)
	}
	return hex.EncodeToString(b)
}
