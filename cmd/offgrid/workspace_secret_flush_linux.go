package main

import (
	"golang.org/x/sys/unix"
	"os"
)

func flushWorkspaceTerminalInput(file *os.File) error {
	return unix.IoctlSetInt(int(file.Fd()), unix.TCFLSH, unix.TCIFLUSH)
}
