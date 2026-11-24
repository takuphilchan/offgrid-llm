//go:build linux || darwin

package execution

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateAncestryUnixModesAndSticky(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		mode   os.FileMode
		reject bool
	}{
		{"readonly-shared", 0755, false}, {"group-write", 0770, true},
		{"world-write", 0777, true}, {"trusted-sticky", 0777 | os.ModeSticky, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			directory := privateAncestryTestDirectory(t)
			child := filepath.Join(directory, "private")
			if err := EnsurePrivateDirectory(child); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(directory, fixture.mode); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(directory)
			if err != nil {
				t.Fatal(err)
			}
			err = CheckPrivateAncestry(child)
			if (err != nil) != fixture.reject {
				t.Fatalf("rejected=%v, want %v: %v", err != nil, fixture.reject, err)
			}
			after, err := os.Stat(directory)
			if err != nil || before.Mode() != after.Mode() {
				t.Fatal("check repaired ancestor mode")
			}
		})
	}
}

func TestPrivateAncestryUnixRejectsForeignOwnerEvenWhenSticky(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("isolated foreign-owner fixture requires root")
	}
	directory := privateAncestryTestDirectory(t)
	child := filepath.Join(directory, "private")
	if err := EnsurePrivateDirectory(child); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(directory, 65534, -1); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chown(directory, 0, -1); err != nil {
			t.Error(err)
		}
	})
	if CheckPrivateAncestry(child) == nil {
		t.Fatal("foreign directory owner can defeat sticky protection")
	}
}
