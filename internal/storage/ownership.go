package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrWorkspaceInUse = errors.New("workspace is already in use; stop the other OffGrid service before retrying")

// Ownership is a process-scoped OS lock, not a PID file. The operating system
// releases it after a crash. Never unlink the lock file: another process could
// otherwise lock a different inode while this process is still writing.
type Ownership struct {
	file *os.File
	once sync.Once
	err  error
}

func AcquireOwnership(directory string) (*Ownership, error) {
	if strings.TrimSpace(directory) == "" {
		return nil, fmt.Errorf("workspace directory is required")
	}
	abs, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(filepath.ToSlash(abs), "//") {
		return nil, fmt.Errorf("workspace must be on a local filesystem")
	}
	if err := os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(abs, ".offgrid-owner.lock")
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("workspace ownership file must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open workspace ownership file: %w", err)
	}
	if err = lockFile(file); err != nil {
		file.Close()
		return nil, err
	}
	return &Ownership{file: file}, nil
}

func (o *Ownership) Close() error {
	if o == nil {
		return nil
	}
	o.once.Do(func() { o.err = errors.Join(unlockFile(o.file), o.file.Close()) })
	return o.err
}
