//go:build linux || darwin

package storage

import (
	"os"
	"testing"
)

func TestBackupPrivateUnixCreation(t *testing.T) {
	file, err := createPrivateBackupFile(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	directory, err := createPrivateRestoreDirectory(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{file.Name(): 0600, directory: 0700} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatal("private mode not enforced", path, err)
		}
	}
}
