package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func readyCandidateReply(req execution.SealChangeSetRequest, child string) execution.WorkerArtifactReply {
	reply := sealedBudgetReply(req, child)
	reply.Copy.Purpose = "test"
	reply.Copy.ManifestDigest = strings.Repeat("b", 64)
	reply.ChangeSet.Validation = execution.ValidationSummary{Version: 1, ID: strings.Repeat("c", 64), State: "passed", InputManifestDigest: reply.Copy.ManifestDigest, CheckCount: 1}
	candidate := *reply.ChangeSet
	candidate.State = "ready"
	candidate.BaseManifestDigest = strings.Repeat("d", 64)
	candidate.ResultManifestDigest = reply.Copy.ManifestDigest
	request := execution.IntegrationCandidateRequest(req)
	identity, _ := json.Marshal(struct {
		Source            execution.Binding
		CopyID, RequestID string
	}{request.Source, request.CopyID, request.RequestID})
	hash := sha256.Sum256(identity)
	candidate.ID = hex.EncodeToString(hash[:])
	reply.Candidate, reply.CandidateState = &candidate, "ready"
	return reply
}

func TestWorkerCaptureRetainsCandidateWithoutRevivingStoppedWork(t *testing.T) {
	m, root, child, req := captureBudgetFixture(t)
	reply := readyCandidateReply(req, child)
	if _, err := m.updateTask(root.ID, func(task *Task) error { finishTask(task, TaskCancelled, "Stopped"); return nil }); err != nil {
		t.Fatal(err)
	}
	if err := m.RecordWorkerCapture(root.ID, root.Actor, child, req, reply); err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	current, _ := restored.GetTask(root.ID)
	output := workerCapturedOutput(current, child)
	if current.Status != TaskCancelled || output == nil || output.Candidate == nil || output.Candidate.State != "ready" || output.Candidate.ChangeSetID != reply.Candidate.ID || current.Checkpoint.StorageBudget.ChargedBytes != 120 {
		t.Fatal("lost evidence or revived stopped work", output)
	}
	if err := restored.RecordWorkerCapture(root.ID, root.Actor, child, req, reply); err != nil {
		t.Fatal("identical retry rejected", err)
	}
	reply.Candidate.OperationsDigest = strings.Repeat("e", 64)
	if err := restored.RecordWorkerCapture(root.ID, root.Actor, child, req, reply); err == nil {
		t.Fatal("conflicting retry changed candidate")
	}
}

func TestWorkerCaptureRejectsForgedIntegrationCandidate(t *testing.T) {
	for _, mode := range []string{"identity", "source", "worker", "instruction", "epoch", "unvalidated", "input", "digest", "state", "integrated", "missing", "unmarked"} {
		t.Run(mode, func(t *testing.T) {
			m, root, child, req := captureBudgetFixture(t)
			reply := readyCandidateReply(req, child)
			switch mode {
			case "identity":
				reply.Candidate.ID = strings.Repeat("f", 64)
			case "source":
				reply.Candidate.Source.Grant = "different"
			case "worker":
				reply.Candidate.WorkerRunID = "different"
			case "instruction":
				reply.Candidate.InstructionVersion++
			case "epoch":
				reply.Candidate.ControlEpoch++
			case "unvalidated":
				reply.Candidate.Validation.State = "failed"
			case "input":
				reply.Candidate.ResultManifestDigest = strings.Repeat("f", 64)
			case "digest":
				reply.Candidate.OperationsDigest = strings.Repeat("z", 64)
			case "state":
				reply.Candidate.State = "sealed"
			case "integrated":
				reply.Candidate.IntegrationStarted = true
			case "missing":
				reply.Candidate = nil
			case "unmarked":
				reply.CandidateState = ""
			}
			if err := m.RecordWorkerCapture(root.ID, root.Actor, child, req, reply); err == nil {
				t.Fatal("forged evidence accepted")
			}
			current, _ := m.GetTask(root.ID)
			if workerCapturedOutput(current, child) != nil || current.Checkpoint.StorageBudget.ChargedBytes != 200 {
				t.Fatal("invalid evidence changed state")
			}
		})
	}
}

func TestWorkerCaptureKeepsSealedOutputWhenCandidateIsUncertain(t *testing.T) {
	m, root, child, req := captureBudgetFixture(t)
	reply := sealedBudgetReply(req, child)
	reply.CandidateState, reply.CandidateError = "uncertain", "execution_output_limit"
	if err := m.RecordWorkerCapture(root.ID, root.Actor, child, req, reply); err != nil {
		t.Fatal(err)
	}
	current, _ := m.GetTask(root.ID)
	output := workerCapturedOutput(current, child)
	if output == nil || output.State != "sealed" || output.Candidate == nil || output.Candidate.State != "uncertain" {
		t.Fatal("candidate failure discarded sealed evidence", output)
	}
}
