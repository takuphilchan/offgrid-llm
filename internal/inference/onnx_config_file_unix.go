//go:build !windows

package inference

import (
	"golang.org/x/sys/unix"
	"os"
)

func openONNXRuntimeConfigFile(filename string) (*os.File, error) {
	fd, err := unix.Open(filename, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), filename), nil
}
