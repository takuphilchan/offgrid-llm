package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workerFixture(t *testing.T) (*Manager, *Runner, ControlScope, *Task, *Task) {
	t.Helper()
	m, r, scope, c := controlFixture(t)
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID == "" {
			return runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"research","goal":"Research the question","depends_on":[],"tools":[]}]}`}}}}), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Worker result"}), nil
	}
	work := admitControlTestWork(t, r, scope, c)
	root, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
	if err != nil || len(root.ChildHistory) != 1 {
		t.Fatal(root, err)
	}
	child, _ := m.GetTask(root.ChildHistory[0].ID)
	if child.Worker == nil {
		t.Fatal("canonical child has no worker identity")
	}
	return m, r, scope, root, child
}

func TestWorkerMessageDeliveryApplicationAndRetry(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	worker, err := m.WorkerSession(scope, child.Worker.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	command := api.WorkerCommand{RequestID: "worker-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "message", Payload: api.WorkerCommandPayload{Content: "Include the unresolved limitations"}}
	accepted, err := r.CommandWorker(scope, command)
	if err != nil || accepted.Message.State != "accepted" {
		t.Fatal(accepted, err)
	}
	if _, err = r.CommandWorker(scope, command); err != nil {
		t.Fatal(err)
	}
	// Delivery can repeat independently of applying at the model checkpoint.
	if err = m.deliverWorkerMessages(child.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.deliverWorkerMessages(child.ID); err != nil {
		t.Fatal(err)
	}
	done, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if done.InstructionVersion != root.InstructionVersion || done.AssignmentVersion != 2 || len(done.Instructions) != 1 || len(done.PendingInstructions) != 0 {
		t.Fatal("mailbox changed work version or applied twice", done)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	message, err := readWorkerMessage(db, scope, worker.ID, command.RequestID)
	if err != nil || message.State != "applied" || message.DeliveredAt == nil || message.AppliedAt == nil {
		t.Fatal(message, err)
	}
	current, err := m.WorkerSession(scope, worker.ID)
	if err != nil || current.Status != TaskCompleted || current.ResultReference == "" || current.CurrentRunID != child.ID {
		t.Fatal(current, err)
	}
	if _, err = m.WorkerSession(ControlScope{scope.Workspace, "bob"}, worker.ID); !errors.Is(err, ErrControlNotFound) {
		t.Fatal("foreign worker exposed", err)
	}
}

func TestWorkerStopDoesNotStopRootOrReviveByMessage(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	stopped, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "stop-worker", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "stop"})
	if err != nil || !stopped.Worker.Stopped || stopped.Worker.Status != TaskCancelled {
		t.Fatal(stopped, err)
	}
	parent, _ := m.GetTask(root.ID)
	if parent.Status != TaskChildren {
		t.Fatal("worker stop stopped parent", parent.Status)
	}
	_, err = r.CommandWorker(scope, api.WorkerCommand{RequestID: "revive", TargetID: worker.ID, ExpectedRevision: stopped.Revision, Kind: "message", Payload: api.WorkerCommandPayload{Content: "Continue anyway"}})
	if !errors.Is(err, ErrRunConflict) {
		t.Fatal("message revived stopped worker", err)
	}
}

func TestWorkerDeliverySurvivesRestartWithoutApplyingTwice(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	command := api.WorkerCommand{RequestID: "restart-message", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "message", Payload: api.WorkerCommandPayload{Content: "Keep this correction"}}
	if _, err := r.CommandWorker(scope, command); err != nil {
		t.Fatal(err)
	}
	if err := m.deliverWorkerMessages(child.ID); err != nil {
		t.Fatal(err)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restarted := NewRunner(loaded, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Result"}), nil
	})
	if _, err := restarted.CommandWorker(scope, command); err != nil {
		t.Fatal(err)
	}
	work, _ := loaded.WorkItem(scope, root.WorkID)
	if _, err := restarted.ControlWork(scope, api.WorkCommand{RequestID: "resume-work", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "continue"}); err != nil {
		t.Fatal(err)
	}
	done, err := restarted.Continue(context.Background(), child.ID, scope.Actor, "start", "", false)
	if err != nil || len(done.Instructions) != 1 || done.AssignmentVersion != 2 {
		t.Fatal(done, err)
	}
}
