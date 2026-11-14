package agents

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func toolBudgetFixture(t *testing.T, execute func(*Manager, string, ToolExecution) (string, error)) (*Manager, *Runner, *Task, *runTestTools) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error { return nil }}
	tools.execute = func(_ string, _ json.RawMessage, grant ToolExecution) (string, error) {
		return execute(m, grant.RunID, grant)
	}
	var calls atomic.Int32
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		if calls.Add(1) == 1 {
			return taskCall("test_write", `{}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Finished"}), nil
	})
	cfg := inputBudgetFixture().Config
	cfg.TimeoutPerStep = time.Minute
	task, err := r.Create("Use the fixture tool", "fixture", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	return m, r, task, tools
}

func TestToolBudgetReservesBeforeDispatchAndSettlesOnce(t *testing.T) {
	m, r, task, tools := toolBudgetFixture(t, func(m *Manager, id string, grant ToolExecution) (string, error) {
		saved, _ := m.GetTask(id)
		b := saved.Checkpoint.ToolBudget
		if b == nil || b.Pending == nil || b.Pending.CallID != grant.CallID || b.Attempts != 1 || b.ChargedNS != int64(time.Minute) || saved.Checkpoint.ExecutingCall != grant.CallID {
			t.Error("tool ran without durable reservation", saved.Checkpoint)
		}
		return "recorded result", nil
	})
	result, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || result.Status != TaskCompleted || tools.calls.Load() != 1 {
		t.Fatal(result, err)
	}
	b := result.Checkpoint.ToolBudget
	if b.Attempts != 1 || b.Pending != nil || b.UnsettledCalls != 0 || b.ChargedNS != b.MeasuredNS || b.ChargedNS >= int64(time.Minute) {
		t.Fatal("unused runtime not released", b)
	}
	before := *b
	settleToolBudget(result, b.LastSettlement.CallID, 0, true)
	if !reflect.DeepEqual(before, *b) {
		t.Fatal("duplicate settlement released runtime twice")
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if !reflect.DeepEqual(b, restored.Checkpoint.ToolBudget) {
		t.Fatal("restart changed settled costs")
	}
}

func TestToolBudgetUnknownOutcomeKeepsChargeAcrossRestart(t *testing.T) {
	m, r, task, tools := toolBudgetFixture(t, func(*Manager, string, ToolExecution) (string, error) {
		return "", errors.New("fixture lost acknowledgment")
	})
	result, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err == nil || result.Status != TaskUncertain {
		t.Fatal(result, err)
	}
	b := result.Checkpoint.ToolBudget
	if b.ChargedNS != int64(time.Minute) || b.UnsettledCalls != 1 || b.LastSettlement == nil || b.LastSettlement.Settled {
		t.Fatal("unknown operation refunded", b)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if restored.Status != TaskUncertain || !reflect.DeepEqual(b, restored.Checkpoint.ToolBudget) || tools.calls.Load() != 1 {
		t.Fatal("restart replayed or refunded unknown work")
	}
}

func TestToolBudgetLateResultAfterStopOnlyRecordsUsage(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	m, r, task, tools := toolBudgetFixture(t, func(*Manager, string, ToolExecution) (string, error) {
		close(entered)
		<-release
		return "late result", nil
	})
	done := make(chan struct{})
	go func() { _, _ = r.Continue(context.Background(), task.ID, "alice", "start", "", false); close(done) }()
	<-entered
	stopped, err := r.Stop(task.ID, "alice", "cancel", "")
	if err != nil {
		t.Fatal(err)
	}
	close(release)
	<-done
	saved, _ := m.GetTask(task.ID)
	if saved.Status != stopped.Status || saved.Status == TaskCompleted || saved.Result != "" || tools.calls.Load() != 1 {
		t.Fatal("late tool result revived work", saved)
	}
	b := saved.Checkpoint.ToolBudget
	if b.LastSettlement == nil || b.LastSettlement.Settled || b.ChargedNS != int64(time.Minute) || b.MeasuredNS <= 0 {
		t.Fatal("late receipt lost usage or refunded cancellation", b)
	}
}

func TestToolBudgetFailedReservationCannotDispatch(t *testing.T) {
	m, r, task, tools := toolBudgetFixture(t, func(*Manager, string, ToolExecution) (string, error) { return "unexpected", nil })
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_tool_reservation BEFORE UPDATE ON agent_tasks WHEN json_extract(NEW.snapshot,'$.checkpoint.tool_budget.pending') IS NOT NULL BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false); err == nil {
		t.Fatal("failed reservation accepted")
	}
	if tools.calls.Load() != 0 {
		t.Fatal("tool dispatched without committed allowance")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_tool_reservation`); err != nil {
		t.Fatal(err)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if restored.Checkpoint.ToolBudget.Attempts != 0 || restored.Checkpoint.ExecutingCall != "" {
		t.Fatal("failed reservation became a dispatched call")
	}
}

func TestToolBudgetUnknownRetriesAndOverrunsCannotReopenAllowance(t *testing.T) {
	task := inputBudgetFixture()
	task.Config.MaxIterations, task.Config.TimeoutPerStep = 1, time.Second
	for i := 0; i < maxToolsPerDecision; i++ {
		if err := reserveToolBudget(task, "same-read"); err != nil {
			t.Fatal(err)
		}
		settleToolBudget(task, "same-read", time.Millisecond, false)
	}
	if err := reserveToolBudget(task, "same-read"); err == nil {
		t.Fatal("retry reopened exhausted tool allowance")
	}
	if task.Checkpoint.ToolBudget.ChargedNS != int64(maxToolsPerDecision*time.Second) {
		t.Fatal("unknown attempts refunded themselves")
	}
	task = inputBudgetFixture()
	task.Config.TimeoutPerStep = time.Second
	if err := reserveToolBudget(task, "slow"); err != nil {
		t.Fatal(err)
	}
	settleToolBudget(task, "slow", 2*time.Second, false)
	if b := task.Checkpoint.ToolBudget; b.ChargedNS != int64(2*time.Second) || b.MeasuredNS != int64(2*time.Second) {
		t.Fatal("deadline presented as a measured cap", b)
	}
}
