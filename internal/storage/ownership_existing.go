package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AcquireExistingOwnership uses the ordinary workspace ownership lock without
// creating a directory or lock file. It is suitable for strictly read-only
// inspection. The opened handle must still identify the observed regular file.
func AcquireExistingOwnership(directory string) (*Ownership, error) {
	if !filepath.IsAbs(directory) || strings.HasPrefix(filepath.ToSlash(directory), "//") {
		return nil, fmt.Errorf("existing ownership requires an absolute local directory")
	}
	path := filepath.Join(directory, ".offgrid-owner.lock")
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("workspace ownership file must be regular")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*Ownership, error) { return nil, errors.Join(err, file.Close()) }
	opened, err := file.Stat()
	if err != nil {
		return fail(err)
	}
	current, err := os.Lstat(path)
	if err != nil {
		return fail(err)
	}
	if !current.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, current) {
		return fail(fmt.Errorf("workspace ownership identity changed"))
	}
	if err = lockFile(file); err != nil {
		return fail(err)
	}
	current, err = os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		unlockErr := unlockFile(file)
		if err == nil {
			err = fmt.Errorf("workspace ownership identity changed")
		}
		return fail(errors.Join(err, unlockErr))
	}
	return &Ownership{file: file}, nil
}
