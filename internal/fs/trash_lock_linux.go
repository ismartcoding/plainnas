//go:build linux

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type diskLock struct {
	f *os.File
}

func lockDiskTrashRoot(diskMount string) (*diskLock, error) {
	if diskMount == "" {
		return nil, fmt.Errorf("empty disk mount")
	}
	trashRoot := filepath.Join(diskMount, ".nas-trash")
	if err := os.MkdirAll(trashRoot, 0o755); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(trashRoot, ".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &diskLock{f: f}, nil
}

func (l *diskLock) Unlock() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}
