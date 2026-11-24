package execution

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPrivateAncestryWindowsMutationMasks(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		name, ace string
		reject    bool
	}{
		{"readonly", "(A;;FRFX;;;WD)", false},
		{"delete-child", "(A;;0x40;;;WD)", true},
		{"delete-self", "(A;;SD;;;WD)", true},
		{"rewrite-dacl", "(A;;WD;;;WD)", true},
		{"rewrite-owner", "(A;;WO;;;WD)", true},
		{"generic-write", "(A;;GW;;;WD)", true},
		{"generic-all", "(A;;GA;;;WD)", true},
		{"inherit-only", "(A;OICIIO;FA;;;WD)", false},
		{"deny-does-not-prove-allow-safe", "(D;;SD;;;WD)(A;;SD;;;WD)", true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			directory := privateAncestryTestDirectory(t)
			child := filepath.Join(directory, "private")
			if err := EnsurePrivateDirectory(child); err != nil {
				t.Fatal(err)
			}
			sd, err := windows.SecurityDescriptorFromString("D:P" + fixture.ace + "(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
			if err != nil {
				t.Fatal(err)
			}
			acl, _, err := sd.DACL()
			if err != nil {
				t.Fatal(err)
			}
			if err := windows.SetNamedSecurityInfo(directory, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
				t.Fatal(err)
			}
			// Restore only this freshly-created fixture's ACL for cleanup when
			// Everyone deny entries also affect the current process.
			t.Cleanup(func() {
				if err := ProtectPrivatePath(directory); err != nil {
					t.Error(err)
				}
			})
			before, err := windows.GetNamedSecurityInfo(directory, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
			if err != nil {
				t.Fatal(err)
			}
			err = CheckPrivateAncestry(child)
			if (err != nil) != fixture.reject {
				t.Fatalf("rejected=%v, want %v: %v", err != nil, fixture.reject, err)
			}
			after, err := windows.GetNamedSecurityInfo(directory, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
			if err != nil || before.String() != after.String() {
				t.Fatal("ancestry check changed permissions")
			}
			if _, err := os.Stat(child); err != nil {
				t.Fatal(err)
			}
		})
	}
}
