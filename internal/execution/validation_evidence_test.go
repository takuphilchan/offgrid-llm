package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Native side effects are limited to this helper's temporary assigned copy.
func TestValidationCommandFixture(t *testing.T) {
	i := slices.Index(os.Args, "offgrid-validation-fixture")
	if i < 0 {
		return
	}
	mode := os.Args[i+1]
	want := "candidate input"
	if mode == "follow-up" {
		want = "follow-up based on current source"
	}
	data, err := os.ReadFile("input.txt")
	if err != nil || string(data) != want {
		os.Exit(9)
	}
	if mode == "fail" {
		os.Exit(7)
	}
	if mode == "mutate" {
		if os.WriteFile("input.txt", []byte("changed while testing"), 0600) != nil {
			os.Exit(8)
		}
	}
	if os.WriteFile("validation.log", []byte("owned command ran"), 0600) != nil {
		os.Exit(6)
	}
	os.Exit(0)
}

func TestValidationEvidenceUsesNativeReceiptsAndExactInputs(t *testing.T) {
	for _, mode := range []string{"pass", "fail", "mutate", "not-run", "unknown", "unmeasured", "changed-before", "late-change"} {
		t.Run(mode, func(t *testing.T) {
			e, store, binding, source := workingCopyFixture(t)
			ctx := context.Background()
			writeCandidateFile(t, source, "input.txt", "candidate input")
			req := workingCopyRequest(binding, "validate")
			req.Purpose = "test"
			w, err := e.PrepareWorker(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "changed-before" {
				writeCandidateFile(t, w.Context.Root, "input.txt", "changed before dispatch")
			}
			commandMode := "pass"
			if mode == "fail" || mode == "mutate" {
				commandMode = mode
			}
			intent := request(w.Endpoint.Worker, w.Context.Binding, "process_start", "test-command", ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestValidationCommandFixture$", "--", "offgrid-validation-fixture", commandMode}, TimeoutSeconds: 10})
			intent.Approved = true // Exact fixture executable, never general approval.
			if mode != "not-run" {
				result := w.Endpoint.Worker.Handle(ctx, intent)
				if mode == "changed-before" {
					if result.Receipt == nil || result.Receipt.State != "rejected" {
						t.Fatal("changed input dispatched", result)
					}
					if _, err = os.Stat(filepath.Join(w.Context.Root, "validation.log")); !os.IsNotExist(err) {
						t.Fatal("rejected command ran")
					}
				} else if result.Error != "" || result.Receipt == nil || result.Receipt.State != "dispatched" {
					t.Fatal(result)
				}
				w.Endpoint.Worker.finishing.Wait()
			}
			if mode == "unknown" {
				if _, err = w.Endpoint.Worker.Journal.db.Exec(`UPDATE execution_operations SET state='uncertain',result=NULL WHERE id=?`, intent.OperationID); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unmeasured" {
				receipt, err := w.Endpoint.Worker.Journal.Inspect(ctx, intent)
				if err != nil {
					t.Fatal(err)
				}
				var result ProcessResult
				if json.Unmarshal(receipt.Result, &result) != nil {
					t.Fatal("bad receipt")
				}
				result.Runtime = nil
				data, _ := json.Marshal(result)
				if _, err = w.Endpoint.Worker.Journal.db.Exec(`UPDATE execution_operations SET result=? WHERE id=?`, data, intent.OperationID); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "late-change" {
				if err = w.Close(); err != nil {
					t.Fatal(err)
				}
				writeCandidateFile(t, w.Context.Root, "input.txt", "changed after close")
			}
			set := sealCandidate(t, w, req)
			want := map[string]string{"pass": "passed", "fail": "failed", "mutate": "inputs_changed", "not-run": "not_run", "unknown": "uncertain", "unmeasured": "uncertain", "changed-before": "inputs_changed", "late-change": "inputs_changed"}[mode]
			if set.Validation.Version != 1 || set.Validation.State != want || set.Validation.InputManifestDigest != w.Record.ManifestDigest || !set.Validation.ValidFor(w.Record) {
				t.Fatal("invalid qualification", set.Validation, want)
			}
			page, err := e.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "validation", Source: binding, ChangeSetID: set.ID, Limit: 1})
			if err != nil || page.Validation == nil || page.Validation.Summary != set.Validation {
				t.Fatal(page, err)
			}
			count := 1
			if mode == "not-run" || mode == "changed-before" {
				count = 0
			}
			if len(page.Validation.Checks) != count {
				t.Fatal("checks not retained", page.Validation)
			}
			if count == 1 && (page.Validation.Checks[0].Command.Program != os.Args[0] || len(page.Validation.Checks[0].RequestDigest) != 64 || len(page.Validation.Checks[0].ReceiptDigest) != 64) {
				t.Fatal("missing exact process provenance")
			}
			foreign := binding
			foreign.Actor = "foreign"
			if _, err = e.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "validation", Source: foreign, ChangeSetID: set.ID, Limit: 1}); !errors.Is(err, ErrAuthority) {
				t.Fatal("foreign validation exposed", err)
			}
			if mode == "pass" {
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
				reopened.mu.Lock()
				retained, err := reopened.inspectValidationLocked(ctx, set.ID, binding, 0, 1)
				reopened.mu.Unlock()
				if err != nil || retained.Summary != set.Validation || len(retained.Checks) != 1 {
					t.Fatal("restart lost validation", retained, err)
				}
			}
			data, err := os.ReadFile(filepath.Join(source, "input.txt"))
			if err != nil || string(data) != "candidate input" {
				t.Fatal("test changed original", string(data), err)
			}
		})
	}
}

func TestValidationCapturePersistenceFailureIsNotPassed(t *testing.T) {
	e, store, binding, _ := workingCopyFixture(t)
	req := workingCopyRequest(binding, "validate")
	req.Purpose = "test"
	w, err := e.PrepareWorker(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.db.Exec(`CREATE TRIGGER reject_validation BEFORE INSERT ON worker_validations BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = w.SealChangeSet(context.Background(), SealChangeSetRequest{RequestID: "seal", CopyID: w.Record.ID, Source: binding, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch}); err == nil {
		t.Fatal("failed receipt persistence qualified output")
	}
	var sets int
	if err = store.db.QueryRow(`SELECT COUNT(*) FROM worker_change_sets WHERE copy_id=?`, w.Record.ID).Scan(&sets); err != nil || sets != 0 {
		t.Fatal("failed validation produced a sealed set", sets, err)
	}
}
