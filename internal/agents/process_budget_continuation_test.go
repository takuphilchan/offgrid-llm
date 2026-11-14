package agents

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestProcessBudgetContinuationKeepsCharges(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	root, err := m.updateTask(root.ID, func(saved *Task) error {
		if err := ensureProcessBudget(saved); err != nil {
			return err
		}
		b := saved.Checkpoint.ProcessBudget
		b.ReservedStarts, b.ChargedNS, b.MeasuredNS = 1, int64(time.Second), int64(time.Second)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	accepted, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "process-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Follow up"}})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(accepted.Worker.RootRunID)
	if !reflect.DeepEqual(root.Checkpoint.ProcessBudget, next.Checkpoint.ProcessBudget) {
		t.Fatal("continuation reopened process capacity")
	}
}

func TestProcessBudgetLimitsAndLegacyReceipts(t *testing.T) {
	m, task, request := processBudgetFixture(t)
	if err := m.RecordExecutionIntent(task.ID, task.Actor, "workspace_process_start", request.Arguments, request); err != nil {
		t.Fatal(err)
	}
	response := processBudgetResponse(request, "owned", "exited", "complete", time.Second, true)
	response.Receipt.Result = []byte(`{"handle":"owned","state":"exited","exit_code":0}`)
	if err := m.RecordExecutionReceipt(task.ID, task.Actor, request, response); err != nil {
		t.Fatal(err)
	}
	saved, _ := m.GetTask(task.ID)
	if b := saved.Checkpoint.ProcessBudget; b.MeasuredNS != 0 || b.ChargedNS != int64(10*time.Second) || b.UnsettledStarts != 1 {
		t.Fatal("old host invented timing", b)
	}
	budgeted, _ := copyTask(saved)
	budgeted.Config.MaxIterations = 1
	budgeted.Checkpoint.ProcessBudget = &ProcessBudget{Version: 1, PerDecisionNS: int64(time.Second), StartsPerDecision: 16}
	if reserveProcessBudget(budgeted, request) == nil {
		t.Fatal("command exceeded available runtime")
	}
	budgeted.Checkpoint.ProcessBudget.PerDecisionNS = int64(time.Hour)
	budgeted.Checkpoint.ProcessBudget.ReservedStarts = 16
	if reserveProcessBudget(budgeted, request) == nil {
		t.Fatal("starts exceeded call allowance")
	}
}

func TestProcessBudgetSchemaTwelveUpgradeKeepsUnknownUsage(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("answer", "fixture", "alice", inputBudgetFixture().Config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(saved *Task) error {
		saved.Status = TaskInterrupted
		saved.Checkpoint.Iteration = 1
		saved.Checkpoint.BudgetedModelCalls = 1
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=12`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if restored.Status != TaskInterrupted || restored.Checkpoint.ProcessBudget != nil {
		t.Fatal("upgrade dispatched or invented measurements")
	}
	r = NewRunner(loaded, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		persisted, _ := loaded.GetTask(task.ID)
		b := persisted.Checkpoint.ProcessBudget
		if b == nil || b.LegacyReservedStarts != 16 || b.UnsettledStarts != 16 || b.MeasuredNS != 0 || b.ChargedNS != 16*max(int64(task.Config.TimeoutPerStep), int64(execution.DefaultProcessTimeout)) {
			t.Error("historical process capacity reopened", b)
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Answer"}), nil
	})
	result, err := r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || result.Status != TaskCompleted {
		t.Fatal(result, err)
	}
	db, err = loaded.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var migrations int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&migrations); err != nil || migrations != 1 {
		t.Fatal("upgrade backup missing", migrations, err)
	}
}
