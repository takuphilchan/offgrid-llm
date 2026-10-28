package execution

import (
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

func privateWindowsObject(path string) (*windows.SECURITY_DESCRIPTOR, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return nil, "", ErrAuthority
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, "", err
	}
	sid := user.User.Sid.String()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return nil, "", err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return nil, "", ErrAuthority
	}
	if owner.String() != sid && owner.String() != "S-1-5-18" && owner.String() != "S-1-5-32-544" {
		return nil, "", ErrAuthority
	}
	return sd, sid, nil
}

func ProtectPrivatePath(path string) error {
	_, sid, err := privateWindowsObject(path)
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}

func CheckPrivatePath(path string) error {
	sd, sid, err := privateWindowsObject(path)
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return ErrAuthority
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err = windows.GetAce(acl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return ErrAuthority
		}
		allowed := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		if allowed != sid && allowed != "S-1-5-18" && allowed != "S-1-5-32-544" {
			return ErrAuthority
		}
	}
	return nil
}
