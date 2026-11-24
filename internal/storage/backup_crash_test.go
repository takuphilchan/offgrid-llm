package storage

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestEncryptedBackupCrashHelper(t *testing.T) {
	root := os.Getenv("OFFGRID_ENCRYPTED_BACKUP_CRASH_ROOT")
	if root == "" {
		return
	}
	marker, err := os.ReadFile(filepath.Join(root, "isolated-fixture"))
	if err != nil || string(marker) != "encrypted backup crash fixture" {
		os.Exit(81)
	}
	secret, err := os.ReadFile(filepath.Join(root, "identity.txt"))
	if err != nil {
		os.Exit(82)
	}
	identity, err := age.ParseX25519Identity(string(secret))
	if err != nil {
		os.Exit(83)
	}
	stage := os.Getenv("OFFGRID_ENCRYPTED_BACKUP_CRASH_STAGE")
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(current, path string) error {
		if current == stage {
			os.Exit(73)
		}
		return nil
	}))
	archive := filepath.Join(root, "backup.age")
	if strings.HasPrefix(stage, "backup_") {
		_, err = BackupWorkspaceWithOptions(ctx, filepath.Join(root, "data"), archive, "v1", BackupWriteOptions{Recipients: []age.Recipient{identity.Recipient()}})
	} else {
		_, err = RestoreWorkspaceWithOptions(ctx, archive, filepath.Join(root, "restored"), "v1", BackupReadOptions{Identities: []age.Identity{identity}})
	}
	if err != nil {
		os.Exit(84)
	}
	os.Exit(85)
}

func TestEncryptedBackupCrashRecoveryBoundaries(t *testing.T) {
	for _, stage := range []string{"backup_closed", "backup_verified", "backup_published", "restore_staged", "restore_file_synced", "restore_published"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			data := filepath.Join(root, "data")
			if err := os.Mkdir(data, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(data, "private.txt"), []byte("original source preserved"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "isolated-fixture"), []byte("encrypted backup crash fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			key := backupTestKey(t)
			if err := os.WriteFile(filepath.Join(root, "identity.txt"), []byte(key.String()), 0600); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(root, "backup.age")
			if strings.HasPrefix(stage, "restore_") {
				if _, err := BackupWorkspaceWithOptions(context.Background(), data, archive, "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(os.Args[0], "-test.run=^TestEncryptedBackupCrashHelper$")
			command.Env = append(os.Environ(), "OFFGRID_ENCRYPTED_BACKUP_CRASH_ROOT="+root, "OFFGRID_ENCRYPTED_BACKUP_CRASH_STAGE="+stage)
			if output, err := command.CombinedOutput(); err == nil || command.ProcessState.ExitCode() != 73 {
				t.Fatalf("crash boundary not reached: %v %s", err, output)
			}
			original, _ := os.ReadFile(filepath.Join(data, "private.txt"))
			if string(original) != "original source preserved" {
				t.Fatal("crash changed source")
			}
			if strings.HasPrefix(stage, "backup_") {
				_, publishedErr := os.Stat(archive)
				if (stage == "backup_published") != (publishedErr == nil) {
					t.Fatal("unexpected publication state", publishedErr)
				}
				entries, _ := os.ReadDir(root)
				verified := 0
				for _, entry := range entries {
					if entry.Name() != "backup.age" && !strings.HasPrefix(entry.Name(), ".offgrid-backup-") {
						continue
					}
					if _, err := VerifyBackupWithOptions(context.Background(), filepath.Join(root, entry.Name()), BackupReadOptions{Identities: []age.Identity{key}, RequireEncryption: true}); err != nil {
						t.Fatal("crash left invalid/plaintext completed archive", err)
					}
					verified++
				}
				if verified == 0 {
					t.Fatal("crash fixture lost archive")
				}
			} else {
				_, publishedErr := os.Stat(filepath.Join(root, "restored"))
				if (stage == "restore_published") != (publishedErr == nil) {
					t.Fatal("unexpected restore publication", publishedErr)
				}
				// A new explicit destination remains recoverable; there is no
				// automatic retry, activation, merging or temp cleanup on restart.
				if _, err := RestoreWorkspaceWithOptions(context.Background(), archive, filepath.Join(root, "explicit-recovery"), "v1", BackupReadOptions{Identities: []age.Identity{key}}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
