package execution

import (
	"errors"
	"os"
	"runtime"

	"golang.org/x/sys/windows"
)

func removeCopyEntry(root *os.Root, name string, expected os.FileInfo) error {
	file, err := openWorkingCopyWindows(root, name, false, true)
	if err != nil {
		return err
	}
	return errors.Join(disposeCopyHandle(file, expected), file.Close())
}

func disposeCopyHandle(file *os.File, expected os.FileInfo) error {
	actual, err := file.Stat()
	if err != nil {
		return err
	}
	if actual.Mode().Type() != expected.Mode().Type() || !os.SameFile(actual, expected) || actual.Mode().IsRegular() && (actual.Size() != expected.Size() || !actual.ModTime().Equal(expected.ModTime())) {
		return ErrRevision
	}
	deleteFile := byte(1)
	err = windows.SetFileInformationByHandle(windows.Handle(file.Fd()), windows.FileDispositionInfo, &deleteFile, 1)
	runtime.KeepAlive(file)
	return err
}
