package processowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func completeDigest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != 71 {
		return false
	}
	decoded, err := hex.DecodeString(value[7:])
	return err == nil && len(decoded) == 32
}
func openLauncher(ctx context.Context, pin Launcher) (*os.File, error) {
	if !filepath.IsAbs(pin.Path) || !completeDigest(pin.SHA256) {
		return nil, errors.New("namespace launcher requires an absolute path and SHA-256 pin")
	}
	before, err := os.Lstat(pin.Path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&0111 == 0 || before.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || before.Size() > 64<<20 {
		return nil, errors.New("namespace launcher must be a bounded ordinary executable")
	}
	return openLauncherFile(ctx, pin, before)
}

func openLauncherFile(ctx context.Context, pin Launcher, before os.FileInfo) (*os.File, error) {
	// A regular path may become a FIFO after Lstat. Nonblocking no-follow open
	// keeps that substitution from hanging startup before cancellation can run.
	fd, err := unix.Open(pin.Path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), pin.Path)
	accepted := false
	defer func() {
		if !accepted {
			file.Close()
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || info.Mode().Perm()&0022 != 0 || !os.SameFile(before, info) {
		return nil, errors.New("namespace launcher identity changed")
	}
	hash := sha256.New()
	buffer := make([]byte, 128<<10)
	var count int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := file.Read(buffer)
		count += int64(n)
		if count > 64<<20 {
			return nil, errors.New("namespace launcher exceeds byte limit")
		}
		hash.Write(buffer[:n])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if count != info.Size() || "sha256:"+hex.EncodeToString(hash.Sum(nil)) != pin.SHA256 {
		return nil, errors.New("namespace launcher pin changed")
	}
	accepted = true
	return file, nil
}
