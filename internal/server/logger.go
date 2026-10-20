package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/AaronConlon/upy/internal/config"
	"github.com/AaronConlon/upy/internal/log"
)

// TaskLogger 负责单个部署任务的日志落盘与敏感信息脱敏
type TaskLogger struct {
	mu   sync.Mutex
	file *os.File
	path string
	tee  io.Writer
}

// NewTaskLogger 创建并初始化一个任务日志记录器
func NewTaskLogger(project, taskID string, extraWriter io.Writer) (*TaskLogger, error) {
	logDir := filepath.Join(config.UserDataDir(), "logs", project)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建任务日志目录失败: %w", err)
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("%s.log", taskID))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("创建任务日志文件失败: %w", err)
	}

	return &TaskLogger{
		file: f,
		path: logPath,
		tee:  extraWriter,
	}, nil
}

// Write 实现 io.Writer，自动脱敏后写入日志文件与分流目标
func (l *TaskLogger) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 敏感信息脱敏
	redacted := log.Redact(string(p))
	data := []byte(redacted)

	if l.tee != nil {
		_, _ = l.tee.Write(data)
	}

	if l.file != nil {
		nw, werr := l.file.Write(data)
		if werr != nil {
			return nw, werr
		}
	}
	return len(p), nil
}

// Path 返回日志文件绝对路径
func (l *TaskLogger) Path() string {
	return l.path
}

// Close 关闭日志文件
func (l *TaskLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// PruneProjectLogs 保留每个项目最新的 maxKeep 份历史日志，多余的删除
func PruneProjectLogs(project string, maxKeep int) ([]string, error) {
	if maxKeep <= 0 {
		return nil, nil
	}
	logDir := filepath.Join(config.UserDataDir(), "logs", project)
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type logItem struct {
		name    string
		path    string
		modTime int64
	}

	var logs []logItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		logs = append(logs, logItem{
			name:    e.Name(),
			path:    filepath.Join(logDir, e.Name()),
			modTime: info.ModTime().UnixNano(),
		})
	}

	if len(logs) <= maxKeep {
		return nil, nil
	}

	// 按最后修改时间降序排序 (最新的排最前)
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].modTime > logs[j].modTime
	})

	var deleted []string
	for i := maxKeep; i < len(logs); i++ {
		if err := os.Remove(logs[i].path); err == nil {
			deleted = append(deleted, logs[i].name)
		}
	}

	return deleted, nil
}
