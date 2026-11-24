package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingCopyOwnershipPublicationPrecedesCopiedBytes(t *testing.T) {
	endpoint, store, binding, source := workingCopyFixture(t)
	if err := os.WriteFile(filepath.Join(source, "input"), []byte("must not copy"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`CREATE TRIGGER ignore_ownership BEFORE UPDATE OF identity ON worker_copy_ownership BEGIN SELECT RAISE(IGNORE); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "unpublished-owner")); !errors.Is(err, ErrRevision) {
		t.Fatal("ignored ownership publication admitted worker", err)
	}
	var id string
	if err := store.db.QueryRow(`SELECT id FROM worker_copies`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.root.Lstat(id + "/content"); !os.IsNotExist(err) {
		t.Fatal("input bytes may have been copied without durable ownership", err)
	}
	if err := store.Release(context.Background(), id, binding); !errors.Is(err, ErrRevision) {
		t.Fatal("unpublished directory was adopted by cleanup", err)
	}
}

func TestWorkingCopyCleanupCancellationRetainsRemainingContents(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "partial-cleanup"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(store.directory, child.Record.ID)
	removed, retained := filepath.Join(root, "z-observe-removal"), filepath.Join(root, "y-retain-on-cancel")
	for _, name := range []string{removed, retained} {
		if err = os.WriteFile(name, []byte("owned fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	quarantine := ""
	err = store.releaseCopy(ctx, child.Record.ID, binding, func(stage, name string) error {
		if stage == "detachment_saved" {
			quarantine = name
		}
		if stage == "entry_removed" && name == "z-observe-removal" {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cleanup ignored cancellation after its first removal", err)
	}
	if !validCleanupName(quarantine) {
		t.Fatal("cleanup did not persist its private quarantine")
	}
	removed = filepath.Join(store.directory, quarantine, "z-observe-removal")
	retained = filepath.Join(store.directory, quarantine, "y-retain-on-cancel")
	if _, err = os.Stat(removed); !os.IsNotExist(err) {
		t.Fatal("fixture did not exercise partial deletion", err)
	}
	if _, err = os.Stat(retained); err != nil {
		t.Fatal("cleanup continued deleting after cancellation", err)
	}
	record, err := store.Inspect(context.Background(), child.Record.ID, binding)
	if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes {
		t.Fatal("partial cleanup refunded capacity", record, err)
	}
	if err = store.Release(context.Background(), child.Record.ID, binding); err != nil {
		t.Fatal("explicit continuation could not finish the original owned root", err)
	}
}

func TestWorkingCopyOversizedCleanupRejectsBeforeDeletion(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "oversized-cleanup"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if err = os.WriteFile(filepath.Join(child.Context.Root, fmt.Sprintf("extra-%03d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	policy := store.policy
	policy.MaxEntries = 1
	reopened, err := OpenWorkingCopies(store.directory, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err = reopened.Release(context.Background(), child.Record.ID, binding); !errors.Is(err, ErrLimit) {
		t.Fatal("oversized cleanup was not bounded", err)
	}
	ownership, err := reopened.copyOwnership(context.Background(), child.Record.ID)
	if err != nil || !ownership.Detached || !validCleanupName(ownership.Quarantine) {
		t.Fatal("missing retained cleanup ownership", ownership, err)
	}
	entries, err := os.ReadDir(filepath.Join(store.directory, ownership.Quarantine, "content"))
	if err != nil || len(entries) != 200 {
		t.Fatal("failed cleanup inventory deleted content", len(entries), err)
	}
}
