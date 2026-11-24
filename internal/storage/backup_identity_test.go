package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestEncryptedRestoreDoesNotCleanReusedStageName(t *testing.T) {
	key := backupTestKey(t)
	plain, _ := os.ReadFile(makeBackupFixture(t, "data.txt", []byte("data"), nil))
	archive := filepath.Join(t.TempDir(), "backup.age")
	if err := os.WriteFile(archive, encryptedBackupBytes(t, plain, key.Recipient()), 0600); err != nil {
		t.Fatal(err)
	}
	var oldStage string
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage == "restore_staged" {
			oldStage = path
		}
		if stage == "restore_published" {
			if err := os.Mkdir(oldStage, 0700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(oldStage, "unrelated.txt"), []byte("preserve"), 0600)
		}
		return nil
	}))
	if _, err := RestoreWorkspaceWithOptions(ctx, archive, filepath.Join(t.TempDir(), "restored"), "0.4.3", BackupReadOptions{Identities: []age.Identity{key}}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(oldStage, "unrelated.txt")); err != nil || string(data) != "preserve" {
		t.Fatal("cleanup deleted unrelated replacement", err)
	}
}

func TestEncryptedBackupRefusesVerifiedPathSubstitution(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	key := backupTestKey(t)
	var replaced string
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage != "backup_verified" {
			return nil
		}
		replaced = path
		if err := os.Rename(path, path+".original"); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("unverified unrelated bytes"), 0600)
	}))
	output := filepath.Join(root, "output.age")
	if _, err := BackupWorkspaceWithOptions(ctx, data, output, "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatal("substitution was published", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("unverified file was published")
	}
	if content, err := os.ReadFile(replaced); err != nil || string(content) != "unverified unrelated bytes" {
		t.Fatal("replacement was cleaned", err)
	}
}

func TestEncryptedRestoreRefusesVerifiedStageSubstitution(t *testing.T) {
	key := backupTestKey(t)
	plain, _ := os.ReadFile(makeBackupFixture(t, "state.txt", []byte("data"), nil))
	archive := filepath.Join(t.TempDir(), "backup.age")
	if err := os.WriteFile(archive, encryptedBackupBytes(t, plain, key.Recipient()), 0600); err != nil {
		t.Fatal(err)
	}
	var replaced string
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage != "restore_verified" {
			return nil
		}
		replaced = path
		if err := os.Rename(path, path+".original"); err != nil {
			return err
		}
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(path, "unrelated"), []byte("preserve"), 0600)
	}))
	target := filepath.Join(t.TempDir(), "restored")
	if _, err := RestoreWorkspaceWithOptions(ctx, archive, target, "0.4.3", BackupReadOptions{Identities: []age.Identity{key}}); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatal("unverified stage published", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("substituted stage was activated")
	}
	if data, err := os.ReadFile(filepath.Join(replaced, "unrelated")); err != nil || string(data) != "preserve" {
		t.Fatal("unrelated stage replacement deleted", err)
	}
}

func TestEncryptedBackupSourceIdentityCannotBeReplacedWithMatchingMetadata(t *testing.T) {
	data, root := t.TempDir(), t.TempDir()
	name := filepath.Join(data, "source.txt")
	if err := os.WriteFile(name, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage != "backup_source_observed" {
			return nil
		}
		if err := os.Rename(path, path+".original"); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("replaced"), 0600); err != nil {
			return err
		}
		return os.Chtimes(path, info.ModTime(), info.ModTime())
	}))
	key := backupTestKey(t)
	if _, err := BackupWorkspaceWithOptions(ctx, data, filepath.Join(root, "backup.age"), "v1", BackupWriteOptions{Recipients: []age.Recipient{key.Recipient()}}); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Fatal("equal-size/time replacement accepted", err)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("changed source produced backup")
	}
}
