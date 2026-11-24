package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkingCopyCleanupInterruptedBoundariesRetainCapacity(t *testing.T) {
	for _, boundary := range []string{"intent_saved", "before_detach", "after_detach", "detachment_saved", "before_entry_remove", "entry_removed", "empty_saved", "before_directory_remove", "directory_removed"} {
		t.Run(boundary, func(t *testing.T) {
			endpoint, store, binding, _ := workingCopyFixture(t)
			ctx := context.Background()
			child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "boundary-"+boundary))
			if err != nil {
				t.Fatal(err)
			}
			if err = child.Close(); err != nil {
				t.Fatal(err)
			}
			interrupted := errors.New("fixture lost boundary acknowledgement")
			err = store.releaseCopy(ctx, child.Record.ID, binding, func(stage, _ string) error {
				if stage == boundary {
					return interrupted
				}
				return nil
			})
			if !errors.Is(err, interrupted) {
				t.Fatal("did not exercise boundary", err)
			}
			record, err := store.Inspect(ctx, child.Record.ID, binding)
			if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes {
				t.Fatal(record, err)
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenWorkingCopies(store.directory, store.policy)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			record, err = reopened.Inspect(ctx, child.Record.ID, binding)
			if err != nil || record.State != "uncertain" || record.ReservedBytes == 0 {
				t.Fatal("restart dispatched release", record, err)
			}
			if err = reopened.Release(ctx, child.Record.ID, binding); err != nil {
				t.Fatal("explicit owned retry failed", err)
			}
			record, err = reopened.Inspect(ctx, child.Record.ID, binding)
			if err != nil || record.State != "released" || record.ReservedBytes != 0 {
				t.Fatal(record, err)
			}
		})
	}
}

func TestWorkingCopyCleanupSubstitutionBeforeDetachRetainsBothObjects(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "detach-substitution"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(store.directory, child.Record.ID)
	retained := filepath.Join(t.TempDir(), "owned-original")
	err = store.releaseCopy(ctx, child.Record.ID, binding, func(stage, _ string) error {
		if stage != "before_detach" {
			return nil
		}
		if err := os.Rename(original, retained); err != nil {
			return err
		}
		if err := os.Mkdir(original, 0700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(original, "unrelated"), []byte("preserve"), 0600)
	})
	if !errors.Is(err, ErrRevision) {
		t.Fatal("post-rename identity was not checked", err)
	}
	owner, err := store.copyOwnership(ctx, child.Record.ID)
	if err != nil || owner.Detached || !validCleanupName(owner.Quarantine) {
		t.Fatal(owner, err)
	}
	foreign := filepath.Join(store.directory, owner.Quarantine, "unrelated")
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "preserve" {
		t.Fatal("raced object was deleted", string(data), err)
	}
	if _, err = os.Stat(filepath.Join(retained, "content")); err != nil {
		t.Fatal("original object lost", err)
	}
	if err = os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	newer := filepath.Join(original, "newer-owner")
	if err = os.WriteFile(newer, []byte("also preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = store.Release(ctx, child.Record.ID, binding); !errors.Is(err, ErrRevision) {
		t.Fatal("mismatched quarantine retried", err)
	}
	for _, path := range []string{foreign, newer, filepath.Join(retained, "content")} {
		if _, err = os.Stat(path); err != nil {
			t.Fatal("cleanup rolled back over newer data", path, err)
		}
	}
}

func TestWorkingCopyCleanupRecreatedOriginalSurvivesSuccessfulDetach(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, "old-path-reused"))
	if err != nil {
		t.Fatal(err)
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(store.directory, child.Record.ID)
	sentinel := filepath.Join(original, "new-content")
	err = store.releaseCopy(ctx, child.Record.ID, binding, func(stage, _ string) error {
		if stage != "detachment_saved" {
			return nil
		}
		if err := os.Mkdir(original, 0700); err != nil {
			return err
		}
		return os.WriteFile(sentinel, []byte("new owner"), 0600)
	})
	if !errors.Is(err, ErrRevision) {
		t.Fatal("reused old pathname resolved cleanup", err)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "new owner" {
		t.Fatal(data, err)
	}
	record, err := store.Inspect(ctx, child.Record.ID, binding)
	if err != nil || record.ReservedBytes == 0 || record.State != "uncertain" {
		t.Fatal(record, err)
	}
}
