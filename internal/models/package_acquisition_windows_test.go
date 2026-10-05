//go:build windows

package models

import (
	"context"
	"golang.org/x/sys/windows"
	"path/filepath"
	"testing"
)

func TestRepairPreservesPriorPackageDuringWindowsSharingFailure(t *testing.T) {
	m, hf, _ := networkPackageFixture(t, "piper")
	store := NewPackageStore(t.TempDir())
	if _, err := store.Import(context.Background(), m, fixturePackageSource); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(store.modelsDir, PackageDirectory, m.ID, m.Revision)
	p, err := windows.UTF16PtrFromString(target)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, acquireErr := store.AcquireFromHub(context.Background(), m, testRepairID, true, fixtureProvenance(), hf, noCheckpoint)
	windows.CloseHandle(handle)
	if acquireErr == nil {
		t.Fatal("replacement unexpectedly succeeded while destination denied delete sharing")
	}
	if _, err = store.Verify(context.Background(), m.ID, m.Revision); err != nil {
		t.Fatal("original was lost", err)
	}
	if _, err = store.AcquireFromHub(context.Background(), m, testRepairID, true, fixtureProvenance(), hf, noCheckpoint); err != nil {
		t.Fatal("explicit repair retry failed", err)
	}
}
