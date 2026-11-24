//go:build !windows

package server

import (
	"os"

	"golang.org/x/sys/unix"
)

// Opening a replaced regular file must not block on a FIFO or follow its new
// final symlink before the caller can validate the retained file identity.
func openInferenceEndpointConfigFile(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
