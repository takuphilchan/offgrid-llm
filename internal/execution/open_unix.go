//go:build !windows

package execution

import (
	"os"
	"syscall"
)

// Nonblocking open ensures a raced-in FIFO cannot hang the worker before the
// descriptor's regular-file check. This is not a device/sandbox isolation claim.
func readOpenFlags() int { return os.O_RDONLY | syscall.O_NONBLOCK }
