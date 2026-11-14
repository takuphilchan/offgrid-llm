package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func integrationFixture(t *testing.T) (*LocalEndpoint, *WorkingCopies, Binding, string, ChangeSet, IntegrateChangeSetRequest) {
	t.Helper()
	e, store, binding, source, _, seal := integrationCandidateFixture(t)
	set, err := store.PrepareIntegrationCandidate(context.Background(), seal)
	if err != nil {
		t.Fatal(err)
	}
	req := IntegrateChangeSetRequest{RequestID: "integrate", Source: binding, ChangeSetID: set.ID, OperationsDigest: set.OperationsDigest, InstructionVersion: set.InstructionVersion, ControlEpoch: set.ControlEpoch, MaxReservedBytes: 4096}
	return e, store, binding, source, set, req
}

func TestUnsupportedIntegrationRetainsReviewableChangesWithoutReplay(t *testing.T) {
	e, store, binding, source, set, req := integrationFixture(t)
	ctx := context.Background()
	before, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.integrateChangeSet(ctx, e.Worker.Session, req, &integrationHooks{beforeReplacement: func() error { return ErrUnsupported }})
	if !errors.Is(err, ErrUnsupported) || page.State != "abandoned" || page.Integrated != 0 || page.Error != ErrUnsupported.Error() {
		t.Fatal("unsupported replacement was reported as an unknown mutation", page, err)
	}
	for _, receipt := range page.Files {
		if receipt.Dispatched {
			t.Fatal("unsupported backend dispatched a file", receipt)
		}
	}
	retained, err := store.InspectChangeSet(ctx, set.ID, binding, 0, 64)
	if err != nil {
		t.Fatal(err)
	}
	readable := 0
	for _, change := range retained.Operations {
		if change.Result == nil || change.Result.Kind != "file" {
			continue
		}
		blob, err := store.ReadChangeBlob(ctx, set.ID, binding, change.Result.SHA256, 0, MaxPageBytes)
		if err != nil || int64(len(blob.Data)) != change.Result.Bytes {
			t.Fatal("review download lost", err)
		}
		readable++
	}
	if readable == 0 {
		t.Fatal("no reviewable proposal retained")
	}
	// Repeating the original command only observes its saved refusal, even if
	// the next call runs on a supported filesystem.
	again, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
	if err != nil || again.ID != page.ID || again.State != "abandoned" {
		t.Fatal(again, err)
	}
	after, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(after) != string(before) {
		t.Fatal("retry mutated original", err)
	}
}

func TestIntegrationAppliesCombinedCandidateAndRetainsQuarantine(t *testing.T) {
	e, store, binding, source, set, req := integrationFixture(t)
	ctx := context.Background()
	page, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
	if err != nil || page.State != "integrated" || page.Integrated != 5 || page.Total != 5 {
		t.Fatal(page, err)
	}
	for name, want := range map[string]string{"input.txt": "candidate input", "independent.txt": "independent input", "new/dependency.txt": "new input"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil || string(data) != want {
			t.Fatal(name, string(data), err)
		}
	}
	if _, err = os.Stat(filepath.Join(source, "remove.txt")); !os.IsNotExist(err) {
		t.Fatal("deletion not applied", err)
	}
	if _, err = os.Stat(filepath.Join(source, "validation.log")); !os.IsNotExist(err) {
		t.Fatal("test log integrated", err)
	}
	for _, file := range page.Files {
		if file.State != "integrated" || !file.Dispatched {
			t.Fatal("missing native receipt", file)
		}
		if file.Displaced != "" {
			retained := filepath.Join(store.directory, set.CopyID, "artifacts", "integration-"+page.ID, fmt.Sprintf("%06d", file.Ordinal), file.Displaced)
			data, err := os.ReadFile(retained)
			if err != nil {
				t.Fatal(err)
			}
			want := "dirty original"
			if file.Path == "remove.txt" {
				want = "retain until integration"
			}
			if string(data) != want {
				t.Fatal("displaced original lost", file, string(data))
			}
		}
	}
	// A retry must not touch an edit made after completed integration.
	writeCandidateFile(t, source, "input.txt", "newer user edit")
	again, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
	if err != nil || again.State != "integrated" || again.ID != page.ID {
		t.Fatal(again, err)
	}
	data, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "newer user edit" {
		t.Fatal("completed integration replayed", string(data), err)
	}
	changed := req
	changed.MaxReservedBytes++
	if _, err = store.IntegrateChangeSet(ctx, e.Worker.Session, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed request accepted", err)
	}
	if err = store.AbandonChangeSet(ctx, set.ID, binding); !errors.Is(err, ErrConflict) {
		t.Fatal("integrated quarantine could be abandoned", err)
	}
	foreign := binding
	foreign.Actor = "foreign"
	if _, err = store.InspectIntegration(ctx, page.ID, foreign, 0, 1); !errors.Is(err, ErrAuthority) {
		t.Fatal("foreign receipts exposed", err)
	}
	first, err := store.InspectIntegration(ctx, page.ID, binding, 0, 1)
	if err != nil || len(first.Files) != 1 || first.NextOffset == nil || *first.NextOffset != 1 {
		t.Fatal("receipt paging", first, err)
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
	retained, err := reopened.InspectIntegration(ctx, page.ID, binding, 0, 64)
	if err != nil || retained.State != "integrated" || retained.Integrated != 5 {
		t.Fatal("restart lost receipts", retained, err)
	}
}

func TestIntegrationCrashAfterEachDispatchNeverReplays(t *testing.T) {
	for crashAt := 0; crashAt < 5; crashAt++ {
		t.Run(fmt.Sprint(crashAt), func(t *testing.T) {
			e, store, binding, source, _, req := integrationFixture(t)
			ctx := context.Background()
			page, err := store.integrateChangeSet(ctx, e.Worker.Session, req, &integrationHooks{afterDispatch: func(i int) error {
				if i == crashAt {
					return errors.New("injected crash before receipt")
				}
				return nil
			}})
			if err == nil || page.State != "uncertain" || page.Integrated != crashAt || page.Files[crashAt].State != "dispatching" || !page.Files[crashAt].Dispatched {
				t.Fatal("incorrect crash accounting", page, err)
			}
			writeCandidateFile(t, source, "input.txt", "user recovery edit")
			again, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
			if err != nil || again.ID != page.ID || again.State != "uncertain" {
				t.Fatal("retry changed uncertain work", again, err)
			}
			data, err := os.ReadFile(filepath.Join(source, "input.txt"))
			if err != nil || string(data) != "user recovery edit" {
				t.Fatal("uncertain operation replayed", string(data), err)
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
			retained, err := reopened.InspectIntegration(ctx, page.ID, binding, 0, 64)
			if err != nil || retained.State != "uncertain" || retained.Integrated != crashAt || retained.Files[crashAt].State != "uncertain" {
				t.Fatal("restart forgot partial effects", retained, err)
			}
			for _, file := range retained.Files[crashAt+1:] {
				if file.State != "pending" || file.Dispatched {
					t.Fatal("unstarted work dispatched", file)
				}
			}
		})
	}
}

func TestIntegrationConflictsRevocationAndPersistenceFaults(t *testing.T) {
	for _, mode := range []string{"preimage", "creation-race", "delete-race", "stop", "dispatch-write", "dispatch-ignored", "receipt-write", "completion-write", "completion-ignored", "capacity", "stale", "unvalidated"} {
		t.Run(mode, func(t *testing.T) {
			e, store, _, source, set, req := integrationFixture(t)
			ctx := context.Background()
			hooks := &integrationHooks{}
			switch mode {
			case "preimage":
				writeCandidateFile(t, source, "input.txt", "pre-existing user edit")
			case "creation-race":
				hooks.beforeStep = func(i int) {
					if i == 1 {
						writeCandidateFile(t, source, "independent.txt", "concurrent create")
					}
				}
			case "delete-race":
				hooks.beforeStep = func(i int) {
					if i == 0 {
						writeCandidateFile(t, source, "remove.txt", "concurrent edit")
					}
				}
			case "stop":
				hooks.beforeStep = func(i int) {
					if i == 1 {
						e.Worker.Session.Revoke()
					}
				}
			case "dispatch-write":
				if _, err := store.db.Exec(`CREATE TRIGGER integration_fault BEFORE UPDATE OF state ON worker_integration_files WHEN NEW.state='dispatching' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
					t.Fatal(err)
				}
			case "dispatch-ignored":
				if _, err := store.db.Exec(`CREATE TRIGGER integration_fault BEFORE UPDATE OF state ON worker_integration_files WHEN NEW.state='dispatching' BEGIN SELECT RAISE(IGNORE); END`); err != nil {
					t.Fatal(err)
				}
			case "receipt-write":
				if _, err := store.db.Exec(`CREATE TRIGGER integration_fault BEFORE UPDATE OF receipt ON worker_integration_files WHEN NEW.receipt IS NOT NULL BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
					t.Fatal(err)
				}
			case "completion-write":
				if _, err := store.db.Exec(`CREATE TRIGGER integration_fault BEFORE UPDATE OF state ON worker_integrations WHEN NEW.state='integrated' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
					t.Fatal(err)
				}
			case "completion-ignored":
				if _, err := store.db.Exec(`CREATE TRIGGER integration_fault BEFORE UPDATE OF state ON worker_integrations WHEN NEW.state='integrated' BEGIN SELECT RAISE(IGNORE); END`); err != nil {
					t.Fatal(err)
				}
			case "capacity":
				req.MaxReservedBytes = 1
			case "stale":
				req.InstructionVersion++
			case "unvalidated":
				if _, err := store.db.Exec(`UPDATE worker_change_sets SET state='validation_failed' WHERE id=?`, set.ID); err != nil {
					t.Fatal(err)
				}
			}
			page, err := store.integrateChangeSet(ctx, e.Worker.Session, req, hooks)
			if err == nil || page.State == "integrated" {
				t.Fatal("fault claimed integration", page, err)
			}
			if mode == "creation-race" || mode == "stop" {
				if page.State != "partial" || page.Integrated != 1 {
					t.Fatal("lost partial outcome", page)
				}
			}
			if mode == "preimage" || mode == "delete-race" {
				if page.State != "conflict" || page.Integrated != 0 {
					t.Fatal("missing explicit conflict", page)
				}
			}
			if mode == "preimage" && (page.ConflictPath != "input.txt" || page.Error == "") {
				t.Fatal("missing persisted conflict explanation", page)
			}
			if (mode == "dispatch-write" || mode == "dispatch-ignored") && (len(page.Files) == 0 || page.Files[0].Dispatched) {
				t.Fatal("failed intent dispatched", page)
			}
			if mode == "receipt-write" && (page.State != "uncertain" || !page.Files[0].Dispatched) {
				t.Fatal("lost dispatched effect", page)
			}
			if mode == "preimage" || mode == "delete-race" || mode == "creation-race" {
				name, want := "input.txt", "pre-existing user edit"
				if mode == "delete-race" {
					name, want = "remove.txt", "concurrent edit"
				}
				if mode == "creation-race" {
					name, want = "independent.txt", "concurrent create"
				}
				data, err := os.ReadFile(filepath.Join(source, name))
				if err != nil || string(data) != want {
					t.Fatal("user edit lost", string(data), err)
				}
			}
		})
	}
}
