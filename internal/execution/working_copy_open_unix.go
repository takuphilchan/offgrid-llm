//go:build linux || darwin

package execution

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func qualifyWorkingCopyDirectory(_ *os.File) error { return nil }

func openWorkingCopyRooted(root *os.Root, name string, writable bool) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	if name == "." {
		return parent, nil
	}
	parts := strings.Split(name, "/")
	for i, part := range parts {
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		} else if writable {
			flags = flags&^unix.O_RDONLY | unix.O_RDWR
		}
		fd, openErr := unix.Openat(int(parent.Fd()), part, flags, 0)
		parent.Close()
		if openErr != nil {
			return nil, &os.PathError{Op: "open worker input", Path: name, Err: openErr}
		}
		parent = os.NewFile(uintptr(fd), name)
	}
	return parent, nil
}
