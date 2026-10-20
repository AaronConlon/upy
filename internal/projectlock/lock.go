// projectlock: 项目级跨进程互斥排他锁 (基于 .deploy.lock 与操作系统 flock)
package projectlock

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AaronConlon/upy/internal/i18n"
)

// LockFileName 项目锁文件名
const LockFileName = ".deploy.lock"

var (
	// ErrLocked 表示项目已被其他进程或操作锁定
	ErrLocked = i18n.Error("项目已被锁定")

	mu        sync.Mutex
	heldLocks = make(map[string]*lockEntry)
)

type lockEntry struct {
	file     *os.File
	path     string
	action   string
	refCount int
}

// Lock 项目锁持有句柄
type Lock struct {
	path string
}

// Acquire 尝试获取项目排他锁。
// 若锁已被其他进程持有，立即返回包含持有信息的错误；
// 在同一进程内支持可重入获取（引用计数）。
func Acquire(root, action string) (*Lock, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	lockPath := filepath.Join(absRoot, LockFileName)

	mu.Lock()
	defer mu.Unlock()

	// 1. 同进程重入支持
	if entry, ok := heldLocks[lockPath]; ok {
		entry.refCount++
		return &Lock{path: lockPath}, nil
	}

	// 2. 打开或创建锁文件
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("无法创建或打开项目锁文件 %s: %w"), lockPath, err)
	}

	// 3. 尝试跨进程文件锁 (非阻塞)
	if err := lockFile(f.Fd()); err != nil {
		_ = f.Close()
		info := readLockInfo(lockPath)
		if info != "" {
			return nil, fmt.Errorf(i18n.T("项目正在进行 %s，无法获取项目锁，请稍后再试: %w"), info, ErrLocked)
		}
		return nil, fmt.Errorf(i18n.T("项目已被其他操作锁定，无法执行 %s，请稍后再试: %w"), action, ErrLocked)
	}

	// 4. 写入当前进程与动作信息
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	meta := fmt.Sprintf("action: %s\npid: %d\ntime: %s\n", action, os.Getpid(), time.Now().Format(time.RFC3339))
	_, _ = f.WriteString(meta)
	_ = f.Sync()

	heldLocks[lockPath] = &lockEntry{
		file:     f,
		path:     lockPath,
		action:   action,
		refCount: 1,
	}

	return &Lock{path: lockPath}, nil
}

// Release 释放项目锁
func (l *Lock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	entry, ok := heldLocks[l.path]
	if !ok {
		return nil
	}

	entry.refCount--
	if entry.refCount <= 0 {
		_ = unlockFile(entry.file.Fd())
		_ = entry.file.Close()
		delete(heldLocks, l.path)
	}
	return nil
}

// readLockInfo 读取已锁文件的上下文信息
func readLockInfo(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	var act, pid string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "action:") {
			act = strings.TrimSpace(strings.TrimPrefix(line, "action:"))
		} else if strings.HasPrefix(line, "pid:") {
			pid = strings.TrimSpace(strings.TrimPrefix(line, "pid:"))
		}
	}
	if act != "" && pid != "" {
		return fmt.Sprintf("%s（PID: %s）", act, pid)
	} else if act != "" {
		return act
	}
	return ""
}
