package server

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func verifyEndpointOwnerSecurity(path string, info os.FileInfo, ancestor bool) error {
	current, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	trusted := func(sid string) bool {
		return sid == current.User.Sid.String() || sid == "S-1-5-18" || sid == "S-1-5-32-544" || sid == "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !trusted(owner.String()) {
		return errors.New("endpoint owner path has untrusted ownership")
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return errors.New("endpoint owner ACL is unavailable")
	}
	write := windows.ACCESS_MASK(windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL | 0x0040)
	if !ancestor {
		write |= windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA | windows.FILE_WRITE_EA | windows.FILE_WRITE_ATTRIBUTES
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask&write != 0 && !trusted((*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()) {
			return errors.New("endpoint owner path allows an untrusted writer")
		}
	}
	if info.Mode().IsRegular() {
		file, err := openObservedInferenceEndpointFile(path, info)
		if err != nil {
			return err
		}
		defer file.Close()
		var identity windows.ByHandleFileInformation
		if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &identity); err != nil {
			return err
		}
		if identity.NumberOfLinks != 1 {
			return errors.New("endpoint owner file cannot have hardlinks")
		}
	}
	return nil
}
