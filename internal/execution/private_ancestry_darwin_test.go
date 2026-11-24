//go:build darwin && cgo

package execution

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrivateAncestryDarwinACL(t *testing.T) {
	for _, fixture := range []struct {
		name, entry string
		reject      bool
	}{
		{"read-only", "everyone allow list,search,readattr,readsecurity", false},
		{"delete-child", "everyone allow delete_child", true},
		{"delete-self", "everyone allow delete", true},
		{"write-security", "everyone allow writesecurity", true},
		{"deny-only", "everyone deny delete", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			directory := privateAncestryTestDirectory(t)
			child := filepath.Join(directory, "private")
			if err := EnsurePrivateDirectory(child); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("/bin/chmod", "+a", fixture.entry, directory).CombinedOutput(); err != nil {
				t.Fatalf("isolated ACL setup: %v %s", err, output)
			}
			t.Cleanup(func() {
				if output, err := exec.Command("/bin/chmod", "-N", directory).CombinedOutput(); err != nil {
					t.Errorf("isolated ACL cleanup: %v %s", err, output)
				}
			})
			if err := CheckPrivateAncestry(child); (err != nil) != fixture.reject {
				t.Fatalf("rejected=%v, want %v: %v", err != nil, fixture.reject, err)
			}
		})
	}
}
