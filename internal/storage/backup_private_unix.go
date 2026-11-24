//go:build linux || darwin

package storage

import (
	"golang.org/x/sys/unix"
	"os"
)

const backupPublicationMovesSource = false

// O_NONBLOCK prevents a regular input swapped for a FIFO from hanging before
// the opened descriptor can be checked. It does not change regular-file reads.
func openBackupInputFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}

func openBackupSourceFile(root *os.Root, path string) (*os.File, error) {
	return root.OpenFile(path, os.O_RDONLY|unix.O_NONBLOCK, 0)
}

func createPrivateBackupFile(parent string) (*os.File, error) {
	return os.CreateTemp(parent, ".offgrid-backup-*")
}

func createPrivateRestoreDirectory(parent string) (string, error) {
	return os.MkdirTemp(parent, ".offgrid-restore-*")
}

func publishBackupFile(source, target string) error { return os.Link(source, target) }

func syncBackupParent(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
