package agents

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func admitControlTestWork(t *testing.T, r *Runner, scope ControlScope, c *Conversation) *WorkItem {
	t.Helper()
	receipt, err := r.AdmitConversationWork(scope, ConversationCommand{RequestID: "work", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "create_work", Payload: ConversationCommandPayload{Input: &ConversationInput{Content: "Do the work"}}}, "model", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	return receipt.Work
}

func TestWorkControlCorrectionAtomicDedupeAndRestart(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	command := api.WorkCommand{RequestID: "correction", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "correct", Payload: api.WorkCommandPayload{Instruction: "Keep user edits"}}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := r.ControlWork(scope, command); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	task, _ := m.GetTask(work.RunID)
	if task.InstructionVersion != 2 || len(task.Instructions) != 1 || task.Checkpoint.Iteration != 0 {
		t.Fatal("correction or budget applied twice", task)
	}
	other := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	restarted := NewRunner(other, NewToolRegistry(), nil)
	if _, err := restarted.ControlWork(scope, command); err != nil {
		t.Fatal(err)
	}
	loaded, _ := other.GetTask(task.ID)
	if loaded.Status != TaskInterrupted || len(loaded.Instructions) != 1 {
		t.Fatal("retry resumed or reapplied", loaded)
	}
	command.Payload.Instruction = "different"
	if _, err := restarted.ControlWork(scope, command); !errors.Is(err, ErrRunConflict) {
		t.Fatal(err)
	}
}

func TestWorkControlFailureDoesNotPublishOrCancel(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	cancelled := false
	r.active[work.RunID] = func() { cancelled = true }
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TRIGGER reject_work_control BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='stop' BEGIN SELECT RAISE(ABORT,'injected'); END`)
	if err != nil {
		t.Fatal(err)
	}
	command := api.WorkCommand{RequestID: "stop", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"}
	if _, err = r.ControlWork(scope, command); err == nil {
		t.Fatal("commit fault ignored")
	}
	task, _ := m.GetTask(work.RunID)
	current, err := m.WorkItem(scope, work.ID)
	if err != nil || cancelled || task.ControlEpoch != 0 || current.Revision != work.Revision || task.Status != TaskPending {
		t.Fatal("uncommitted stop published", task, current, cancelled, err)
	}
	delete(r.active, work.RunID)
}

func TestWorkStopAcceptsWithoutModelAndLateAnswerCannotWin(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		close(entered)
		<-release
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "late success"}), nil
	}
	work := admitControlTestWork(t, r, scope, c)
	if _, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("model never entered")
	}
	work, _ = m.WorkItem(scope, work.ID)
	command := api.WorkCommand{RequestID: "stop", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"}
	accepted, err := r.ControlWork(scope, command)
	if err != nil || accepted.Work.Status != "stopping" {
		close(release)
		t.Fatal(accepted, err)
	}
	// A second deliberate Stop cannot pretend the first cancellation settled.
	command.RequestID, command.ExpectedRevision = "stop-again", accepted.Revision
	second, err := r.ControlWork(scope, command)
	if err != nil || second.Work.Status != "stopping" {
		close(release)
		t.Fatal(second, err)
	}
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	final, err := m.WorkItem(scope, work.ID)
	task, _ := m.GetTask(work.RunID)
	if err != nil || final.Status != TaskCancelled || task.Result == "late success" || task.ControlPending {
		t.Fatal(final, task, err)
	}
}

func TestWorkStopPreservesAlreadyCompletedOutcome(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	if _, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	work, _ = m.WorkItem(scope, work.ID)
	accepted, err := r.ControlWork(scope, api.WorkCommand{RequestID: "late-stop", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"})
	if err != nil || accepted.Work.Status != TaskCompleted || accepted.Work.ResultReference != work.ResultReference {
		t.Fatal(accepted, err)
	}
}

func TestWorkPauseContinuePreservesAllowanceAndDoesNotDispatchOnRetry(t *testing.T) {
	m, r, scope, c := controlFixture(t)
	work := admitControlTestWork(t, r, scope, c)
	_, err := m.updateTask(work.RunID, func(task *Task) error { task.Checkpoint.Iteration = 3; return nil })
	if err != nil {
		t.Fatal(err)
	}
	paused, err := r.ControlWork(scope, api.WorkCommand{RequestID: "pause", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "pause"})
	if err != nil || paused.Work.Status != TaskInterrupted {
		t.Fatal(paused, err)
	}
	command := api.WorkCommand{RequestID: "continue", TargetID: work.ID, ExpectedRevision: paused.Revision, Kind: "continue"}
	continued, err := r.ControlWork(scope, command)
	if err != nil || continued.Work.Status != TaskPending {
		t.Fatal(continued, err)
	}
	if _, err = r.ControlWork(scope, command); err != nil {
		t.Fatal(err)
	}
	task, _ := m.GetTask(work.RunID)
	if task.Checkpoint.Iteration != 3 || !task.SchedulerReady || len(r.active) != 0 {
		t.Fatal("continuation reopened allowance or retry dispatched", task)
	}
}
