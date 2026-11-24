//go:build !windows

package main

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// A regular key file replaced by a FIFO must not block before fstat can reject
// its new identity. O_NONBLOCK does not change regular-file reads.
func openWorkspaceKeyHandle(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}

func readWorkspaceSecretFile(ctx context.Context, file *os.File, p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		fds := []unix.PollFd{{Fd: int32(file.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 100)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			continue
		}
		n, err = unix.Read(int(file.Fd()), p)
		if n < 0 {
			n = 0
		}
		if errors.Is(err, unix.EINTR) || errors.Is(err, unix.EAGAIN) {
			continue
		}
		if n == 0 && err == nil {
			return 0, io.EOF
		}
		return n, err
	}
}

func workspaceTerminalInput(ctx context.Context, file *os.File) func() ([]byte, error) {
	return func() ([]byte, error) {
		var b [1]byte
		n, err := readWorkspaceSecretFile(ctx, file, b[:])
		return b[:n], err
	}
}
