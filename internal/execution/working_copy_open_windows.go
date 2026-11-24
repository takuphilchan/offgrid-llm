package execution

import (
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func qualifyWorkingCopyDirectory(directory *os.File) error {
	// Go's Readdir uses handle-returned file IDs only with these capabilities.
	// Its pathname-based ID fallback cannot bind a later open to this snapshot.
	var flags uint32
	err := windows.GetVolumeInformationByHandle(windows.Handle(directory.Fd()), nil, 0, nil, nil, &flags, nil, 0)
	runtime.KeepAlive(directory)
	if err != nil {
		return err
	}
	const required = windows.FILE_SUPPORTS_OBJECT_IDS | windows.FILE_SUPPORTS_OPEN_BY_FILE_ID
	if flags&required != required {
		return ErrUnsupported
	}
	return nil
}

func openWorkingCopyRooted(root *os.Root, name string, writable bool) (*os.File, error) {
	return openWorkingCopyWindows(root, name, writable, false)
}

func openWorkingCopyWindows(root *os.Root, name string, writable, deletion bool) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	if name == "." {
		return parent, nil
	}
	parts := strings.Split(name, "/")
	for i, part := range parts {
		objectName, nameErr := windows.NewNTUnicodeString(part)
		if nameErr != nil {
			parent.Close()
			return nil, nameErr
		}
		attributes := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: objectName, Attributes: windows.OBJ_CASE_INSENSITIVE}
		attributes.Length = uint32(unsafe.Sizeof(attributes))
		access := uint32(windows.FILE_GENERIC_READ)
		if writable && i == len(parts)-1 {
			access |= windows.FILE_GENERIC_WRITE
		}
		sharing := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)
		if deletion && i == len(parts)-1 {
			access = windows.FILE_READ_ATTRIBUTES | windows.SYNCHRONIZE | windows.DELETE
			// Refuse existing/new writers while inspecting and disposing this
			// exact object. A pathname replacement cannot redirect disposition.
			sharing = windows.FILE_SHARE_READ | windows.FILE_SHARE_DELETE
		}
		var handle windows.Handle
		var status windows.IO_STATUS_BLOCK
		err = windows.NtCreateFile(&handle, access, &attributes, &status, nil, 0,
			sharing,
			windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
		runtime.KeepAlive(parent)
		parent.Close()
		if err != nil {
			return nil, &os.PathError{Op: "open worker input", Path: name, Err: err}
		}
		var info windows.ByHandleFileInformation
		err = windows.GetFileInformationByHandle(handle, &info)
		if err == nil && (info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 && (!deletion || i < len(parts)-1) || (i < len(parts)-1 && info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0)) {
			err = ErrPath
		}
		if err != nil {
			windows.CloseHandle(handle)
			return nil, err
		}
		parent = os.NewFile(uintptr(handle), name)
	}
	return parent, nil
}
