package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerContinuationRetainsContextIdentityAndRootAllowance(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	priorRoot, _ := m.GetTask(root.ID)
	command := api.WorkerCommand{RequestID: "follow-up-run", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Expand the unresolved questions"}}
	accepted, err := r.CommandWorker(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Worker.ID != worker.ID || accepted.Worker.CurrentRunID == child.ID || len(accepted.Worker.RunIDs) != 2 {
		t.Fatal("session was recreated", accepted)
	}
	if _, err = r.CommandWorker(scope, command); err != nil {
		t.Fatal(err)
	}
	parent, _ := m.GetTask(root.ID)
	next, _ := m.GetTask(accepted.Worker.CurrentRunID)
	settlement := parent.Checkpoint.ChildBudgetSettlements[child.ID]
	if settlement.Allocated != child.Config.MaxIterations || settlement.Retained != 1 || parent.Checkpoint.ReservedChildIterations != priorRoot.Checkpoint.ReservedChildIterations-(settlement.Allocated-settlement.Retained)+next.Config.MaxIterations || parent.Checkpoint.BudgetedModelCalls != priorRoot.Checkpoint.BudgetedModelCalls || workerSessionCount(parent) != 1 {
		t.Fatal("continuation reset allowance or created a session")
	}
	retained := false
	for _, message := range next.Checkpoint.Messages {
		retained = retained || message.StringContent() == "Worker result"
	}
	if !retained || next.InstructionVersion != child.InstructionVersion || next.AssignmentVersion != child.AssignmentVersion+1 {
		t.Fatal("continuation lost context or provenance", next)
	}
	if _, err = r.Continue(context.Background(), next.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	stored, err := reloaded.WorkerSession(scope, worker.ID)
	if err != nil || stored.CurrentRunID != next.ID || len(stored.RunIDs) != 2 {
		t.Fatal(stored, err)
	}
}

func TestStoppedWorkerRequiresExplicitUserContinuation(t *testing.T) {
	m, r, scope, _, child := workerFixture(t)
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	stopped, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "stop", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "stop"})
	if err != nil {
		t.Fatal(err)
	}
	command := api.WorkerCommand{RequestID: "continue", TargetID: worker.ID, ExpectedRevision: stopped.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Continue the review"}}
	if _, err = r.continueWorker(scope, command, false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("automatic revival", err)
	}
	accepted, err := r.CommandWorker(scope, command)
	if err != nil || accepted.Worker.Stopped || accepted.Worker.CurrentRunID == child.ID {
		t.Fatal(accepted, err)
	}
}
