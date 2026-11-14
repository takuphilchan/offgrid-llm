package execution

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func changeSetFixture(t *testing.T) (*PreparedWorker, *WorkingCopies, SealChangeSetRequest, string) {
	t.Helper()
	e, store, b, source := workingCopyFixture(t)
	for name, content := range map[string]string{"edit.txt": "original dirty input", "delete.txt": "preserve original", "same.txt": "unchanged"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	req := workingCopyRequest(b, "changes")
	w, err := e.PrepareWorker(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	seal := SealChangeSetRequest{RequestID: "seal", CopyID: w.Record.ID, Source: b, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch}
	return w, store, seal, source
}

func TestChangeSetCapturesImmutableOperationsWithoutEditingSource(t *testing.T) {
	w, s, req, source := changeSetFixture(t)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(w.Context.Root, "edit.txt"), []byte("worker result"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(w.Context.Root, "delete.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.Context.Root, "new.txt"), []byte("worker result"), 0600); err != nil {
		t.Fatal(err)
	}
	set, err := w.SealChangeSet(ctx, req)
	if err != nil || set.State != "sealed" || set.OperationCount != 3 || set.BlobBytes != int64(len("worker result")) {
		t.Fatal(set, err)
	}
	select {
	case <-w.done:
	default:
		t.Fatal("worker not quiescent before sealing")
	}
	if actual, err := os.ReadFile(filepath.Join(source, "edit.txt")); err != nil || string(actual) != "original dirty input" {
		t.Fatal("source changed", string(actual), err)
	}
	if _, err = os.Stat(filepath.Join(source, "delete.txt")); err != nil {
		t.Fatal("source deletion", err)
	}
	page, err := s.InspectChangeSet(ctx, set.ID, req.Source, 0, 2)
	if err != nil || len(page.Operations) != 2 || page.NextOffset == nil {
		t.Fatal(page, err)
	}
	if page.Operations[0].Path != "delete.txt" || page.Operations[0].Kind != "delete" || page.Operations[1].Kind != "update" {
		t.Fatal(page.Operations)
	}
	resultHash := page.Operations[1].Result.SHA256
	if page.Operations[1].Base.Mode != page.Operations[1].Result.Mode {
		t.Fatal("private modes proposed as chmods")
	}
	page.Operations[1].Result.SHA256 = "tampered page"
	page, err = s.InspectChangeSet(ctx, set.ID, req.Source, 0, 2)
	if err != nil || page.Operations[1].Result.SHA256 != resultHash {
		t.Fatal("caller modified cache", page, err)
	}
	if err = os.WriteFile(filepath.Join(w.Context.Root, "edit.txt"), []byte("late edit"), 0600); err != nil {
		t.Fatal(err)
	}
	blob, err := s.ReadChangeBlob(ctx, set.ID, req.Source, resultHash, 0, 6)
	if err != nil || string(blob.Data) != "worker" || blob.NextOffset == nil {
		t.Fatal(blob, err)
	}
	last, err := s.ReadChangeBlob(ctx, set.ID, req.Source, resultHash, *blob.NextOffset, MaxPageBytes)
	if err != nil || string(last.Data) != " result" || last.NextOffset != nil {
		t.Fatal(last, err)
	}
	again, err := s.SealChangeSet(ctx, req)
	if err != nil || again != set {
		t.Fatal("retry rebuilt immutable result", again, err)
	}
	changed := req
	changed.ControlEpoch++
	if _, err = s.SealChangeSet(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("conflicting retry accepted", err)
	}
	foreign := req.Source
	foreign.Actor = "foreign"
	if _, err = s.ReadChangeBlob(ctx, set.ID, foreign, resultHash, 0, 8); !errors.Is(err, ErrAuthority) {
		t.Fatal("foreign artifact exposed", err)
	}
	if err = w.Release(ctx); !errors.Is(err, ErrConflict) {
		t.Fatal("retained change set deleted", err)
	}
	if err = s.AbandonChangeSet(ctx, set.ID, req.Source); err != nil {
		t.Fatal(err)
	}
	if err = w.Release(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestChangeSetBlobPagingCrossesChunksAndDetectsCorruption(t *testing.T) {
	w, s, req, _ := changeSetFixture(t)
	// Explicit fixture policy grants only temporary directories more capacity.
	s.policy.MaxCopyBytes, s.policy.MaxTotalBytes = 128<<10, 512<<10
	content := bytes.Repeat([]byte{0, 1, 2, 255, 12, 64, 9}, 7000)
	if err := os.WriteFile(filepath.Join(w.Context.Root, "binary.dat"), content, 0600); err != nil {
		t.Fatal(err)
	}
	set, err := w.SealChangeSet(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	hash := digest(content)
	page, err := s.ReadChangeBlob(context.Background(), set.ID, req.Source, hash, MaxPageBytes-3, MaxPageBytes)
	if err != nil || !bytes.Equal(page.Data, content[MaxPageBytes-3:2*MaxPageBytes-3]) {
		t.Fatal("cross-chunk page", err)
	}
	blobPath := filepath.Join(s.directory, req.CopyID, "artifacts", set.ID, hash)
	f, err := os.OpenFile(blobPath, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteAt([]byte{99}, MaxPageBytes+1)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReadChangeBlob(context.Background(), set.ID, req.Source, hash, MaxPageBytes, 10); !errors.Is(err, ErrRevision) {
		t.Fatal("corrupt chunk returned", err)
	}
	// Independent pages remain verifiable without rescanning unrelated bytes.
	if page, err = s.ReadChangeBlob(context.Background(), set.ID, req.Source, hash, 0, 10); err != nil || !bytes.Equal(page.Data, content[:10]) {
		t.Fatal(page, err)
	}
}

func TestChangeSetReservationAndFailedSealNeverReplay(t *testing.T) {
	for _, stage := range []string{"reserve", "seal"} {
		t.Run(stage, func(t *testing.T) {
			w, s, req, _ := changeSetFixture(t)
			if err := os.WriteFile(filepath.Join(w.Context.Root, "edit.txt"), []byte("output"), 0600); err != nil {
				t.Fatal(err)
			}
			trigger := `CREATE TRIGGER change_fault BEFORE INSERT ON worker_change_sets BEGIN SELECT RAISE(FAIL,'fixture'); END`
			if stage == "seal" {
				trigger = `CREATE TRIGGER change_fault BEFORE UPDATE ON worker_change_sets WHEN NEW.state='sealed' BEGIN SELECT RAISE(FAIL,'fixture'); END`
			}
			if _, err := s.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			_, err := w.SealChangeSet(context.Background(), req)
			if err == nil {
				t.Fatal("failed receipt acknowledged")
			}
			id, _ := changeSetIdentity(req)
			r, err := s.Inspect(context.Background(), req.CopyID, req.Source)
			if err != nil {
				t.Fatal(err)
			}
			if stage == "reserve" {
				if r.ArtifactReservedBytes != 0 {
					t.Fatal("failed reservation published")
				}
				if _, err = os.Stat(filepath.Join(s.directory, req.CopyID, "artifacts", id)); !os.IsNotExist(err) {
					t.Fatal("artifact written before reservation", err)
				}
			} else {
				if r.ArtifactReservedBytes != int64(len("output")) {
					t.Fatal("lost seal refunded storage", r)
				}
				again, err := s.SealChangeSet(context.Background(), req)
				if err != nil || again.State != "uncertain" {
					t.Fatal("uncertain seal replayed", again, err)
				}
			}
		})
	}
}
