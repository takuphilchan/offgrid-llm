//go:build linux || darwin

package verification

import (
	"os"
	"syscall"
)

// A regular file replaced with a FIFO between inspection and open must not
// block the verifier. Nonblocking mode has no effect on regular-file reads.
func openRead(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}

func singleLink(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink == 1
}
