package inference

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
)

func (lease *managedOllamaFiles) open(path string, directory bool) (*os.File, error) {
	resolved, resolveErr := filepath.EvalSymlinks(path)
	if resolveErr != nil || !strings.EqualFold(filepath.Clean(resolved), filepath.Clean(path)) {
		return nil, errors.New("managed Ollama paths cannot traverse links")
	}
	before, err := os.Lstat(path)
	if err != nil || before.IsDir() != directory || !directory && !before.Mode().IsRegular() || checkManagedOllamaPrivatePath(path) != nil {
		return nil, errors.New("managed Ollama paths must be existing private regular files or directories")
	}
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access, flags, sharing := uint32(windows.GENERIC_READ), uint32(windows.FILE_FLAG_OPEN_REPARSE_POINT), uint32(windows.FILE_SHARE_READ)
	if directory {
		access = windows.GENERIC_READ
		flags |= windows.FILE_FLAG_BACKUP_SEMANTICS
		sharing |= windows.FILE_SHARE_WRITE
	}
	handle, err := windows.CreateFile(encoded, access, sharing, nil, windows.OPEN_EXISTING, flags, 0)
	if err != nil {
		return nil, errors.New("managed Ollama file lease is unavailable")
	}
	file := os.NewFile(uintptr(handle), path)
	var attributes windows.ByHandleFileInformation
	info, statErr := file.Stat()
	attrErr := windows.GetFileInformationByHandle(handle, &attributes)
	if statErr != nil || attrErr != nil || attributes.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || !directory && attributes.NumberOfLinks != 1 || !os.SameFile(before, info) {
		file.Close()
		return nil, errors.New("managed Ollama file identity changed or is a link")
	}
	lease.files = append(lease.files, file)
	return file, nil
}
