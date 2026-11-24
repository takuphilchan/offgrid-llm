package inference

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"strings"
)

func openONNXRuntimeConfigFile(filename string) (*os.File, error) {
	if strings.HasPrefix(filename, `\\`) {
		return nil, errors.New("ONNX enrollment cannot use a device or network path")
	}
	name, err := windows.UTF16PtrFromString(filename)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), filename), nil
}
