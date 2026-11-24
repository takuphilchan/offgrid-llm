package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func stoppedModelWorker(t *testing.T) (*Manager, *Runner, *Task, *Task) {
	t.Helper()
	m, r, root, children := modelGraphFixture(t)
	calls := 0
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		return nil, errors.New("fixture runtime unavailable")
	}
	child, err := r.Continue(context.Background(), children[0].ID, root.Actor, "start", "", false)
	if err == nil || child.Status != TaskInterrupted || child.Checkpoint.DecisionRepairs != 1 {
		t.Fatalf("interrupted worker: %+v %v", child, err)
	}
	scope := ControlScope{root.Origin.Workspace, root.Actor}
	worker, err := m.WorkerSession(scope, child.Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommandWorker(scope, api.WorkerCommand{RequestID: "stop-interrupted-worker", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "stop"}); err != nil {
		t.Fatal(err)
	}
	child, _ = m.GetTask(child.ID)
	if child.Status != TaskCancelled || child.Checkpoint.DecisionRepairs != 1 {
		t.Fatal("stop lost unresolved sequence")
	}
	return m, r, root, child
}

func TestModelContinuationResolvesOnlyItsPriorSequence(t *testing.T) {
	m, r, root, old := stoppedModelWorker(t)
	other := root.ChildHistory[1].ID
	if _, err := m.updateTask(root.ID, func(task *Task) error { task.Checkpoint.ModelBudget.Repairs[other] = 1; return nil }); err != nil {
		t.Fatal(err)
	}
	scope := ControlScope{root.Origin.Workspace, root.Actor}
	worker, err := m.WorkerSession(scope, old.Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	command := api.WorkerCommand{RequestID: "repair-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Complete the comparison"}}
	receipt, err := r.CommandWorker(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(receipt.Worker.CurrentRunID)
	parent, _ := m.GetTask(root.ID)
	if next.Checkpoint.DecisionRepairs != 1 || parent.Checkpoint.ModelBudget.Repairs[next.ID] != 1 || parent.Checkpoint.ModelBudget.Repairs[old.ID] != 0 || parent.Checkpoint.ModelBudget.Switches != 1 {
		t.Fatal("admission reset correction charge")
	}
	// Restart observes the same transfer, not both charges or neither charge.
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, _ := reloaded.GetTask(root.ID)
	if saved.Checkpoint.ModelBudget.Repairs[next.ID] != 1 || saved.Checkpoint.ModelBudget.Repairs[old.ID] != 0 {
		t.Fatal("restart changed transferred debt")
	}
	calls := 0
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if task.Checkpoint.DecisionRepairs != 1 {
			t.Fatal("inference began with refunded correction sequence")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Compared the supplied findings"}), nil
	}
	finished, err := r.Continue(context.Background(), next.ID, root.Actor, "start", "", false)
	if err != nil || finished.Status != TaskCompleted || calls != 1 {
		t.Fatalf("followup: %+v %v", finished, err)
	}
	parent, _ = m.GetTask(root.ID)
	if len(parent.Checkpoint.ModelBudget.Repairs) != 1 || parent.Checkpoint.ModelBudget.Repairs[other] != 1 || parent.Checkpoint.ModelBudget.Switches != 1 {
		t.Fatal("valid decision did not resolve only corrective debt")
	}
	prior, _ := m.GetTask(old.ID)
	if prior.Status != TaskCancelled || prior.Checkpoint.DecisionRepairs != 1 {
		t.Fatal("historical attempt changed")
	}
}

func TestModelContinuationCannotResetExhaustedGraph(t *testing.T) {
	m, r, root, old := stoppedModelWorker(t)
	if _, err := m.updateTask(root.ID, func(task *Task) error { task.Checkpoint.ModelBudget.Repairs[root.ChildHistory[1].ID] = 2; return nil }); err != nil {
		t.Fatal(err)
	}
	scope := ControlScope{root.Origin.Workspace, root.Actor}
	worker, _ := m.WorkerSession(scope, old.Worker.SessionID)
	receipt, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "exhausted-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Finish comparison"}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return nil, errors.New("must not execute")
	}
	stopped, err := r.Continue(context.Background(), receipt.Worker.CurrentRunID, root.Actor, "start", "", false)
	if err != nil || calls != 0 || stopped.Status != TaskInterrupted || stopped.ErrorCode != "model_decision_repair_limit" {
		t.Fatalf("continuation reset exhaustion: %+v %v calls=%d", stopped, err, calls)
	}
}

func TestModelContinuationRollbackKeepsOldCharge(t *testing.T) {
	m, r, root, old := stoppedModelWorker(t)
	scope := ControlScope{root.Origin.Workspace, root.Actor}
	worker, _ := m.WorkerSession(scope, old.Worker.SessionID)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_model_followup BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='reject-followup' BEGIN SELECT RAISE(ABORT,'fixture receipt failure'); END`); err != nil {
		t.Fatal(err)
	}
	command := api.WorkerCommand{RequestID: "reject-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Complete comparison"}}
	if _, err = r.CommandWorker(scope, command); err == nil {
		t.Fatal("failed transaction accepted")
	}
	parent, _ := m.GetTask(root.ID)
	current, _ := m.WorkerSession(scope, worker.ID)
	if parent.Checkpoint.ModelBudget.Repairs[old.ID] != 1 || len(parent.Checkpoint.ModelBudget.Repairs) != 1 || current.CurrentRunID != old.ID {
		t.Fatal("failed command partially moved correction charge")
	}
}
