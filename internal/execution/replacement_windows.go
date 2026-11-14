//go:build windows

package execution

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var replaceFilePreserving = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")

func pinReplacementDirectory(root *os.Root, name string) (*replacementDirectory, error) {
	absolute, err := filepath.Abs(root.Name())
	if err != nil {
		return nil, err
	}
	full := filepath.Join(absolute, filepath.FromSlash(name))
	volume := filepath.VolumeName(full)
	// Remote/UNC filesystems require separate qualification. Keep the first
	// backend restricted to local drive paths and reject reparse points.
	if len(volume) != 2 || volume[1] != ':' {
		return nil, ErrUnsupported
	}
	current := volume + `\`
	parts := strings.Split(strings.TrimPrefix(full, current), `\`)
	files := []*os.File{}
	fail := func(err error) (*replacementDirectory, error) { closeReplacementFiles(files); return nil, err }
	for i := 0; i <= len(parts); i++ {
		if i > 0 {
			if parts[i-1] == "" {
				continue
			}
			current = filepath.Join(current, parts[i-1])
		}
		encoded, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return fail(err)
		}
		h, err := windows.CreateFile(encoded, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			return fail(err)
		}
		f := os.NewFile(uintptr(h), current)
		files = append(files, f)
		var info windows.ByHandleFileInformation
		if err = windows.GetFileInformationByHandle(h, &info); err != nil {
			return fail(err)
		}
		if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return fail(ErrUnsupported)
		}
		if strings.EqualFold(current, absolute) {
			expected, err := root.Stat(".")
			if err != nil {
				return fail(err)
			}
			actual, err := f.Stat()
			if err != nil {
				return fail(err)
			}
			if !os.SameFile(expected, actual) {
				return fail(ErrPath)
			}
		}
	}
	return &replacementDirectory{file: files[len(files)-1], path: full, close: func() error { return closeReplacementFiles(files) }}, nil
}

func replacePinned(target *replacementDirectory, name string, private *replacementDirectory, staged, backup string) (string, error) {
	to, err := windows.UTF16PtrFromString(filepath.Join(target.path, name))
	if err != nil {
		return backup, err
	}
	from, err := windows.UTF16PtrFromString(filepath.Join(private.path, staged))
	if err != nil {
		return backup, err
	}
	saved, err := windows.UTF16PtrFromString(filepath.Join(private.path, backup))
	if err != nil {
		return backup, err
	}
	// No ignore-ACL/merge flags and no unsupported WRITE_THROUGH flag. A failed
	// call may already have moved the original; callers retain all names.
	ok, _, callErr := replaceFilePreserving.Call(uintptr(unsafe.Pointer(to)), uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(saved)), 0, 0, 0)
	if ok == 0 {
		return backup, callErr
	}
	return backup, nil
}

func syncReplacement(target, private *replacementDirectory) error {
	// ReplaceFile has no supported write-through flag. File data is synced by
	// staging; crash recovery must inspect the journal and preserved names.
	return nil
}

func qualifyReplacementVolume(target, private *replacementDirectory) error {
	var a, b windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(target.file.Fd()), &a); err != nil {
		return err
	}
	if err := windows.GetFileInformationByHandle(windows.Handle(private.file.Fd()), &b); err != nil {
		return err
	}
	if a.VolumeSerialNumber != b.VolumeSerialNumber {
		return ErrUnsupported
	}
	return nil
}

func movePinnedExclusive(from *replacementDirectory, name string, to *replacementDirectory, destination string) error {
	source, err := windows.UTF16PtrFromString(filepath.Join(from.path, name))
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(filepath.Join(to.path, destination))
	if err != nil {
		return err
	}
	// No replacement, cross-volume copy, or delayed reboot operation.
	return windows.MoveFileEx(source, target, 0)
}
