//go:build linux

package execution

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func pinReplacementDirectory(root *os.Root, name string) (*replacementDirectory, error) {
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	files := []*os.File{f}
	if name != "." {
		for _, part := range strings.Split(name, "/") {
			fd, err := unix.Openat(int(f.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			if err != nil {
				closeReplacementFiles(files)
				return nil, err
			}
			f = os.NewFile(uintptr(fd), part)
			files = append(files, f)
		}
	}
	return &replacementDirectory{file: f, close: func() error { return closeReplacementFiles(files) }}, nil
}

func replacePinned(target *replacementDirectory, name string, private *replacementDirectory, staged, backup string) (string, error) {
	// Exchange retains the exact displaced inode at the pre-journaled staged
	// name. Unsupported filesystems and cross-device stores never fall back to
	// ordinary rename, copy-overwrite or unlink.
	err := unix.Renameat2(int(private.file.Fd()), staged, int(target.file.Fd()), name, unix.RENAME_EXCHANGE)
	return staged, err
}

func syncReplacement(target, private *replacementDirectory) error {
	return errors.Join(target.file.Sync(), private.file.Sync())
}

func qualifyReplacementVolume(target, private *replacementDirectory) error {
	var a, b unix.Stat_t
	if err := unix.Fstat(int(target.file.Fd()), &a); err != nil {
		return err
	}
	if err := unix.Fstat(int(private.file.Fd()), &b); err != nil {
		return err
	}
	if a.Dev != b.Dev {
		return ErrUnsupported
	}
	return nil
}

func movePinnedExclusive(from *replacementDirectory, name string, to *replacementDirectory, destination string) error {
	return unix.Renameat2(int(from.file.Fd()), name, int(to.file.Fd()), destination, unix.RENAME_NOREPLACE)
}
