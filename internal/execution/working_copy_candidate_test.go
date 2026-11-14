package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func candidateReference(set ChangeSet) ChangeSetReference {
	return ChangeSetReference{ID: set.ID, OperationsDigest: set.OperationsDigest, InstructionVersion: set.InstructionVersion, ControlEpoch: set.ControlEpoch}
}

func sealCandidate(t *testing.T, w *PreparedWorker, req WorkingCopyRequest) ChangeSet {
	t.Helper()
	set, err := w.SealChangeSet(context.Background(), SealChangeSetRequest{RequestID: "seal", CopyID: w.Record.ID, Source: req.Parent, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func writeCandidateFile(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkingCopyCandidateCombinesAndRetainsTransitiveInputs(t *testing.T) {
	e, store, binding, source := workingCopyFixture(t)
	store.policy.MaxTotalBytes = 12 * 1024
	ctx := context.Background()
	writeCandidateFile(t, source, "a.txt", "dirty a")
	writeCandidateFile(t, source, "b.txt", "dirty b")
	writeCandidateFile(t, source, "remove.txt", "retained original")
	firstReq := workingCopyRequest(binding, "first")
	first, err := e.PrepareWorker(ctx, firstReq)
	if err != nil {
		t.Fatal(err)
	}
	writeCandidateFile(t, first.Context.Root, "a.txt", "edited a")
	writeCandidateFile(t, first.Context.Root, "new/dependency.txt", "created input")
	if err = os.Remove(filepath.Join(first.Context.Root, "remove.txt")); err != nil {
		t.Fatal(err)
	}
	firstSet := sealCandidate(t, first, firstReq)
	// Altering an old private directory cannot substitute bytes after capture.
	writeCandidateFile(t, first.Context.Root, "a.txt", "late unrelated write")
	secondReq := workingCopyRequest(binding, "independent")
	second, err := e.PrepareWorker(ctx, secondReq)
	if err != nil {
		t.Fatal(err)
	}
	writeCandidateFile(t, second.Context.Root, "b.txt", "edited b")
	secondSet := sealCandidate(t, second, secondReq)
	testReq := workingCopyRequest(binding, "test")
	testReq.Prerequisites = []ChangeSetReference{candidateReference(firstSet), candidateReference(secondSet)}
	testReq.RequiredPaths = []string{"new/dependency.txt"}
	tester, err := e.PrepareWorker(ctx, testReq)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"a.txt": "edited a", "b.txt": "edited b", "new/dependency.txt": "created input"} {
		data, err := os.ReadFile(filepath.Join(tester.Context.Root, name))
		if err != nil || string(data) != want {
			t.Fatalf("candidate %s = %q: %v", name, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(tester.Context.Root, "remove.txt")); !os.IsNotExist(err) {
		t.Fatal("candidate retained deleted input", err)
	}
	var encoded []byte
	if err = store.db.QueryRow(`SELECT manifest FROM worker_copies WHERE id=?`, tester.Record.ID).Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	var manifest WorkingCopyManifest
	if json.Unmarshal(encoded, &manifest) != nil || len(manifest.Prerequisites) != 2 || digest(encoded) != tester.Record.ManifestDigest {
		t.Fatal("candidate provenance missing")
	}
	testSet := sealCandidate(t, tester, testReq)
	if testSet.OperationCount != 0 {
		t.Fatal("prerequisite edits attributed to test worker", testSet)
	}
	// A downstream assignment needs only the immediate result. Its immutable
	// input manifest retains the transitive edits, each applied exactly once.
	lastReq := workingCopyRequest(binding, "transitive")
	lastReq.Prerequisites = []ChangeSetReference{candidateReference(firstSet), candidateReference(testSet)}
	last, err := e.PrepareWorker(ctx, lastReq)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(last.Context.Root, "b.txt"))
	if err != nil || string(data) != "edited b" {
		t.Fatal("transitive input lost", string(data), err)
	}
	writeCandidateFile(t, last.Context.Root, "a.txt", "follow-on edit")
	lastSet := sealCandidate(t, last, lastReq)
	if lastSet.OperationCount != 1 {
		t.Fatal(lastSet)
	}
	finalReq := workingCopyRequest(binding, "final")
	finalReq.Prerequisites = []ChangeSetReference{candidateReference(lastSet)}
	final, err := e.PrepareWorker(ctx, finalReq)
	if err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(final.Context.Root, "a.txt"))
	if err != nil || string(data) != "follow-on edit" {
		t.Fatal(string(data), err)
	}
	for name, want := range map[string]string{"a.txt": "dirty a", "b.txt": "dirty b", "remove.txt": "retained original"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil || string(data) != want {
			t.Fatal("original changed", name, string(data), err)
		}
	}
}

func TestWorkingCopyCandidateRejectsStaleConflictingAndCorruptInputs(t *testing.T) {
	for _, scenario := range []string{"user-edit", "overlap", "digest", "version", "epoch", "abandoned", "blob", "capacity", "foreign", "deleted-parent"} {
		t.Run(scenario, func(t *testing.T) {
			e, store, binding, source := workingCopyFixture(t)
			store.policy.MaxTotalBytes = 6 * 1024
			ctx := context.Background()
			writeCandidateFile(t, source, "input.txt", "original")
			writeCandidateFile(t, source, "dir/old.txt", "original child")
			editReq := workingCopyRequest(binding, "edit")
			worker, err := e.PrepareWorker(ctx, editReq)
			if err != nil {
				t.Fatal(err)
			}
			writeCandidateFile(t, worker.Context.Root, "input.txt", "changed")
			if scenario == "deleted-parent" {
				if err = os.Remove(filepath.Join(worker.Context.Root, "dir/old.txt")); err != nil {
					t.Fatal(err)
				}
				if err = os.Remove(filepath.Join(worker.Context.Root, "dir")); err != nil {
					t.Fatal(err)
				}
			}
			set := sealCandidate(t, worker, editReq)
			req := workingCopyRequest(binding, "dependent")
			req.Prerequisites = []ChangeSetReference{candidateReference(set)}
			want := ErrRevision
			switch scenario {
			case "user-edit":
				writeCandidateFile(t, source, "input.txt", "new user content")
				want = ErrConflict
			case "overlap":
				otherReq := workingCopyRequest(binding, "other")
				other, err := e.PrepareWorker(ctx, otherReq)
				if err != nil {
					t.Fatal(err)
				}
				writeCandidateFile(t, other.Context.Root, "input.txt", "another result")
				req.Prerequisites = append(req.Prerequisites, candidateReference(sealCandidate(t, other, otherReq)))
				want = ErrConflict
			case "digest":
				req.Prerequisites[0].OperationsDigest = strings.Repeat("0", 64)
			case "version":
				req.Prerequisites[0].InstructionVersion++
			case "epoch":
				req.Prerequisites[0].ControlEpoch++
			case "abandoned":
				if err = store.AbandonChangeSet(ctx, set.ID, binding); err != nil {
					t.Fatal(err)
				}
			case "blob":
				writeCandidateFile(t, store.directory, set.CopyID+"/artifacts/"+set.ID+"/"+digest([]byte("changed")), "corrupt")
			case "capacity":
				req.MaxBytes = 2
				want = ErrLimit
			case "foreign":
				otherBinding := binding
				otherBinding.Job = "another-root"
				if err = e.Admit(ctx, otherBinding); err != nil {
					t.Fatal(err)
				}
				req.Parent = otherBinding
				want = ErrAuthority
			case "deleted-parent":
				writeCandidateFile(t, source, "dir/new.txt", "new user input")
				want = ErrConflict
			}
			if _, err = e.PrepareWorker(ctx, req); !errors.Is(err, want) {
				t.Fatalf("wanted %v, got %v", want, err)
			}
			var ready int
			if err = store.db.QueryRow(`SELECT COUNT(*) FROM worker_copies WHERE json_extract(record,'$.worker_run_id')=? AND state='ready'`, req.WorkerRunID).Scan(&ready); err != nil || ready != 0 {
				t.Fatal("failed candidate admitted", ready, err)
			}
			wantContent := "original"
			if scenario == "user-edit" {
				wantContent = "new user content"
			}
			data, err := os.ReadFile(filepath.Join(source, "input.txt"))
			if err != nil || string(data) != wantContent {
				t.Fatal("original modified", string(data), err)
			}
		})
	}
}
