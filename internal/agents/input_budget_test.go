package agents

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func inputBudgetFixture() *Task {
	cfg := DefaultAgentConfig()
	cfg.ContextWindow, cfg.MaxIterations, cfg.MaxTokens = 4096, 4, 64
	return &Task{Config: cfg, Checkpoint: &Checkpoint{}}
}

func TestInputBudgetChargesBeforeDispatchAndSettlesMeasuredUsageOnce(t *testing.T) {
	task := inputBudgetFixture()
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	if task.Checkpoint.InputBudget.ChargedTokens != 4096 {
		t.Fatal("input not reserved")
	}
	usage := api.Usage{PromptTokens: 200, CompletionTokens: 10, TotalTokens: 210}
	if err := recordDecisionUsage(task, usage); err != nil {
		t.Fatal(err)
	}
	before := *task.Checkpoint.InputBudget
	if before.ChargedTokens != 200 || before.MeasuredTokens != 200 || before.LastSettledCall != 1 {
		t.Fatal(before)
	}
	if err := recordDecisionUsage(task, usage); err == nil || *task.Checkpoint.InputBudget != before {
		t.Fatal("duplicate usage refunded twice")
	}
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	if err := recordDecisionUsage(task, api.Usage{}); err != nil {
		t.Fatal(err)
	}
	if task.Checkpoint.InputBudget.ChargedTokens != 4296 || task.Checkpoint.InputBudget.UnmeasuredCalls != 1 {
		t.Fatal("missing usage was refunded")
	}
	task.Config.ContextWindow = 65536
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	if task.Config.ContextWindow != 4096 || task.Checkpoint.InputBudget.Window != 4096 {
		t.Fatal("input allowance widened")
	}
}

func TestInputBudgetRejectsOversizedUsageBeforeAction(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{}
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		response := taskCall("write_file", `{"path":"unused","content":"unused"}`)
		response.Usage = api.Usage{PromptTokens: 4097, CompletionTokens: 1, TotalTokens: 4098}
		return response, nil
	})
	task, err := r.Create("inspect", "fixture", "alice", inputBudgetFixture().Config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || result.Status != TaskFailed || tools.calls.Load() != 0 || result.PendingApproval != nil {
		t.Fatal("oversized input reached action admission", result, err)
	}
	if result.Checkpoint.InputBudget.ChargedTokens != 4096 || result.Checkpoint.InputBudget.MeasuredTokens != 0 {
		t.Fatal("invalid usage released capacity")
	}
}

func TestInputBudgetLegacyAttemptsAndProbesRemainConservative(t *testing.T) {
	task := inputBudgetFixture()
	task.Checkpoint.Iteration = 1
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	if got := task.Checkpoint.InputBudget; got.ModelCalls != 2 || got.ChargedTokens != 8192 || got.UnmeasuredCalls != 2 {
		t.Fatal("legacy allowance reopened", got)
	}
	if err := reserveSyntheticProbeBudget(task, 1, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := task.Checkpoint.InputBudget; got.ProbeCalls != 1 || got.ChargedTokens != 12288 {
		t.Fatal("probe was not charged", got)
	}
	task.Checkpoint.ReservedChildIterations = 1
	before := *task.Checkpoint.InputBudget
	if err := reserveSyntheticProbeBudget(task, 1, time.Millisecond); err == nil || *task.Checkpoint.InputBudget != before {
		t.Fatal("probe consumed child capacity")
	}
}

func TestInputBudgetFailedGenerationRetainsChargeAndPreparationAcrossRestart(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return nil, errors.New("fixture failure")
	})
	r.ContextWindow = func(*Task) int { return 8192 }
	cfg := inputBudgetFixture().Config
	cfg.ContextWindow = 0
	task, err := r.Create("answer", "fixture", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false); err == nil {
		t.Fatal("expected interrupted generation")
	}
	saved, _ := m.GetTask(task.ID)
	if saved.Config.ContextWindow != 8192 || saved.Checkpoint.InputBudget.ChargedTokens != 8192 || saved.Checkpoint.ContextPreparations != 1 {
		t.Fatal("failed attempt lost costs", saved.Checkpoint)
	}
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if !reflect.DeepEqual(saved.Checkpoint.InputBudget, restored.Checkpoint.InputBudget) || restored.Checkpoint.ContextPreparations != 1 {
		t.Fatal("restart changed charges")
	}
}

func TestInputBudgetWorkContinuationPreservesCharges(t *testing.T) {
	m, r, scope, root, child := completedWorkerWork(t)
	worker, _ := m.WorkerSession(scope, child.Worker.SessionID)
	accepted, err := r.CommandWorker(scope, api.WorkerCommand{RequestID: "input-cost-followup", TargetID: worker.ID, ExpectedRevision: worker.Revision, Kind: "continue", Payload: api.WorkerCommandPayload{Content: "Follow up"}})
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.GetTask(accepted.Worker.RootRunID)
	if !reflect.DeepEqual(root.Checkpoint.InputBudget, next.Checkpoint.InputBudget) || next.Checkpoint.ContextPreparations != root.Checkpoint.ContextPreparations {
		t.Fatal("work attempt reopened input budget")
	}
	if next.Checkpoint.ReservedChildIterations <= root.Checkpoint.ReservedChildIterations {
		t.Fatal("follow-up has no root allocation")
	}
}

func TestInputBudgetSchemaTenUpgradePreservesUnknownCosts(t *testing.T) {
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
	if _, err = db.Exec(`UPDATE agent_schema SET version=10`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	loaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err = loaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	restored, _ := loaded.GetTask(task.ID)
	if restored.Status != TaskInterrupted || restored.Checkpoint.InputBudget != nil {
		t.Fatal("upgrade dispatched or invented measurements")
	}
	r = NewRunner(loaded, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		persisted, _ := loaded.GetTask(task.ID)
		if persisted.Checkpoint.InputBudget == nil || persisted.Checkpoint.InputBudget.ChargedTokens != 8192 {
			t.Error("input not reserved durably before dispatch")
		}
		answer := runAnswer(api.ChatMessage{Role: "assistant", Content: "Answer"})
		answer.Usage = api.Usage{PromptTokens: 100, CompletionTokens: 5, TotalTokens: 105}
		return answer, nil
	})
	result, err := r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || result.Status != TaskCompleted {
		t.Fatal(result, err)
	}
	if got := result.Checkpoint.InputBudget; got.ChargedTokens != 4196 || got.MeasuredTokens != 100 || got.UnmeasuredCalls != 1 {
		t.Fatal("old costs were refunded", got)
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
