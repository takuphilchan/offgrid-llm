package inference

import (
	"errors"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var errONNXMutableRuntime = errors.New("ONNX runtime requires administrator-provisioned read-only ownership and directory ACLs")

const onnxDirectoryDeleteChild = 0x0040 // FILE_DELETE_CHILD, distinct from FILE_DELETE.

func onnxPrivilegedSID(sid string) bool {
	return sid == "S-1-5-18" || sid == "S-1-5-32-544" || sid == "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464"
}

// Retained file handles do not prevent a new importable file being created in
// an existing directory. The first supported Windows installation therefore
// requires administrator-owned ACLs, inaccessible to this unprivileged service.
// Administrator/OS mutation remains outside this boundary.
func checkONNXRuntimeNamespace(path string, ancestor bool) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	return validateONNXRuntimeSecurityDescriptor(sd, ancestor)
}

func validateONNXRuntimeSecurityDescriptor(sd *windows.SECURITY_DESCRIPTOR, ancestor bool) error {
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !onnxPrivilegedSID(owner.String()) {
		return errONNXMutableRuntime
	}
	acl, _, err := sd.DACL()
	if err != nil || acl == nil {
		return errONNXMutableRuntime
	}
	write := windows.ACCESS_MASK(windows.DELETE | windows.WRITE_DAC | windows.WRITE_OWNER | windows.GENERIC_WRITE | windows.GENERIC_ALL | onnxDirectoryDeleteChild)
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
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return errONNXMutableRuntime
		}
		if ace.Mask&write != 0 && !onnxPrivilegedSID((*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()) {
			return errONNXMutableRuntime
		}
	}
	return nil
}

func checkONNXRuntimeAncestors(path string) error {
	if strings.HasPrefix(path, `\\`) {
		return errONNXMutableRuntime
	}
	for depth, current := 0, filepath.Dir(path); ; depth++ {
		if depth > 128 {
			return errONNXMutableRuntime
		}
		if err := checkONNXRuntimeNamespace(current, true); err != nil {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func checkONNXUnprivilegedToken() error {
	token := windows.GetCurrentProcessToken()
	var elevated, out uint32
	if err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevated)), 4, &out); err != nil || out != 4 || elevated != 0 {
		return errors.New("ONNX runtime requires a non-elevated service token")
	}
	user, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	if onnxPrivilegedSID(user.User.Sid.String()) {
		return errors.New("ONNX runtime cannot run as a privileged service account")
	}
	groups, err := token.GetTokenGroups()
	if err != nil {
		return err
	}
	for _, group := range groups.AllGroups() {
		if group.Attributes&windows.SE_GROUP_ENABLED != 0 && group.Attributes&windows.SE_GROUP_USE_FOR_DENY_ONLY == 0 && onnxPrivilegedSID(group.Sid.String()) {
			return errors.New("ONNX runtime cannot use an enabled privileged group")
		}
	}
	var required uint32
	err = windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &required)
	if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) || required < 4 || required > 64<<10 {
		return errors.New("ONNX service privileges cannot be verified")
	}
	buffer := make([]byte, required)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buffer[0], required, &out); err != nil || out > required || out < 4 {
		return errors.New("ONNX service privileges cannot be read")
	}
	privileges := (*windows.Tokenprivileges)(unsafe.Pointer(&buffer[0]))
	if privileges.PrivilegeCount > (out-4)/uint32(unsafe.Sizeof(windows.LUIDAndAttributes{})) {
		return errors.New("ONNX service privilege record is inconsistent")
	}
	// Allow only ordinary interactive-account privileges; even a disabled
	// privileged right can later be enabled by the worker's inherited token.
	allowed := map[windows.LUID]bool{}
	for _, name := range []string{"SeChangeNotifyPrivilege", "SeIncreaseWorkingSetPrivilege", "SeTimeZonePrivilege", "SeShutdownPrivilege", "SeUndockPrivilege"} {
		text, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return err
		}
		var wanted windows.LUID
		if err := windows.LookupPrivilegeValue(nil, text, &wanted); err != nil {
			return err
		}
		allowed[wanted] = true
	}
	for _, privilege := range privileges.AllPrivileges() {
		if privilege.Attributes&windows.SE_PRIVILEGE_REMOVED == 0 && !allowed[privilege.Luid] {
			return errors.New("ONNX service token can acquire a privileged filesystem/process override")
		}
	}
	return nil
}
