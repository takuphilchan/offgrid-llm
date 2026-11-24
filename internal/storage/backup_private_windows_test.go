package storage

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestBackupPrivateWindowsCreation(t *testing.T) {
	file, err := createPrivateBackupFile(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	directory, err := createPrivateRestoreDirectory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(directory, "private.txt")
	if err := os.WriteFile(child, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{file.Name(), directory, child} {
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		acl, _, err := sd.DACL()
		if err != nil || acl == nil || acl.AceCount != 2 {
			t.Fatal("unexpected private ACL", path, acl, err)
		}
		for i := uint32(0); i < uint32(acl.AceCount); i++ {
			var ace *windows.ACCESS_ALLOWED_ACE
			if err := windows.GetAce(acl, i, &ace); err != nil {
				t.Fatal(err)
			}
			if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
				t.Fatal("unexpected ACE")
			}
			sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
			if sid != user.User.Sid.String() && sid != "S-1-5-18" {
				t.Fatal("private state accessible to other identity", sid)
			}
		}
	}
}
