package processowner

import (
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
)

func readProc(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, (128<<10)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > 128<<10 {
		return nil, errors.New("provider proc metadata exceeds bounds")
	}
	return raw, nil
}

func pidfdExited(fd int) (bool, error) {
	if fd < 0 {
		return false, errors.New("provider pidfd is unavailable")
	}
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	var err error
	for range 3 {
		_, err = unix.Poll(fds, 0)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	if err != nil {
		return false, err
	}
	if fds[0].Revents&(unix.POLLNVAL|unix.POLLERR) != 0 {
		return false, errors.New("provider pidfd is invalid")
	}
	return fds[0].Revents&(unix.POLLIN|unix.POLLHUP) != 0, nil
}
