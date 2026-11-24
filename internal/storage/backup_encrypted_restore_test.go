package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestEncryptedRestoreCommittedWALAfterCrash(t *testing.T) {
	key := backupTestKey(t)
	testBackupRestoresCommittedWALAfterCrash(t, BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}, BackupReadOptions{Identities: []age.Identity{key}, RequireEncryption: true})
}

func TestEncryptedRestoreRetainsDatabaseAndVersionGates(t *testing.T) {
	key := backupTestKey(t)
	for _, tc := range []struct {
		name, path, version string
		data                []byte
	}{
		{"corrupt SQLite", "broken.db", "0.4.3", []byte("not a SQLite database")},
		{"wrong version", "state.txt", "different", []byte("private state")},
		{"development version", "state.txt", "dev", []byte("private state")},
		{"traversal", "../escape", "0.4.3", []byte("outside")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plainFile := makeBackupFixture(t, tc.path, tc.data, nil)
			plain, _ := os.ReadFile(plainFile)
			root := t.TempDir()
			archive := filepath.Join(root, "archive.age")
			if err := os.WriteFile(archive, encryptedBackupBytes(t, plain, key.Recipient()), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := RestoreWorkspaceWithOptions(context.Background(), archive, filepath.Join(root, "restored"), tc.version, BackupReadOptions{Identities: []age.Identity{key}}); err == nil {
				t.Fatal("invalid restore published")
			}
			entries, _ := os.ReadDir(root)
			if len(entries) != 1 {
				t.Fatal("failed restore retained unreported output", entries)
			}
		})
	}
}
