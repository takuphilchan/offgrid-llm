//go:build !windows

package execution

import "os"

// This is used only after verified supervisor settlement and atomic detachment
// into the private cleanup namespace. It is not inode-conditional unlink on
// arbitrary shared folders; unknown worker descendants prevent this admission.
func removeCopyEntry(root *os.Root, name string, expected os.FileInfo) error {
	if err := checkedCopyEntry(root, name, expected); err != nil {
		return err
	}
	return root.Remove(name)
}
