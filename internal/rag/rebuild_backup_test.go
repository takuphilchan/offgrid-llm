package rag

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

type redirectRebuildAlias struct {
	recipient age.Recipient
	redirect  func() error
}

func (r redirectRebuildAlias) Wrap(key []byte) ([]*age.Stanza, error) {
	if err := r.redirect(); err != nil {
		return nil, err
	}
	return r.recipient.Wrap(key)
}

func TestRebuildBackupBindsCanonicalOwnedDirectory(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "original")
	other := filepath.Join(root, "unrelated")
	alias := filepath.Join(root, "alias")
	for _, dir := range []string{source, other} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(source, alias); err != nil {
		t.Skip("directory symlink unavailable:", err)
	}
	key, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	options := storage.BackupWriteOptions{Recipients: []age.Recipient{redirectRebuildAlias{key.Recipient(), func() error {
		if err := os.Remove(alias); err != nil {
			return err
		}
		return os.Symlink(other, alias)
	}}}}
	called := false
	_, err = rebuildWithRecoveryBackup(context.Background(), alias, filepath.Join(root, "backup.age"), "9.9.9", options, func(_ context.Context, owned string) error {
		called = true
		if owned != source {
			t.Fatalf("rebuild retargeted from %s to %s", source, owned)
		}
		if owner, err := storage.AcquireOwnership(source); !errors.Is(err, storage.ErrWorkspaceInUse) {
			if owner != nil {
				owner.Close()
			}
			t.Fatal("rebuild lost original ownership", err)
		}
		owner, err := storage.AcquireOwnership(other)
		if err != nil {
			t.Fatal("unrelated workspace unexpectedly locked", err)
		}
		owner.Close()
		return nil
	})
	if err != nil || !called {
		t.Fatal("owned rebuild callback failed", err)
	}
}
