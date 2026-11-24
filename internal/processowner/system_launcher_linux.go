package processowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// SystemLauncher pins the host's root-owned util-linux launcher. The fixed path
// is not selected by PATH, a project, a model, or a renderer. It grants no task
// authority. Callers retain the resulting pin for their supervisor lifetime.
func SystemLauncher(ctx context.Context) (Launcher, error) {
	return pinSystemExecutable(ctx, "/usr/bin/unshare")
}

func systemShell(ctx context.Context) (Launcher, error) {
	path, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		return Launcher{}, err
	}
	return pinSystemExecutable(ctx, path)
}

func pinSystemExecutable(ctx context.Context, path string) (Launcher, error) {
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return Launcher{}, err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 || info.Mode().Perm()&0022 != 0 || info.Mode()&os.ModeSymlink != 0 {
			return Launcher{}, errors.New("system namespace launcher ancestry must be root-owned and non-writable")
		}
		if current == path {
			if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || info.Size() < 1 || info.Size() > 64<<20 {
				return Launcher{}, errors.New("system namespace launcher is not a bounded ordinary executable")
			}
		} else if !info.IsDir() {
			return Launcher{}, errors.New("system namespace launcher parent is not a directory")
		}
		if current == "/" {
			break
		}
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return Launcher{}, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Launcher{}, errors.New("system namespace launcher changed before open")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || info.Mode().Perm()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		return Launcher{}, errors.New("system namespace launcher ownership changed")
	}
	hash := sha256.New()
	buffer := make([]byte, 128<<10)
	var count int64
	for {
		if err := ctx.Err(); err != nil {
			return Launcher{}, err
		}
		n, err := file.Read(buffer)
		count += int64(n)
		if count > 64<<20 {
			return Launcher{}, errors.New("system namespace launcher exceeds limit")
		}
		hash.Write(buffer[:n])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Launcher{}, err
		}
	}
	if count != info.Size() {
		return Launcher{}, errors.New("system namespace launcher changed while hashing")
	}
	return Launcher{Path: path, SHA256: "sha256:" + hex.EncodeToString(hash.Sum(nil))}, nil
}
