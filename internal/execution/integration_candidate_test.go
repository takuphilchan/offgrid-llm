package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// All commands use the owned test helper in temporary private copies.
func integrationCandidateFixture(t *testing.T) (*LocalEndpoint, *WorkingCopies, Binding, string, *PreparedWorker, SealChangeSetRequest) {
	return integrationCandidateFixtureWith(t, nil, nil)
}

func integrationCandidateFixtureWith(t *testing.T, setup, mutate func(string)) (*LocalEndpoint, *WorkingCopies, Binding, string, *PreparedWorker, SealChangeSetRequest) {
	t.Helper()
	requireIntegrationReplacementBackend(t)
	e, store, binding, source := workingCopyFixture(t)
	store.policy.MaxTotalBytes = 12 * 1024
	ctx := context.Background()
	writeCandidateFile(t, source, "input.txt", "dirty original")
	writeCandidateFile(t, source, "remove.txt", "retain until integration")
	if setup != nil {
		setup(source)
	}
	editReq := workingCopyRequest(binding, "editor")
	edit, err := e.PrepareWorker(ctx, editReq)
	if err != nil {
		t.Fatal(err)
	}
	writeCandidateFile(t, edit.Context.Root, "input.txt", "candidate input")
	writeCandidateFile(t, edit.Context.Root, "new/dependency.txt", "new input")
	if err = os.Remove(filepath.Join(edit.Context.Root, "remove.txt")); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(edit.Context.Root)
	}
	editSet := sealCandidate(t, edit, editReq)
	otherReq := workingCopyRequest(binding, "independent")
	other, err := e.PrepareWorker(ctx, otherReq)
	if err != nil {
		t.Fatal(err)
	}
	writeCandidateFile(t, other.Context.Root, "independent.txt", "independent input")
	otherSet := sealCandidate(t, other, otherReq)
	testReq := workingCopyRequest(binding, "tester")
	testReq.Purpose = "test"
	testReq.Prerequisites = []ChangeSetReference{candidateReference(editSet), candidateReference(otherSet)}
	w, err := e.PrepareWorker(ctx, testReq)
	if err != nil {
		t.Fatal(err)
	}
	intent := request(w.Endpoint.Worker, w.Context.Binding, "process_start", "validate", ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestValidationCommandFixture$", "--", "offgrid-validation-fixture", "pass"}, TimeoutSeconds: 10})
	intent.Approved = true
	result := w.Endpoint.Worker.Handle(ctx, intent)
	if result.Error != "" || result.Receipt == nil || result.Receipt.State != "dispatched" {
		t.Fatal(result)
	}
	w.Endpoint.Worker.finishing.Wait()
	seal := SealChangeSetRequest{RequestID: "seal", CopyID: w.Record.ID, Source: binding, InstructionVersion: testReq.InstructionVersion, ControlEpoch: testReq.ControlEpoch, MaxReservedBytes: 2048}
	set, err := w.SealChangeSet(ctx, seal)
	if err != nil || set.Validation.State != "passed" {
		t.Fatal(set, err)
	}
	return e, store, binding, source, w, seal
}

func TestIntegrationCandidatePreservesCombinedInputsAndRetry(t *testing.T) {
	e, store, binding, source, w, req := integrationCandidateFixture(t)
	ctx := context.Background()
	set, err := store.PrepareIntegrationCandidate(ctx, req)
	if err != nil || set.State != "ready" || set.OperationCount != 5 || set.Validation.State != "passed" || set.ResultManifestDigest != w.Record.ManifestDigest {
		t.Fatal(set, err)
	}
	page, err := store.InspectChangeSet(ctx, set.ID, binding, 0, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range page.Operations {
		if change.Path == "validation.log" {
			t.Fatal("test log proposed for integration")
		}
	}
	// Reuse the combined candidate as one prerequisite, without applying its
	// contributing edits twice or including the test worker's generated logs.
	nextReq := workingCopyRequest(binding, "next")
	nextReq.Prerequisites = []ChangeSetReference{candidateReference(set)}
	next, err := e.PrepareWorker(ctx, nextReq)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"input.txt": "candidate input", "new/dependency.txt": "new input", "independent.txt": "independent input"} {
		data, err := os.ReadFile(filepath.Join(next.Context.Root, name))
		if err != nil || string(data) != want {
			t.Fatal(name, string(data), err)
		}
	}
	for _, name := range []string{"validation.log", "remove.txt"} {
		if _, err = os.Stat(filepath.Join(next.Context.Root, name)); !os.IsNotExist(err) {
			t.Fatal("unexpected candidate file", name, err)
		}
	}
	writeCandidateFile(t, w.Context.Root, "input.txt", "late private edit")
	again, err := store.PrepareIntegrationCandidate(ctx, req)
	if err != nil || again != set {
		t.Fatal("retry recaptured files", again, err)
	}
	changed := req
	changed.MaxReservedBytes++
	if _, err = store.PrepareIntegrationCandidate(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("conflicting retry accepted", err)
	}
	data, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "dirty original" {
		t.Fatal("original changed", string(data), err)
	}
	if err = e.Close(); err != nil {
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
	again, err = reopened.PrepareIntegrationCandidate(ctx, req)
	if err != nil || again != set {
		t.Fatal("restart lost exact candidate", again, err)
	}
}

func TestIntegrationCandidateFaultsNeverBecomeReady(t *testing.T) {
	for _, mode := range []string{"changed-input", "capacity", "write-receipt", "final-receipt", "revoked-evidence"} {
		t.Run(mode, func(t *testing.T) {
			e, store, binding, _, w, req := integrationCandidateFixture(t)
			ctx := context.Background()
			switch mode {
			case "changed-input":
				writeCandidateFile(t, w.Context.Root, "input.txt", "changed after validation")
			case "capacity":
				req.MaxReservedBytes = 1
			case "write-receipt":
				if _, err := store.db.Exec(`CREATE TRIGGER candidate_fault BEFORE INSERT ON worker_change_blobs BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
					t.Fatal(err)
				}
			case "final-receipt":
				if _, err := store.db.Exec(`CREATE TRIGGER candidate_fault BEFORE UPDATE OF state ON worker_change_sets WHEN NEW.state='ready' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
					t.Fatal(err)
				}
			case "revoked-evidence":
				if _, err := store.db.Exec(`UPDATE worker_change_sets SET state='abandoned' WHERE copy_id!=?`, w.Record.ID); err != nil {
					t.Fatal(err)
				}
			}
			candidate, err := store.PrepareIntegrationCandidate(ctx, req)
			if err == nil || candidate.State == "ready" {
				t.Fatal("fault qualified candidate", candidate, err)
			}
			if mode == "write-receipt" || mode == "final-receipt" {
				if candidate.State != "uncertain" {
					t.Fatal(candidate)
				}
				if _, err = store.db.Exec(`DROP TRIGGER candidate_fault`); err != nil {
					t.Fatal(err)
				}
				if err = e.Close(); err != nil {
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
				again, err := reopened.PrepareIntegrationCandidate(ctx, req)
				if err != nil || again.State != "uncertain" || again.ID != candidate.ID {
					t.Fatal("uncertain candidate replayed", again, err)
				}
				record, err := reopened.Inspect(ctx, w.Record.ID, binding)
				if err != nil || record.ArtifactReservedBytes < candidate.BlobBytes {
					t.Fatal("unknown storage refunded", record, err)
				}
			}
		})
	}
}
