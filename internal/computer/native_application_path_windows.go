//go:build windows

package computer

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Validate the opened object's final path, without enumerating its ancestors.
// AppModel registrations can be readable inside WindowsApps even when listing
// that directory is denied. EvalSymlinks incorrectly rejects those packages.
// This still rejects redirected paths, devices, remote paths and alternate streams.
func validateInstalledApplicationPath(path string) error {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) || len(filepath.VolumeName(path)) != 2 || strings.ContainsAny(path[2:], ":\x00") {
		return ErrControlScope
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ErrControlScope
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return ErrControlScope
	}
	defer windows.CloseHandle(handle)
	// FILE_NAME_NORMALIZED | VOLUME_NAME_DOS. Never fall back to the opened
	// (unresolved) spelling when canonicalization is unavailable.
	buffer := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), 0)
	if err != nil || n == 0 || n >= uint32(len(buffer)) {
		return ErrControlScope
	}
	resolved := windows.UTF16ToString(buffer[:n])
	if !strings.EqualFold(resolved, `\\?\`+path) {
		return ErrControlScope
	}
	return nil
}
