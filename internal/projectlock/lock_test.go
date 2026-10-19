package projectlock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireAndRelease(t *testing.T) {
	dir := t.TempDir()

	l, err := Acquire(dir, "测试操作")
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	lockPath := filepath.Join(dir, LockFileName)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("Expected lock file to exist at %s: %v", lockPath, err)
	}

	content, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("Failed to read lock file: %v", err)
	}
	if string(content) == "" {
		t.Fatal("Lock file content should not be empty")
	}

	if err := l.Release(); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Should be able to acquire again
	l2, err := Acquire(dir, "第二次操作")
	if err != nil {
		t.Fatalf("Acquire again failed: %v", err)
	}
	_ = l2.Release()
}

func TestReentrantLockInSameProcess(t *testing.T) {
	dir := t.TempDir()

	l1, err := Acquire(dir, "部署")
	if err != nil {
		t.Fatalf("l1 Acquire failed: %v", err)
	}

	// Same process acquire should succeed (reentrant)
	l2, err := Acquire(dir, "清理")
	if err != nil {
		t.Fatalf("l2 Acquire failed: %v", err)
	}

	// Release l2
	if err := l2.Release(); err != nil {
		t.Fatalf("l2 Release failed: %v", err)
	}

	// Lock should still be held by l1
	mu.Lock()
	entry, ok := heldLocks[l1.path]
	mu.Unlock()
	if !ok || entry.refCount != 1 {
		t.Fatalf("Expected entry to still exist with refCount 1, got ok=%v, entry=%+v", ok, entry)
	}

	// Release l1
	if err := l1.Release(); err != nil {
		t.Fatalf("l1 Release failed: %v", err)
	}

	mu.Lock()
	_, ok = heldLocks[l1.path]
	mu.Unlock()
	if ok {
		t.Fatal("Expected entry to be removed after all releases")
	}
}

func TestLockConflict(t *testing.T) {
	dir := t.TempDir()

	l1, err := Acquire(dir, "部署任务")
	if err != nil {
		t.Fatalf("l1 Acquire failed: %v", err)
	}
	defer l1.Release()

	// Try to open the file and lock directly with OS flock to simulate another process
	lockPath := filepath.Join(dir, LockFileName)
	f, err := os.OpenFile(lockPath, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer f.Close()

	if err := lockFile(f.Fd()); err == nil {
		t.Fatal("Expected lockFile to fail due to conflict, but it succeeded")
	}
}
