package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestChangeSetReopenRetainsResultsAndNeverReplaysInterruptedStages(t *testing.T) {
	for _, state := range []string{"sealed", "draft", "validating", "applying"} {
		t.Run(state, func(t *testing.T) {
			w, store, req, source := changeSetFixture(t)
			ctx := context.Background()
			const output = "retained immutable output"
			if err := os.WriteFile(filepath.Join(w.Context.Root, "edit.txt"), []byte(output), 0600); err != nil {
				t.Fatal(err)
			}
			set, err := w.SealChangeSet(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if state != "sealed" {
				set.State = state
				set.IntegrationStarted = state == "applying"
				header, _ := json.Marshal(set)
				if _, err = store.db.Exec(`UPDATE worker_change_sets SET state=?,header=? WHERE id=?`, state, header, set.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err = store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenWorkingCopies(store.directory, store.policy)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			retry, err := reopened.SealChangeSet(ctx, req)
			want := "uncertain"
			if state == "sealed" {
				want = "sealed"
			}
			if err != nil || retry.State != want || retry.ID != set.ID {
				t.Fatal("restart repeated capture", retry, err)
			}
			page, err := reopened.ReadChangeBlob(ctx, set.ID, req.Source, digest([]byte(output)), 0, MaxPageBytes)
			if state == "sealed" && (err != nil || string(page.Data) != output) {
				t.Fatal("sealed result unavailable", page, err)
			}
			if state != "sealed" && err == nil {
				t.Fatal("uncertain result treated as sealed")
			}
			if state == "applying" {
				if err = reopened.AbandonChangeSet(ctx, set.ID, req.Source); !errors.Is(err, ErrConflict) {
					t.Fatal("potential effects abandoned", err)
				}
			}
			original, _ := os.ReadFile(filepath.Join(source, "edit.txt"))
			if string(original) != "original dirty input" {
				t.Fatal("recovery mutated original")
			}
		})
	}
}

func TestChangeSetUsesClosedCopyHeadroomBeforeChargingMoreStorage(t *testing.T) {
	w, store, req, _ := changeSetFixture(t)
	store.policy.MaxTotalBytes = 1024
	data := bytes.Repeat([]byte("x"), 250)
	if err := os.WriteFile(filepath.Join(w.Context.Root, "edit.txt"), data, 0600); err != nil {
		t.Fatal(err)
	}
	set, err := w.SealChangeSet(context.Background(), req)
	if err != nil || set.State != "sealed" {
		t.Fatal(set, err)
	}
	record, err := store.Measure(context.Background(), req.CopyID, req.Source)
	if err != nil || record.ReservedBytes != 1024 || record.ArtifactReservedBytes != 250 || record.MeasuredBytes <= record.ContentBytes {
		t.Fatal("closed-copy headroom charged twice", record, err)
	}
}

func TestChangeSetCannotExceedServiceCaptureReservation(t *testing.T) {
	w, store, req, _ := changeSetFixture(t)
	req.MaxReservedBytes = 1024
	if err := os.WriteFile(filepath.Join(w.Context.Root, "edit.txt"), bytes.Repeat([]byte("x"), 600), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.SealChangeSet(context.Background(), req); !errors.Is(err, ErrLimit) {
		t.Fatal("capture exceeded root allowance", err)
	}
	record, err := store.Inspect(context.Background(), req.CopyID, req.Source)
	if err != nil || record.ArtifactReservedBytes != 0 {
		t.Fatal("unreserved artifact bytes written", record, err)
	}
}

func TestChangeSetRejectsUnrepresentableWorkerChanges(t *testing.T) {
	for _, kind := range []string{"git", "permissions"} {
		t.Run(kind, func(t *testing.T) {
			if kind == "permissions" && runtime.GOOS == "windows" {
				t.Skip("Unix mode changes are not represented by Windows mode bits")
			}
			w, _, req, _ := changeSetFixture(t)
			if kind == "git" {
				if err := os.Mkdir(filepath.Join(w.Context.Root, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(w.Context.Root, ".git", "index"), []byte("worker metadata"), 0600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Chmod(filepath.Join(w.Context.Root, "edit.txt"), 0400); err != nil {
				t.Fatal(err)
			}
			if _, err := w.SealChangeSet(context.Background(), req); !errors.Is(err, ErrUnsupported) {
				t.Fatal("unsupported changes silently omitted", err)
			}
		})
	}
}
