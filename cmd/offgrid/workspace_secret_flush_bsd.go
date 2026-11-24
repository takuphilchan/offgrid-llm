//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package main

import (
	"golang.org/x/sys/unix"
	"os"
)

func flushWorkspaceTerminalInput(file *os.File) error {
	return unix.IoctlSetPointerInt(int(file.Fd()), unix.TIOCFLUSH, 1)
}
