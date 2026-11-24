//go:build windows

package main

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"golang.org/x/sys/windows"
)

func makeBrowserStoragePublic(t *testing.T, path string) {
	t.Helper()
	// The caller supplies only disposable test state. Broad access is deliberate
	// fixture input; production must reject it without rewriting the DACL.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if execution.CheckPrivatePath(path) == nil {
		t.Fatal("fixture did not expose broad access")
	}
}
