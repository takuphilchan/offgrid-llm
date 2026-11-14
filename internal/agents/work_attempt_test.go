package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func completedWorkerWork(t *testing.T) (*Manager, *Runner, ControlScope, *Task, *Task) {
	t.Helper()
	m, r, scope, root, child := workerFixture(t)
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Retained completed outcome"}), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	root = awaitTask(t, m, root.ID, TaskCompleted)
	settleTask(t, r, root.ID)
	return m, r, scope, root, child
}

func TestWorkerFollowupAfterCompletionPreservesAttemptsAndBudget(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	var err error
	root, err = m.updateTask(root.ID, func(task *Task) error {
		task.Config.ComputerSession, task.Config.ComputerDriver = "previous-session", "native"
		task.Config.ComputerScope = "desktop"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	before, _ := json.Marshal(root)
	command := api.WorkerCommand{RequestID: "after-completion", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Expand the remaining questions"}}
	if _, err := r.continueWorker(scope, command, false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("model reopened completed work", err)
	}
	accepted, err := r.CommandWorker(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Worker.ID != worker.ID || accepted.Worker.RootRunID == root.ID {
		t.Fatal("no distinct retained attempt", accepted.Worker)
	}
	next, _ := m.GetTask(accepted.Worker.RootRunID)
	assignment, _ := m.GetTask(accepted.Worker.CurrentRunID)
	if next.Config.ComputerSession != "" || next.Config.ComputerDriver != "" || next.Config.ComputerScope != "" || next.Config.Execution != nil {
		t.Fatal("new attempt inherited a run-bound grant")
	}
	if next.WorkID != root.WorkID || assignment.ParentID != next.ID || next.Config.MaxIterations != root.Config.MaxIterations || next.Checkpoint.Iteration != root.Checkpoint.Iteration || next.Checkpoint.BudgetedModelCalls != root.Checkpoint.BudgetedModelCalls || next.Checkpoint.ChargedCompletionTokens != root.Checkpoint.ChargedCompletionTokens || next.Checkpoint.ChargedInferenceNS != root.Checkpoint.ChargedInferenceNS || next.Checkpoint.ReservedChildIterations != root.Checkpoint.ReservedChildIterations+assignment.Config.MaxIterations {
		t.Fatal("work identity or accumulated allowance changed", next)
	}
	if workerSessionCount(next) != 1 {
		t.Fatal("same worker counted twice")
	}
	finals := 0
	for _, message := range next.Checkpoint.Messages {
		if message.Role == "assistant" && message.StringContent() == root.Result {
			finals++
		}
	}
	if finals != 1 {
		t.Fatal("completed outcome duplicated in retained context", finals)
	}
	previous, _ := m.GetTask(root.ID)
	after, _ := json.Marshal(previous)
	if !bytes.Equal(before, after) {
		t.Fatal("completed run rewritten")
	}
	if _, err = r.Archive(root.ID, scope.Actor, true); err != nil {
		t.Fatal("historical root could not be archived", err)
	}
	if _, err = r.Archive(child.ID, scope.Actor, true); !errors.Is(err, ErrRunConflict) {
		t.Fatal("child archive bypassed root ownership", err)
	}
	if err = m.StorageError(); err != nil {
		t.Fatal("historical inspection poisoned storage", err)
	}
	work, _ := m.WorkItem(scope, root.WorkID)
	if len(work.RunIDs) != 2 || work.RunIDs[0] != root.ID || work.RunID != next.ID {
		t.Fatal("attempt history missing", work)
	}
	runs, err := m.ConversationRuns(scope, root.Origin.Conversation)
	if err != nil || len(runs) != 2 {
		t.Fatal("historical outcome missing from conversation", runs, err)
	}
	retry, err := r.CommandWorker(scope, command)
	if err != nil || retry.Worker.CurrentRunID != assignment.ID {
		t.Fatal("retry created another attempt", retry, err)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	recovered, _ := loaded.WorkItem(scope, root.WorkID)
	if recovered.RunID != next.ID || len(recovered.RunIDs) != 2 {
		t.Fatal("restart lost attempt history", recovered)
	}
	pending, _ := loaded.GetTask(next.ID)
	if pending.Status != TaskInterrupted || pending.Checkpoint.ReservedChildIterations != next.Checkpoint.ReservedChildIterations {
		t.Fatal("restart dispatched or refunded work", pending)
	}
}

func TestWorkerWorkAttemptRollsBackWithReceiptAndRejectsExhaustion(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		m, r, scope, root, child := completedWorkerWork(t)
		worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
		db, err := m.controlDatabase()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if exhausted {
			_, err = m.updateTask(root.ID, func(task *Task) error { task.Checkpoint.BudgetedModelCalls = task.Config.MaxIterations; return nil })
		} else {
			_, err = db.Exec(`CREATE TRIGGER reject_attempt_receipt BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='followup' BEGIN SELECT RAISE(ABORT,'injected'); END`)
		}
		if err != nil {
			t.Fatal(err)
		}
		command := api.WorkerCommand{RequestID: "followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Continue"}}
		if _, err = r.CommandWorker(scope, command); err == nil {
			t.Fatal("invalid attempt accepted")
		}
		work, _ := m.WorkItem(scope, root.WorkID)
		retained, _ := m.WorkerSession(scope, worker.ID)
		if work.RunID != root.ID || len(work.RunIDs) != 1 || retained.CurrentRunID != child.ID {
			t.Fatal("partial identity switch", work, retained)
		}
		if !exhausted {
			if _, err = db.Exec(`DROP TRIGGER reject_attempt_receipt`); err != nil {
				t.Fatal(err)
			}
			if _, err = r.CommandWorker(scope, command); err != nil {
				t.Fatal("retry failed after rollback", err)
			}
		}
	}
}

func TestWorkerFollowupAttemptsExecuteWithoutReopeningAllowance(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	initial, _ := m.GetTask(root.ID)
	completed := 1
	for attempt := 0; attempt < initial.Config.MaxIterations; attempt++ {
		worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
		command := api.WorkerCommand{RequestID: "attempt-" + runID(), TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Continue the same bounded review"}}
		receipt, err := r.CommandWorker(scope, command)
		if err != nil {
			if completed < 2 {
				t.Fatal("no follow-up completed", err)
			}
			work, _ := m.WorkItem(scope, root.WorkID)
			if len(work.RunIDs) != completed {
				t.Fatal("failed admission changed attempt history")
			}
			return
		}
		if _, err = r.Continue(context.Background(), receipt.Worker.CurrentRunID, scope.Actor, "start", "", false); err != nil {
			t.Fatal(err)
		}
		r.advanceDelegation(context.Background(), receipt.Worker.RootRunID)
		next := awaitTask(t, m, receipt.Worker.RootRunID, TaskCompleted)
		settleTask(t, r, next.ID)
		completed++
		if next.Checkpoint.BudgetedModelCalls <= initial.Checkpoint.BudgetedModelCalls || next.Config.MaxIterations != initial.Config.MaxIterations {
			t.Fatal("allowance reset", next)
		}
	}
	t.Fatal("unbounded follow-up attempts")
}

func TestWorkAttemptUpgradePreservesHistoryAndFencesOldRunners(t *testing.T) {
	m, _, scope, root, _ := completedWorkerWork(t)
	legacy, err := m.WorkItem(scope, root.WorkID)
	if err != nil {
		t.Fatal(err)
	}
	legacy.RunIDs = nil // Actual v8 records have only the current run identity.
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_control_records SET snapshot=? WHERE workspace=? AND actor=? AND id=?`, data, scope.Workspace, scope.Actor, root.WorkID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=8`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	work, err := loaded.WorkItem(scope, root.WorkID)
	if err != nil || work.RunID != root.ID {
		t.Fatal(work, err)
	}
	runs, err := loaded.ConversationRuns(scope, root.Origin.Conversation)
	if err != nil || len(runs) != 1 || runs[0] != root.ID {
		t.Fatal("v8 history fallback lost the original run", runs, err)
	}
	db, err = loaded.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err = db.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil || version != taskSchemaVersion {
		t.Fatal("old-runner fence absent", version, err)
	}
	var manifest string
	if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&manifest); err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(manifest)) {
		t.Fatal("migration manifest missing")
	}
}

func TestWorkAttemptReusesOtherRetainedWorkersWithinCurrentAttempt(t *testing.T) {
	m, r, scope, conversation := controlFixture(t)
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("delegate_tasks", `{"tasks":[{"key":"research","goal":"Research","depends_on":[],"tools":[]},{"key":"review","goal":"Review","depends_on":[],"tools":[]}]}`), nil
	}
	work := admitControlTestWork(t, r, scope, conversation)
	root, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
	if err != nil || len(root.ChildHistory) != 2 {
		t.Fatal(root, err)
	}
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Retained evidence"}), nil
	}
	for _, child := range root.ChildHistory {
		if _, err = r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
			t.Fatal(err)
		}
	}
	r.advanceDelegation(context.Background(), root.ID)
	awaitTask(t, m, root.ID, TaskCompleted)
	settleTask(t, r, root.ID)
	newRoot := ""
	for i, child := range root.ChildHistory {
		worker, err := m.WorkerSession(scope, child.WorkerID)
		if err != nil {
			t.Fatal(err)
		}
		command := api.WorkerCommand{RequestID: "reuse-" + runID(), TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Follow up on this evidence"}}
		// Only the first user command opens a work attempt. The assistant can
		// then reuse another retained worker without recreating its context.
		accepted, err := r.continueWorker(scope, command, i == 0)
		if err != nil {
			t.Fatal(err)
		}
		if newRoot != "" && accepted.Worker.RootRunID != newRoot {
			t.Fatal("second worker created another root")
		}
		newRoot = accepted.Worker.RootRunID
	}
	current, _ := m.GetTask(newRoot)
	if len(current.Delegation.Children) != 2 || workerSessionCount(current) != 2 {
		t.Fatal("retained workers lost or duplicated", current)
	}
	for _, child := range current.Delegation.Children {
		if _, err = r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
			t.Fatal(err)
		}
	}
	r.advanceDelegation(context.Background(), newRoot)
	awaitTask(t, m, newRoot, TaskCompleted)
	settleTask(t, r, newRoot)
	saved, _ := m.WorkItem(scope, work.ID)
	if len(saved.RunIDs) != 2 {
		t.Fatal("unexpected attempt history", saved)
	}
}
