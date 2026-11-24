package execution

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWorkingCopyOpenRejectsWindowsJunction(t *testing.T) {
	directory := t.TempDir()
	original := filepath.Join(directory, "original")
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "input"), []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	expected, err := root.Lstat("original/input")
	if err != nil {
		t.Fatal(err)
	}
	// Directory junction creation on NTFS does not need symbolic-link privilege.
	// Both target and link are freshly generated fixture-owned paths.
	junction := filepath.Join(directory, "junction")
	createWorkingCopyTestJunction(t, junction, original)
	if file, err := openWorkingCopyFile(root, "junction/input", expected); err == nil {
		file.Close()
		t.Fatal("worker input followed directory junction")
	}
	if err = syncReplacementFile(root, "junction/input", expected); err == nil {
		t.Fatal("replacement sync followed directory junction")
	}
	if data, err := root.ReadFile("original/input"); err != nil || string(data) != "preserved" {
		t.Fatal("rejected junction changed retained bytes", string(data), err)
	}
}

func createWorkingCopyTestJunction(t *testing.T, name, target string) {
	t.Helper()
	if err := os.Mkdir(name, 0700); err != nil {
		t.Fatal(err)
	}
	encodedName, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(encodedName, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	substitute, err := windows.UTF16FromString(`\??\` + target)
	if err != nil {
		t.Fatal(err)
	}
	printable, err := windows.UTF16FromString(target)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 16+2*(len(substitute)+len(printable)))
	binary.LittleEndian.PutUint32(buffer, windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:], uint16(len(buffer)-8))
	binary.LittleEndian.PutUint16(buffer[10:], uint16((len(substitute)-1)*2))
	binary.LittleEndian.PutUint16(buffer[12:], uint16(len(substitute)*2))
	binary.LittleEndian.PutUint16(buffer[14:], uint16((len(printable)-1)*2))
	for i, unit := range append(substitute, printable...) {
		binary.LittleEndian.PutUint16(buffer[16+i*2:], unit)
	}
	var returned uint32
	if err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil); err != nil {
		t.Fatal("create disposable NTFS junction:", err)
	}
}
