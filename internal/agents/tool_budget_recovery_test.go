package agents

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestToolBudgetFailedReceiptKeepsDurableReservation(t *testing.T) {
	m, r, task, tools := toolBudgetFixture(t, func(m *Manager, _ string, _ ToolExecution) (string, error) {
		db, err := m.controlDatabase()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		_, err = db.Exec(`CREATE TRIGGER reject_tool_receipt BEFORE UPDATE ON agent_tasks WHEN json_extract(NEW.snapshot,'$.checkpoint.tool_budget.last_settlement') IS NOT NULL BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`)
		if err != nil {
			t.Fatal(err)
		}
		return "effect completed", nil
	})
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false); err == nil {
		t.Fatal("failed receipt accepted")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_tool_receipt`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	b := restored.Checkpoint.ToolBudget
	if b.Pending == nil || b.LastSettlement != nil || b.ChargedNS != int64(time.Minute) || b.UnsettledCalls != 1 || tools.calls.Load() != 1 {
		t.Fatal("missing receipt refunded or replayed operation", b)
	}
}

func TestToolBudgetWorkContinuationPreservesCharges(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	root, err := m.updateTask(root.ID, func(saved *Task) error {
		saved.Checkpoint.ToolBudget = &ToolBudget{Version: 1, CallsPerDecision: 16, PerCallNS: int64(saved.Config.TimeoutPerStep), Attempts: 1, ChargedNS: int64(time.Millisecond), MeasuredNS: int64(time.Millisecond)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	accepted, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "tool-cost-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Follow up"}})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(accepted.Worker.RootRunID)
	if !reflect.DeepEqual(root.Checkpoint.ToolBudget, next.Checkpoint.ToolBudget) {
		t.Fatal("work attempt reopened tool budget")
	}
}

func TestToolBudgetSchemaElevenUpgradePreservesUnknownCosts(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	task, err := r.Create("answer", "fixture", "alice", inputBudgetFixture().Config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(saved *Task) error {
		saved.Status = TaskInterrupted
		saved.Checkpoint.Iteration, saved.Checkpoint.BudgetedModelCalls = 1, 1
		saved.Checkpoint.ChargedCompletionTokens = int64(saved.Config.MaxTokens)
		saved.Checkpoint.ChargedInferenceNS = int64(saved.Config.TimeoutPerStep)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=11`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if restored.Status != TaskInterrupted || restored.Checkpoint.ToolBudget != nil {
		t.Fatal("upgrade dispatched or invented measurements")
	}
	r = NewRunner(loaded, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		persisted, _ := loaded.GetTask(task.ID)
		b := persisted.Checkpoint.ToolBudget
		if b == nil || b.LegacyReservedCalls != 16 || b.Attempts != 16 || b.ChargedNS != 16*int64(task.Config.TimeoutPerStep) || b.MeasuredNS != 0 {
			t.Error("historical tool allowance reopened", b)
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
