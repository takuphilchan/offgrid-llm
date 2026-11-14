package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkingCopyFollowupAfterIntegrationPreservesCurrentSource(t *testing.T) {
	e, store, binding, source, set, integrate := integrationFixture(t)
	store.policy.MaxTotalBytes = 24 * 1024
	ctx := context.Background()
	input, err := store.Manifest(ctx, set.CopyID, binding)
	if err != nil || len(input.Prerequisites) < 1 {
		t.Fatal(input, err)
	}
	page, err := store.IntegrateChangeSet(ctx, e.Worker.Session, integrate)
	if err != nil || page.State != "integrated" {
		t.Fatal(page, err)
	}
	// A later user edit and recreation of an earlier deletion must survive.
	writeCandidateFile(t, source, "input.txt", "user changed after integration")
	writeCandidateFile(t, source, "remove.txt", "user restored file")
	nextBinding := binding
	nextBinding.Job = "follow-up-root"
	if err = e.Admit(ctx, nextBinding); err != nil {
		t.Fatal(err)
	}
	nextReq := workingCopyRequest(nextBinding, "follow-up-editor")
	nextReq.ControlEpoch = integrate.ControlEpoch + 1
	prior := input.Prerequisites[0]
	prior.SourceJob = binding.Job
	nextReq.Prerequisites = []ChangeSetReference{prior}
	nextReq.Integrated = []IntegrationReference{{ID: page.ID, SourceJob: binding.Job}}
	next, err := e.PrepareWorker(ctx, nextReq)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"input.txt": "user changed after integration", "remove.txt": "user restored file"} {
		data, readErr := os.ReadFile(filepath.Join(next.Context.Root, name))
		if readErr != nil || string(data) != want {
			t.Fatal("old effects replayed", name, string(data), readErr)
		}
	}
	writeCandidateFile(t, next.Context.Root, "input.txt", "follow-up based on current source")
	nextSet := sealCandidate(t, next, nextReq)
	testReq := workingCopyRequest(nextBinding, "follow-up-test")
	testReq.Purpose, testReq.ControlEpoch = "test", nextReq.ControlEpoch
	testReq.Prerequisites = []ChangeSetReference{candidateReference(nextSet)}
	testReq.Integrated = nextReq.Integrated
	tester, err := e.PrepareWorker(ctx, testReq)
	if err != nil {
		t.Fatal(err)
	}
	intent := request(tester.Endpoint.Worker, tester.Context.Binding, "process_start", "validate-follow-up", ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestValidationCommandFixture$", "--", "offgrid-validation-fixture", "follow-up"}, TimeoutSeconds: 10})
	intent.Approved = true
	response := tester.Endpoint.Worker.Handle(ctx, intent)
	if response.Error != "" || response.Receipt == nil || response.Receipt.State != "dispatched" {
		t.Fatal(response)
	}
	tester.Endpoint.Worker.finishing.Wait()
	seal := SealChangeSetRequest{RequestID: "seal", CopyID: tester.Record.ID, Source: nextBinding, InstructionVersion: testReq.InstructionVersion, ControlEpoch: testReq.ControlEpoch, MaxReservedBytes: 4096}
	validated, err := tester.SealChangeSet(ctx, seal)
	if err != nil || validated.Validation.State != "passed" {
		t.Fatal(validated, err)
	}
	candidate, err := store.PrepareIntegrationCandidate(ctx, seal)
	if err != nil || candidate.State != "ready" || candidate.OperationCount != 1 {
		t.Fatal("old edits entered new integration", candidate, err)
	}
	changes, err := store.InspectChangeSet(ctx, candidate.ID, nextBinding, 0, 64)
	if err != nil || changes.Operations[0].Path != "input.txt" || changes.Operations[0].Base.SHA256 != digest([]byte("user changed after integration")) {
		t.Fatal("wrong new baseline", changes, err)
	}
	result, err := store.IntegrateChangeSet(ctx, e.Worker.Session, IntegrateChangeSetRequest{RequestID: "second-integration", Source: nextBinding, ChangeSetID: candidate.ID, OperationsDigest: candidate.OperationsDigest, InstructionVersion: testReq.InstructionVersion, ControlEpoch: testReq.ControlEpoch, MaxReservedBytes: 4096})
	if err != nil || result.State != "integrated" || result.Total != 1 {
		t.Fatal(result, err)
	}
	for name, want := range map[string]string{"input.txt": "follow-up based on current source", "remove.txt": "user restored file"} {
		data, readErr := os.ReadFile(filepath.Join(source, name))
		if readErr != nil || string(data) != want {
			t.Fatal(name, string(data), readErr)
		}
	}
}

func TestWorkingCopyRejectsUnprovenIntegratedInputs(t *testing.T) {
	for _, mode := range []string{"missing", "foreign", "partial", "unknown", "digest"} {
		t.Run(mode, func(t *testing.T) {
			e, store, binding, source, set, req := integrationFixture(t)
			ctx := context.Background()
			input, err := store.Manifest(ctx, set.CopyID, binding)
			if err != nil {
				t.Fatal(err)
			}
			page, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
			if err != nil {
				t.Fatal(err)
			}
			next := workingCopyRequest(binding, "rejected-follow-up")
			next.Prerequisites = []ChangeSetReference{input.Prerequisites[0]}
			next.Integrated = []IntegrationReference{{ID: page.ID, SourceJob: binding.Job}}
			switch mode {
			case "missing":
				next.Integrated = nil
			case "foreign":
				next.Integrated[0].SourceJob = "foreign-job"
			case "partial":
				_, err = store.db.Exec(`UPDATE worker_integrations SET state='partial' WHERE id=?`, page.ID)
			case "unknown":
				next.Integrated[0].ID = strings.Repeat("f", 64)
			case "digest":
				next.Prerequisites[0].OperationsDigest = strings.Repeat("f", 64)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = e.PrepareWorker(ctx, next); err == nil {
				t.Fatal("unproven applied input accepted")
			}
			data, err := os.ReadFile(filepath.Join(source, "input.txt"))
			if err != nil || string(data) != "candidate input" {
				t.Fatal("rejected preparation touched source", err)
			}
		})
	}
}
