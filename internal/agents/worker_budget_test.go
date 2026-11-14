package agents

import (
	"context"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerBudgetSettlementKeepsUnknownAndSpentCharges(t *testing.T) {
	for _, tc := range []struct {
		name     string
		change   func(*Task)
		retained int
		settles  bool
	}{
		{"completed", func(*Task) {}, 1, true},
		{"unused cancelled", func(c *Task) { c.Status = TaskCancelled; c.Checkpoint = &Checkpoint{} }, 0, true},
		{"failed attempts", func(c *Task) { c.Status = TaskFailed; c.Checkpoint.BudgetedModelCalls = 3 }, 3, true},
		{"unmeasured output", func(c *Task) { c.Checkpoint.ChargedCompletionTokens = 201 }, 3, true},
		{"unmeasured time", func(c *Task) { c.Checkpoint.ChargedInferenceNS = int64(3 * time.Second) }, 3, true},
		{"process starts", func(c *Task) {
			c.Checkpoint.ProcessBudget = &ProcessBudget{Version: 1, PerDecisionNS: int64(16 * 180 * time.Second), StartsPerDecision: 16, ReservedStarts: 33, ChargedNS: int64(time.Second)}
		}, 3, true},
		{"process time", func(c *Task) {
			c.Checkpoint.ProcessBudget = &ProcessBudget{Version: 1, PerDecisionNS: int64(16 * 180 * time.Second), StartsPerDecision: 16, ReservedStarts: 1, ChargedNS: int64(33 * 180 * time.Second)}
		}, 3, true},
		{"tool retries", func(c *Task) {
			c.Checkpoint.ToolBudget = &ToolBudget{Version: 1, CallsPerDecision: 16, PerCallNS: int64(time.Second), Attempts: 33, ChargedNS: int64(33 * time.Second), UnsettledCalls: 33}
		}, 3, true},
		{"tool overrun", func(c *Task) {
			c.Checkpoint.ToolBudget = &ToolBudget{Version: 1, CallsPerDecision: 16, PerCallNS: int64(time.Second), Attempts: 1, ChargedNS: int64(33 * time.Second), MeasuredNS: int64(33 * time.Second)}
		}, 3, true},
		{"legacy accepted decisions", func(c *Task) { c.Checkpoint.BudgetedModelCalls = 0; c.Checkpoint.Iteration = 2 }, 2, true},
		{"uncertain", func(c *Task) { c.Status = TaskUncertain }, 4, false},
		{"interrupted", func(c *Task) { c.Status = TaskInterrupted }, 4, false},
		{"unsettled process", func(c *Task) { c.ProcessStopPending = true }, 4, false},
		{"unsettled call", func(c *Task) { c.Checkpoint.ExecutingCall = "pending" }, 4, false},
		{"missing checkpoint", func(c *Task) { c.Checkpoint = nil }, 4, false},
		{"completed without usage evidence", func(c *Task) { c.Checkpoint = &Checkpoint{} }, 4, false},
		{"started without usage evidence", func(c *Task) {
			c.Status = TaskCancelled
			now := time.Now()
			c.StartedAt = &now
			c.Checkpoint = &Checkpoint{}
		}, 4, false},
		{"invalid charges", func(c *Task) { c.Checkpoint.ChargedInferenceNS = -1 }, 4, false},
		{"overrun", func(c *Task) { c.Checkpoint.BudgetedModelCalls = 5 }, 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := AgentConfig{MaxIterations: 4, MaxTokens: 100, TimeoutPerStep: time.Second}
			parent := &Task{ID: "parent", WorkID: "work", Actor: "actor", Config: config, Checkpoint: &Checkpoint{ReservedChildIterations: 4}, ChildHistory: []ChildLink{{ID: "child"}}}
			child := &Task{ID: "child", ParentID: parent.ID, Actor: parent.Actor, Worker: &WorkerBinding{WorkID: parent.WorkID}, Status: TaskCompleted, Config: config, Checkpoint: &Checkpoint{Iteration: 1, BudgetedModelCalls: 1, ChargedCompletionTokens: 100, ChargedInferenceNS: int64(time.Second)}}
			tc.change(child)
			r := &Runner{manager: &Manager{tasks: map[string]*Task{child.ID: child}}}
			if got := r.settleChildBudgetsLocked(parent); got != tc.settles || parent.Checkpoint.ReservedChildIterations != tc.retained {
				t.Fatalf("settled=%v retained=%d, want %v/%d", got, parent.Checkpoint.ReservedChildIterations, tc.settles, tc.retained)
			}
			if r.settleChildBudgetsLocked(parent) {
				t.Fatal("duplicate settlement refunded twice")
			}
			if restoreChildReservations([]*Task{parent, child})[parent.ID] || parent.Checkpoint.ReservedChildIterations != tc.retained {
				t.Fatal("restart changed the committed reservation")
			}
		})
	}
}

func TestWorkerBudgetSettlementPersistsAcrossRestart(t *testing.T) {
	m, _, _, root, child := completedWorkerWork(t)
	receipt, ok := root.Checkpoint.ChildBudgetSettlements[child.ID]
	if !ok || receipt.Allocated != child.Config.MaxIterations || receipt.Retained != 1 || root.Checkpoint.ReservedChildIterations != 1 {
		t.Fatal("unused worker capacity was not returned", root.Checkpoint)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(root.ID)
	if restored.Checkpoint.ReservedChildIterations != 1 || restored.Checkpoint.ChildBudgetSettlements[child.ID] != receipt {
		t.Fatal("restart lost settlement", restored.Checkpoint)
	}
}

func TestWorkerBudgetReceiptFailureCannotExposeRefund(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_settlement BEFORE UPDATE ON agent_tasks WHEN OLD.id='` + root.ID + `' BEGIN SELECT RAISE(ABORT,'injected receipt failure'); END`); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	m.mu.Lock()
	candidate, _ := copyTask(m.tasks[root.ID])
	changed := r.settleChildBudgetsLocked(candidate)
	err = m.saveTask(candidate)
	m.mu.Unlock()
	r.mu.Unlock()
	if !changed || err == nil {
		t.Fatal("fault did not reject settlement", changed, err)
	}
	unchanged, _ := m.GetTask(root.ID)
	if unchanged.Checkpoint.ReservedChildIterations != child.Config.MaxIterations {
		t.Fatal("uncommitted refund escaped")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_settlement`); err != nil {
		t.Fatal(err)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(root.ID)
	if restored.Checkpoint.ReservedChildIterations != child.Config.MaxIterations || len(restored.Checkpoint.ChildBudgetSettlements) != 0 {
		t.Fatal("recovery invented a refund")
	}
}

func TestWorkerBudgetSettlementWaitsForGenerationRelease(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	r.active[child.ID] = func() {}
	r.mu.Unlock()
	r.advanceDelegation(context.Background(), root.ID)
	waiting, _ := m.GetTask(root.ID)
	if waiting.Status != TaskChildren || waiting.Checkpoint.ReservedChildIterations != child.Config.MaxIterations {
		t.Fatal("active generation was refunded")
	}
	r.mu.Lock()
	delete(r.active, child.ID)
	r.mu.Unlock()
}

func TestWorkerBudgetRecoveryCountsReceiptsAndInheritedCosts(t *testing.T) {
	config := AgentConfig{MaxIterations: 4, MaxTokens: 100, TimeoutPerStep: time.Second}
	parent := &Task{ID: "parent", WorkID: "work", Config: config, Checkpoint: &Checkpoint{InheritedChildIterations: 2, ReservedChildIterations: 4, ChildBudgetSettlements: map[string]ChildBudgetSettlement{}}, ChildHistory: []ChildLink{{ID: "one"}, {ID: "two"}}}
	var tasks []*Task
	tasks = append(tasks, parent)
	for _, link := range parent.ChildHistory {
		child := &Task{ID: link.ID, ParentID: parent.ID, Worker: &WorkerBinding{WorkID: parent.WorkID}, Status: TaskCompleted, Config: config, Checkpoint: &Checkpoint{BudgetedModelCalls: 1}}
		tasks = append(tasks, child)
		parent.Checkpoint.ChildBudgetSettlements[child.ID] = ChildBudgetSettlement{Allocated: 4, Retained: 1}
	}
	if restoreChildReservations(tasks)[parent.ID] || parent.Checkpoint.ReservedChildIterations != 4 {
		t.Fatal("valid receipts were charged twice")
	}
	parent.Checkpoint.ReservedChildIterations = 0
	if !restoreChildReservations(tasks)[parent.ID] || parent.Checkpoint.ReservedChildIterations != 4 {
		t.Fatal("prior attempts regained spent capacity")
	}
	parent.Config.MaxIterations = 12
	parent.Checkpoint.ChildBudgetSettlements["one"] = ChildBudgetSettlement{Allocated: 4, Retained: 0}
	if !restoreChildReservations(tasks)[parent.ID] || parent.Checkpoint.ReservedChildIterations != 7 {
		t.Fatal("invalid receipt released charged calls")
	}
}

func TestWorkerBudgetContinuationRefundCommitsWithCommand(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	command := api.WorkerCommand{RequestID: "settle-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Follow up"}}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_refund_command BEFORE INSERT ON agent_control_commands WHEN NEW.request_id='settle-followup' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CommandWorker(scope, command); err == nil {
		t.Fatal("receipt failure accepted a follow-up")
	}
	unchanged, _ := m.GetTask(root.ID)
	if unchanged.Checkpoint.ReservedChildIterations != child.Config.MaxIterations || len(unchanged.Checkpoint.ChildBudgetSettlements) != 0 {
		t.Fatal("uncommitted settlement published")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_refund_command`); err != nil {
		t.Fatal(err)
	}
	accepted, err := r.CommandWorker(scope, command)
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(accepted.Worker.CurrentRunID)
	for i := 0; i < 2; i++ {
		if _, err = r.CommandWorker(scope, command); err != nil {
			t.Fatal(err)
		}
		current, _ := m.GetTask(root.ID)
		if current.Checkpoint.ReservedChildIterations != 1+next.Config.MaxIterations || len(current.Checkpoint.ChildBudgetSettlements) != 1 {
			t.Fatal("retry charged or refunded twice")
		}
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	current, _ := loaded.GetTask(root.ID)
	if current.Checkpoint.ReservedChildIterations != 1+next.Config.MaxIterations {
		t.Fatal("restart changed committed allocation")
	}
}

func TestWorkerBudgetSchemaNineUpgradeDoesNotInventSettlement(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=9`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(root.ID)
	if restored.Status != TaskInterrupted || restored.Checkpoint.ReservedChildIterations != child.Config.MaxIterations || len(restored.Checkpoint.ChildBudgetSettlements) != 0 {
		t.Fatal("upgrade dispatched work or invented a refund")
	}
	db, err = loaded.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err = db.QueryRow(`SELECT COUNT(*) FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&count); err != nil || count != 1 {
		t.Fatal("upgrade backup absent", count, err)
	}
}
