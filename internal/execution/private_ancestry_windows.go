package execution

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TrustedInstaller owns protected Windows ancestors and is already an OS-wide
// privileged authority. No other service/user/group identity is inferred trusted.
const ancestryTrustedInstaller = "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"

func checkPrivateAncestor(path string, _ os.FileInfo) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	trusted := func(sid string) bool {
		return sid == user.User.Sid.String() || sid == "S-1-5-18" || sid == "S-1-5-32-544" || sid == ancestryTrustedInstaller
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !trusted(owner.String()) {
		return ErrAuthority
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return ErrAuthority
	}
	// DELETE can move this ancestor even if its own parent is readonly.
	// Write-data/attributes/EA also cannot be accepted as harmless read access
	// (for example, changing a directory's reparse metadata).
	const fileDeleteChild = 0x40
	const mutation = windows.DELETE | fileDeleteChild | windows.WRITE_DAC | windows.WRITE_OWNER |
		windows.FILE_WRITE_DATA | windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES | windows.GENERIC_ALL | windows.GENERIC_WRITE
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return ErrAuthority
		}
		if ace.Mask&mutation == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		if !trusted(sid) && sid != "S-1-3-4" {
			return ErrAuthority
		}
	}
	return nil
}
