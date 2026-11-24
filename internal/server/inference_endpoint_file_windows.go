package server

import (
	"errors"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func openInferenceEndpointConfigFile(path string) (*os.File, error) {
	if strings.HasPrefix(path, `\\`) {
		return nil, errors.New("endpoint configuration cannot use a device or network path")
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
