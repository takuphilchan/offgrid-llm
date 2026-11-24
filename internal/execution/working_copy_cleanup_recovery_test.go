package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingCopyCleanupOwnershipSurvivesRenameAndRestart(t *testing.T) {
	endpoint, store, binding, source := workingCopyFixture(t)
	if err := os.WriteFile(filepath.Join(source, "input"), []byte("retained input"), 0600); err != nil {
		t.Fatal(err)
	}
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "renamed-cleanup"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := store.copyOwnership(context.Background(), child.Record.ID)
	if err != nil || before.Identity == "" {
		t.Fatal("missing durable native identity", before, err)
	}
	original := filepath.Join(store.directory, child.Record.ID)
	retained := filepath.Join(t.TempDir(), "retained-original")
	if err = os.Rename(original, retained); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(original, "unrelated")
	if err = os.WriteFile(sentinel, []byte("new directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWorkingCopies(store.directory, store.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err = os.Stat(sentinel); err != nil {
		t.Fatal("restart dispatched cleanup", err)
	}
	if err = reopened.Release(context.Background(), child.Record.ID, binding); !errors.Is(err, ErrRevision) {
		t.Fatal("recreated directory was adopted after restart", err)
	}
	if data, err := os.ReadFile(filepath.Join(retained, "content", "input")); err != nil || string(data) != "retained input" {
		t.Fatal("original owned input was lost", string(data), err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "new directory" {
		t.Fatal("replacement content was removed", string(data), err)
	}
	record, err := reopened.Inspect(context.Background(), child.Record.ID, binding)
	if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes {
		t.Fatal("ownership uncertainty refunded capacity", record, err)
	}
	// Only the fixture's known sentinel and empty replacement are removed here.
	// Restoring the original object proves identity survives rename and restart.
	if err = os.Remove(sentinel); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(original); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(retained, original); err != nil {
		t.Fatal(err)
	}
	after, err := reopened.copyOwnership(context.Background(), child.Record.ID)
	if err != nil || after != before {
		t.Fatal("native identity changed through bookkeeping", after, before, err)
	}
	if err = reopened.Release(context.Background(), child.Record.ID, binding); err != nil {
		t.Fatal("original object could not be explicitly released", err)
	}
	if _, err = reopened.Manifest(context.Background(), child.Record.ID, binding); err != nil {
		t.Fatal("cleanup erased the retained input manifest", err)
	}
}

func TestWorkingCopyCleanupLegacyOwnershipIsNotInvented(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "legacy-cleanup"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`DROP TABLE worker_copy_ownership; PRAGMA user_version=10;`); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWorkingCopies(store.directory, store.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err = reopened.Manifest(context.Background(), child.Record.ID, binding); err != nil {
		t.Fatal("migration lost legacy input evidence", err)
	}
	if err = reopened.Release(context.Background(), child.Record.ID, binding); !errors.Is(err, ErrUnsupported) {
		t.Fatal("legacy pathname became ownership proof", err)
	}
	if _, err = os.Stat(child.Context.Root); err != nil {
		t.Fatal("legacy contents were deleted", err)
	}
	var count, version int
	if err = reopened.db.QueryRow(`SELECT COUNT(*) FROM worker_copy_ownership`).Scan(&count); err != nil || count != 0 {
		t.Fatal("legacy ownership was synthesized", count, err)
	}
	if err = reopened.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 12 {
		t.Fatal("unsafe old cleanup implementation can reopen schema", version, err)
	}
}

func TestWorkingCopyCleanupPublishedReceiptDoesNotTouchRecreatedPath(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	child, err := endpoint.PrepareWorker(context.Background(), workingCopyRequest(binding, "completed-cleanup"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	recreated := filepath.Join(store.directory, child.Record.ID)
	if err = os.Mkdir(recreated, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(recreated, "later-owner")
	if err = os.WriteFile(sentinel, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = store.Release(context.Background(), child.Record.ID, binding); err != nil {
		t.Fatal("completed receipt became a new cleanup", err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "preserve" {
		t.Fatal("completed retry deleted unrelated contents", string(data), err)
	}
}
