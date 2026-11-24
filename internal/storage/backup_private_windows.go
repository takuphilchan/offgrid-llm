package storage

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const backupPublicationMovesSource = true

func openBackupInputFile(path string) (*os.File, error) { return os.Open(path) }

func openBackupSourceFile(root *os.Root, path string) (*os.File, error) { return root.Open(path) }

// The protected DACL is supplied at creation, before any content exists. Chmod
// after os.CreateTemp cannot revoke handles opened during a permissive ACL gap.
func backupPrivateAttributes() (*windows.SecurityAttributes, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return nil, err
	}
	return &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: descriptor}, nil
}

func backupPrivateName(parent, prefix string) (string, *uint16, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", nil, err
	}
	path, err := filepath.Abs(filepath.Join(parent, prefix+hex.EncodeToString(entropy[:])))
	if err != nil {
		return "", nil, err
	}
	native := path
	if !strings.HasPrefix(native, `\\?\`) {
		if strings.HasPrefix(native, `\\`) {
			native = `\\?\UNC\` + strings.TrimPrefix(native, `\\`)
		} else {
			native = `\\?\` + native
		}
	}
	wide, err := windows.UTF16PtrFromString(native)
	return path, wide, err
}

func createPrivateBackupFile(parent string) (*os.File, error) {
	attributes, err := backupPrivateAttributes()
	if err != nil {
		return nil, err
	}
	path, wide, err := backupPrivateName(parent, ".offgrid-backup-")
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(wide, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, attributes, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func createPrivateRestoreDirectory(parent string) (string, error) {
	attributes, err := backupPrivateAttributes()
	if err != nil {
		return "", err
	}
	path, wide, err := backupPrivateName(parent, ".offgrid-restore-")
	if err != nil {
		return "", err
	}
	if err := windows.CreateDirectory(wide, attributes); err != nil {
		return "", err
	}
	return path, nil
}

func publishBackupFile(source, target string) error { return publishDirectory(source, target) }

// Windows uses no-replace MoveFileEx(MOVEFILE_WRITE_THROUGH) for publication.
// Directory fsync is not a portable Windows API; file contents are flushed first.
func syncBackupParent(string) error { return nil }
