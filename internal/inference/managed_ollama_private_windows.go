package inference

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Inference cannot import execution (its service-client/RAG path depends back
// on inference). Keep this read-only launch check local; it never changes ACLs.
func checkManagedOllamaPrivatePath(path string) error {
	denied := errors.New("managed Ollama directory or file is not account-private")
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	current := user.User.Sid.String()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	allowed := func(sid string) bool { return sid == current || sid == "S-1-5-18" || sid == "S-1-5-32-544" }
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !allowed(owner.String()) {
		return denied
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return denied
	}
	for i := uint32(0); i < uint32(acl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !allowed((*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()) {
			return denied
		}
	}
	return nil
}
