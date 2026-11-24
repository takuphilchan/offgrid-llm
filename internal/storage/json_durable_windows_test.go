package storage

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestDurableSnapshotWindowsPrivateCreationAndPersistentReaderFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	if err := WriteDurableJSON(path, map[string]bool{"enabled": true}); err != nil {
		t.Fatal(err)
	}
	security, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := security.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("snapshot inherited a public DACL", err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	err = WriteDurableJSON(path, map[string]bool{"enabled": false})
	reader.Close()
	if err == nil {
		t.Fatal("persistent reader lock ignored")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != `{"enabled":true}` {
		t.Fatal("failed publication lost original", string(raw), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("failed private publication leaked temp", entries, err)
	}
}
