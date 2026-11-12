//go:build linux || darwin

package main

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func newAgentInputPoller() (func() ([]byte, error), error) {
	fd := int(os.Stdin.Fd())
	return func() ([]byte, error) {
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, 0)
		if err == unix.EINTR || n == 0 {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		var data [4096]byte
		n, err = unix.Read(fd, data[:])
		if err == unix.EINTR || err == unix.EAGAIN {
			return nil, nil
		}
		if err == nil && n == 0 {
			return nil, io.EOF
		}
		return data[:max(0, n)], err
	}, nil
}
