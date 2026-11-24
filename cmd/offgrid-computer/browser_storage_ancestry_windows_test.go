package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"golang.org/x/sys/windows"
)

func TestBrowserStorageRejectsMutableAncestorWithoutRepair(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.MkdirTemp(home, ".offgrid-storage-ancestry-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(parent) != filepath.Clean(home) || !strings.HasPrefix(filepath.Base(parent), ".offgrid-storage-ancestry-") {
			t.Fatal("fixture cleanup escaped its owner")
		}
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	if err := execution.ProtectPrivatePath(parent); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(parent, "computer")
	if err := prepareBrowserStorage(context.Background(), directory, nil); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")(A;;0x40;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if err := execution.CheckPrivatePath(directory); err != nil {
		t.Fatalf("private child fixture changed: %v", err)
	}
	before, err := windows.GetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareBrowserStorage(context.Background(), directory, nil); err == nil {
		t.Fatal("private child admitted below Everyone DELETE_CHILD ancestor")
	}
	actual, err := windows.GetNamedSecurityInfo(parent, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if actual.String() != before.String() {
		t.Fatal("ancestor permissions were silently repaired")
	}
}
