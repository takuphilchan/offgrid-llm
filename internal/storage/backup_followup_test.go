package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestEncryptedBackupFollowUpRetainsOwnershipAndVerifiedArchive(t *testing.T) {
	data, root := t.TempDir(), t.TempDir()
	key := backupTestKey(t)
	archive := filepath.Join(root, "backup.age")
	called := false
	want := errors.New("follow-up failed after backup")
	manifest, err := BackupWorkspaceThen(context.Background(), data, archive, "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}, func(manifest *BackupManifest) error {
		called = true
		if manifest.Encryption != BackupEncryptionAge {
			t.Fatal("follow-up received wrong transport")
		}
		if lock, err := AcquireOwnership(data); !errors.Is(err, ErrWorkspaceInUse) {
			if lock != nil {
				lock.Close()
			}
			t.Fatal("workspace owner was released before follow-up", err)
		}
		if _, err := VerifyBackupWithOptions(context.Background(), archive, BackupReadOptions{Identities: []age.Identity{key}}); err != nil {
			t.Fatal("follow-up started before verification", err)
		}
		return want
	})
	if !called || manifest == nil || !errors.Is(err, want) || !strings.Contains(err.Error(), archive) {
		t.Fatal("retained backup outcome lost", manifest, err)
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatal("follow-up failure removed backup")
	}
	owner, err := AcquireOwnership(data)
	if err != nil {
		t.Fatal("follow-up leaked ownership", err)
	}
	owner.Close()
}

func TestEncryptedBackupFollowUpNeverStartsOnBackupFailure(t *testing.T) {
	key := backupTestKey(t)
	for _, stage := range []string{"backup_opened", "backup_verified", "backup_published"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = context.WithValue(ctx, backupBoundaryKey{}, backupBoundaryHook(func(current, path string) error {
				if current == stage {
					cancel()
				}
				return nil
			}))
			called := false
			_, err := BackupWorkspaceThen(ctx, t.TempDir(), filepath.Join(t.TempDir(), "backup.age"), "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}, func(*BackupManifest) error { called = true; return nil })
			if called || !errors.Is(err, context.Canceled) {
				t.Fatal("cancelled backup reached mutation", called, err)
			}
		})
	}
}
