package execution

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Compare physical local-volume paths, not DOS/extended/substituted drive
// spellings. EvalSymlinks alone preserves namespace forms that filepath.Rel
// treats as different volumes even when they name the same directory.
func canonicalPrivateLocation(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", ErrPath
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", ErrPath
	}
	handle, err := windows.CreateFile(p, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		// Preserve missing-leaf errors for resolvePrivateLocation. Once opened,
		// a failed canonicalization must never be mistaken for a missing leaf.
		return "", err
	}
	defer windows.CloseHandle(handle)
	const volumeNameGUID = 1
	buffer := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], uint32(len(buffer)), volumeNameGUID)
	if err != nil || n == 0 || n >= uint32(len(buffer)) {
		return "", ErrPath
	}
	resolved := windows.UTF16ToString(buffer[:n])
	volume := filepath.VolumeName(resolved)
	if !strings.HasPrefix(volume, `\\?\Volume{`) || !strings.HasSuffix(volume, "}") || !filepath.IsAbs(resolved) {
		return "", ErrPath
	}
	// Network shares and non-Mount-Manager volumes cannot establish this local
	// identity and fail closed; never fall back to a lexical disjointness claim.
	return filepath.Clean(resolved), nil
}
