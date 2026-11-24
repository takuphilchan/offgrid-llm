package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreEpochIsFreshAndInventoriedWithoutChangingOriginal(t *testing.T) {
	for _, historical := range []string{"", strings.Repeat("a", 64), "historical malformed marker"} {
		t.Run(historical, func(t *testing.T) {
			parent := t.TempDir()
			source := filepath.Join(parent, "source")
			if err := os.Mkdir(source, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "endpoint-policy.json"), []byte("historical enabled policy"), 0600); err != nil {
				t.Fatal(err)
			}
			if historical != "" {
				if err := os.WriteFile(filepath.Join(source, RestoreEpochFilename), []byte(historical), 0600); err != nil {
					t.Fatal(err)
				}
			}
			archive := filepath.Join(parent, "original.zip")
			if _, err := BackupWorkspace(context.Background(), source, archive, "0.4.3"); err != nil {
				t.Fatal(err)
			}
			restored := filepath.Join(parent, "restored")
			if _, err := RestoreWorkspace(context.Background(), archive, restored, "0.4.3"); err != nil {
				t.Fatal(err)
			}
			epoch, err := ReadRestoreEpoch(restored)
			if err != nil || len(epoch) != 64 || epoch == historical {
				t.Fatalf("restored historical authority: epoch=%q err=%v", epoch, err)
			}
			if _, err := hex.DecodeString(epoch); err != nil {
				t.Fatal(err)
			}
			if raw, err := os.ReadFile(filepath.Join(source, RestoreEpochFilename)); historical != "" && (err != nil || string(raw) != historical) || historical == "" && !os.IsNotExist(err) {
				t.Fatalf("original epoch changed: %q %v", raw, err)
			}
			nextArchive := filepath.Join(parent, "restored.zip")
			manifest, err := BackupWorkspace(context.Background(), restored, nextArchive, "0.4.3")
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256([]byte(epoch))
			found := false
			for _, item := range manifest.Files {
				if item.Path == RestoreEpochFilename {
					found = item.Size == 64 && item.SHA256 == hex.EncodeToString(digest[:])
				}
			}
			if !found {
				t.Fatal("next backup did not inventory the actual new restore epoch")
			}
			second := filepath.Join(parent, "second")
			if _, err := RestoreWorkspace(context.Background(), nextArchive, second, "0.4.3"); err != nil {
				t.Fatal(err)
			}
			secondEpoch, err := ReadRestoreEpoch(second)
			if err != nil || secondEpoch == epoch || secondEpoch == "" {
				t.Fatalf("repeated restore reused epoch: %q %v", secondEpoch, err)
			}
		})
	}
}

func TestRestoreEpochFailureCannotPublishWorkspace(t *testing.T) {
	archive := makeBackupFixture(t, "state.txt", []byte("preserved"), nil)
	target := filepath.Join(t.TempDir(), "restored")
	failure := errors.New("epoch boundary interrupted")
	ctx := context.WithValue(context.Background(), backupBoundaryKey{}, backupBoundaryHook(func(stage, path string) error {
		if stage == "restore_epoch_written" {
			if epoch, err := ReadRestoreEpoch(path); err != nil || len(epoch) != 64 {
				t.Fatalf("epoch was not synced before boundary: %q %v", epoch, err)
			}
			return failure
		}
		return nil
	}))
	if _, err := RestoreWorkspace(ctx, archive, target, "0.4.3"); !errors.Is(err, failure) {
		t.Fatalf("lost epoch boundary failure: %v", err)
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Fatalf("interrupted epoch published target: %v", err)
	}
	if _, err := VerifyBackup(context.Background(), archive); err != nil {
		t.Fatal("source archive changed", err)
	}
}

func TestRestoreEpochReaderRejectsMalformedOrSpecialState(t *testing.T) {
	root := t.TempDir()
	if epoch, err := ReadRestoreEpoch(root); err != nil || epoch != "" {
		t.Fatal(epoch, err)
	}
	path := filepath.Join(root, RestoreEpochFilename)
	for _, value := range []string{"", strings.Repeat("A", 64), strings.Repeat("x", 64), strings.Repeat("a", 65)} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadRestoreEpoch(root); err == nil {
			t.Fatal("malformed epoch accepted")
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRestoreEpoch(root); err == nil {
		t.Fatal("directory accepted as epoch")
	}
}

func TestWorkspaceActivationEpochIsNonemptyStableAndDoesNotRepairCorruption(t *testing.T) {
	dir := t.TempDir()
	first, err := EnsureWorkspaceActivationEpoch(dir)
	if err != nil || len(first) != 64 {
		t.Fatal(first, err)
	}
	second, err := EnsureWorkspaceActivationEpoch(dir)
	if err != nil || second != first {
		t.Fatal("ordinary review rotated activation", second, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("activation creation leaked temporary state", entries, err)
	}
	path := filepath.Join(dir, RestoreEpochFilename)
	if err := os.WriteFile(path, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureWorkspaceActivationEpoch(dir); err == nil {
		t.Fatal("corrupt activation marker silently replaced")
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "corrupt" {
		t.Fatal("corrupt original was not retained", string(raw), err)
	}
}
