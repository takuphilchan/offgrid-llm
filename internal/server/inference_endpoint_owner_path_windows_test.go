package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TEMP on developer hosts can intentionally delegate Modify to sandbox users.
// Create only this owned fixture with a private DACL under a verified ancestry;
// never change the permissions of the existing user directories.
func endpointOwnerFixtureDirectory(t *testing.T) string {
	t.Helper()
	parent, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyEndpointOwnerPath(parent, true); err != nil {
		t.Fatal("isolated private fixture parent unavailable", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := endpointOwnerNonce()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "offgrid-owner-test-"+nonce)
	wide, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	if err := windows.CreateDirectory(wide, &attributes); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if filepath.Dir(path) != parent || !strings.HasPrefix(filepath.Base(path), "offgrid-owner-test-") {
			t.Error("unsafe fixture cleanup path")
			return
		}
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	})
	return path
}
