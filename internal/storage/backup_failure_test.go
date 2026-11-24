package storage

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestEncryptedBackupCancellationAndFailureBoundaries(t *testing.T) {
	key := backupTestKey(t)
	for _, stage := range []string{"backup_opened", "backup_before_sync", "backup_closed", "backup_verified", "backup_published"} {
		for _, cancelled := range []bool{false, true} {
			t.Run(stage+map[bool]string{false: "/failure", true: "/cancel"}[cancelled], func(t *testing.T) {
				root, data := t.TempDir(), t.TempDir()
				if err := os.WriteFile(filepath.Join(data, "private.txt"), []byte("never a plaintext archive"), 0600); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				injected := errors.New("injected storage failure")
				ctx = context.WithValue(ctx, backupBoundaryKey{}, backupBoundaryHook(func(current, path string) error {
					if current != stage {
						return nil
					}
					if current == "backup_closed" {
						raw, err := os.ReadFile(path)
						if err != nil || !bytes.HasPrefix(raw, []byte("age-encryption.org/v1\n")) {
							t.Fatal("temporary archive was not ciphertext", err)
						}
					}
					if cancelled {
						cancel()
						return ctx.Err()
					}
					return injected
				}))
				archive := filepath.Join(root, "backup.age")
				manifest, err := BackupWorkspaceWithOptions(ctx, data, archive, "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}})
				if err == nil || (cancelled && !errors.Is(err, context.Canceled)) || (!cancelled && !errors.Is(err, injected)) {
					t.Fatal("lost failure", err)
				}
				entries, _ := os.ReadDir(root)
				if stage == "backup_published" {
					if len(entries) != 1 || manifest == nil || !strings.Contains(err.Error(), "published") {
						t.Fatal("published outcome lost", manifest, err, entries)
					}
					if _, err := VerifyBackupWithOptions(context.Background(), archive, BackupReadOptions{Identities: []age.Identity{key}}); err != nil {
						t.Fatal(err)
					}
				} else if len(entries) != 0 || manifest != nil {
					t.Fatal("failed backup published or retained temp", entries, manifest)
				}
			})
		}
	}
}

func TestEncryptedRestoreCancellationBoundaries(t *testing.T) {
	key := backupTestKey(t)
	plainFile := makeBackupFixture(t, "private.txt", []byte("private data"), nil)
	plain, _ := os.ReadFile(plainFile)
	archive := filepath.Join(t.TempDir(), "backup.age")
	if err := os.WriteFile(archive, encryptedBackupBytes(t, plain, key.Recipient()), 0600); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{"restore_authenticated", "restore_staged", "restore_file_synced", "restore_verified", "restore_published"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = context.WithValue(ctx, backupBoundaryKey{}, backupBoundaryHook(func(current, path string) error {
				if current == stage {
					cancel()
					return ctx.Err()
				}
				return nil
			}))
			manifest, err := RestoreWorkspaceWithOptions(ctx, archive, filepath.Join(root, "restored"), "0.4.3", BackupReadOptions{Identities: []age.Identity{key}})
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			entries, _ := os.ReadDir(root)
			if stage == "restore_published" {
				if manifest == nil || len(entries) != 1 || !strings.Contains(err.Error(), "restored at") {
					t.Fatal("published restore outcome lost", manifest, err)
				}
			} else if manifest != nil || len(entries) != 0 {
				t.Fatal("cancelled restore retained or published state", entries, manifest)
			}
		})
	}
}

func TestEncryptedBackupPublicationRacePreservesWinner(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	archive := filepath.Join(root, "winner.age")
	key := backupTestKey(t)
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage == "backup_verified" {
			return os.WriteFile(archive, []byte("other creator"), 0600)
		}
		return nil
	}))
	if _, err := BackupWorkspaceWithOptions(ctx, data, archive, "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}); err == nil {
		t.Fatal("replaced other output")
	}
	content, _ := os.ReadFile(archive)
	if string(content) != "other creator" {
		t.Fatal("winner changed")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("temporary ciphertext retained")
	}
}

func TestBackupCleanupFailureRetainsCauseAndPath(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "retained"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	original := errors.New("original failure")
	err := original
	identity, statErr := os.Lstat(directory)
	if statErr != nil {
		t.Fatal(statErr)
	}
	cleanupBackupPath(directory, false, identity, &err)
	if !errors.Is(err, original) || !strings.Contains(err.Error(), directory) || !strings.Contains(err.Error(), "retained") {
		t.Fatal("cleanup failure hidden", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "retained")); err != nil {
		t.Fatal("cleanup deleted unrelated child")
	}
}
