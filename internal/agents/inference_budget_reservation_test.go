package agents

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestModelBudgetReservationIsDurableAndConservative(t *testing.T) {
	config := DefaultAgentConfig()
	config.MaxIterations, config.MaxTokens, config.TimeoutPerStep = 2, 4, 50*time.Millisecond
	task := &Task{Config: config, Checkpoint: &Checkpoint{}}
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	if task.Checkpoint.BudgetedModelCalls != 1 || task.Checkpoint.ChargedCompletionTokens != 4 || task.Checkpoint.ChargedInferenceNS != int64(50*time.Millisecond) {
		t.Fatalf("first generation was not charged before dispatch: %+v", task.Checkpoint)
	}
	// Simulate restart after an interrupted generation: no response or usage is
	// required to retain the pessimistic charge and attempted-call slot.
	restored, err := copyTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := reserveModelBudget(restored); err != nil {
		t.Fatal(err)
	}
	if err := reserveModelBudget(restored); err == nil || restored.Checkpoint.BudgetedModelCalls != 2 {
		t.Fatalf("restart reopened exhausted attempt budget: %+v, %v", restored.Checkpoint, err)
	}
	if got := restored.Checkpoint.ChargedCompletionTokens; got != 8 {
		t.Fatalf("unknown generation cost was refunded: %d", got)
	}
}

func TestMeasuredModelBudgetRefundsOnlyObservedUnusedCapacity(t *testing.T) {
	config := DefaultAgentConfig()
	config.MaxIterations, config.MaxTokens, config.TimeoutPerStep = 3, 4, 50*time.Millisecond
	task := &Task{Config: config, Checkpoint: &Checkpoint{}}
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	settleModelTime(task, 5*time.Millisecond)
	if err := recordDecisionUsage(task, api.Usage{PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5}); err != nil {
		t.Fatal(err)
	}
	if task.Checkpoint.ChargedCompletionTokens != 2 || task.Checkpoint.MeasuredCompletionTokens != 2 || task.Checkpoint.ChargedInferenceNS != int64(5*time.Millisecond) || task.Checkpoint.MeasuredInferenceNS != int64(5*time.Millisecond) {
		t.Fatalf("measured usage was not reconciled: %+v", task.Checkpoint)
	}
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	if err := recordDecisionUsage(task, api.Usage{}); err != nil {
		t.Fatal(err)
	}
	if task.Checkpoint.UnmeasuredModelCalls != 1 || task.Checkpoint.ChargedCompletionTokens != 6 {
		t.Fatalf("missing usage was treated as zero: %+v", task.Checkpoint)
	}
}

func TestFailedModelCallKeepsDurableChargeBeforeExplicitResume(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	var calls atomic.Int32
	runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		return nil, errors.New("fixture runtime unavailable")
	})
	config := DefaultAgentConfig()
	config.MaxIterations, config.MaxTokens, config.TimeoutPerStep = 1, 4, time.Second
	task, err := runner.Create("answer", "fixture", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err == nil || calls.Load() != 1 {
		t.Fatalf("first failed call did not run exactly once: calls=%d err=%v", calls.Load(), err)
	}
	stored, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
	if !ok || stored.Checkpoint.BudgetedModelCalls != 1 || stored.Checkpoint.ChargedCompletionTokens != 4 || stored.Checkpoint.ChargedInferenceNS != int64(time.Second) {
		t.Fatalf("failed generation was not durably charged: %+v", stored)
	}
	result, err := runner.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || result.Status != TaskFailed || !strings.Contains(result.Error, "aggregate inference budget") || calls.Load() != 1 {
		t.Fatalf("resume repeated exhausted generation: task=%+v calls=%d err=%v", result, calls.Load(), err)
	}
}

func TestChildAllocationCountsFailedParentGeneration(t *testing.T) {
	task := &Task{Config: AgentConfig{MaxIterations: 5}, Checkpoint: &Checkpoint{Iteration: 1, BudgetedModelCalls: 2}}
	if _, err := childDecisionBudget(task, 2); err == nil {
		t.Fatal("failed parent generation was not counted before child admission")
	}
}

func TestSavedComputerProbeChargesRootBeforeLocalInference(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	runner := NewRunner(manager, &runTestTools{}, nil)
	config := DefaultAgentConfig()
	config.TaskFirst = true
	config.MaxIterations, config.MaxTokens = 2, 512
	config.RequestedComputerScope, config.RequestedComputerApprovalMode = "desktop", "full"
	task, err := runner.Create("inspect the test desktop", "fixture", "alice", config)
	if err != nil || task.Status != TaskInput {
		t.Fatalf("expected saved access boundary: %+v, %v", task, err)
	}
	if err := runner.ReserveSyntheticProbeBudget(task.ID, "bob", 384, 3*time.Minute); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("another actor charged the task: %v", err)
	}
	if err := runner.ReserveSyntheticProbeBudget(task.ID, "alice", 384, 3*time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := runner.ReserveSyntheticProbeBudget(task.ID, "alice", 128, 90*time.Second); err != nil {
		t.Fatal(err)
	}
	stored, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
	if !ok || stored.Checkpoint.SyntheticProbeCalls != 2 || stored.Checkpoint.SyntheticProbeTokens != 512 || stored.Checkpoint.ChargedCompletionTokens != 512 || stored.Checkpoint.SyntheticProbeNS != int64(270*time.Second) {
		t.Fatalf("root probe ledger was not durable: %+v", stored)
	}
	if err := runner.ReserveSyntheticProbeBudget(task.ID, "alice", 513, time.Second); !errors.Is(err, ErrInferenceBudget) {
		t.Fatalf("probe exceeded root output cap: %v", err)
	}
	if err := runner.ReserveSyntheticProbeBudget(task.ID, "alice", 1, 10*time.Minute); !errors.Is(err, ErrInferenceBudget) {
		t.Fatalf("probe exceeded root time cap: %v", err)
	}
	stored, _ = manager.GetTask(task.ID)
	if stored.Checkpoint.SyntheticProbeCalls != 2 {
		t.Fatalf("rejected probe consumed a check or modified grants: %+v", stored.Checkpoint)
	}
}

func TestProbeBudgetReducesAvailableChildReservations(t *testing.T) {
	config := DefaultAgentConfig()
	config.MaxIterations, config.MaxTokens, config.TimeoutPerStep = 5, 512, time.Minute
	task := &Task{Config: config, Checkpoint: &Checkpoint{Iteration: 1, BudgetedModelCalls: 1}}
	if err := reserveSyntheticProbeBudget(task, 1536, 3*time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := childDecisionBudget(task, 1); err == nil {
		t.Fatalf("synthetic inference displaced parent verification room: %+v", task.Checkpoint)
	}
}

func TestProbeCannotConsumeAlreadyReservedChildBudget(t *testing.T) {
	config := DefaultAgentConfig()
	config.MaxIterations, config.MaxTokens, config.TimeoutPerStep = 2, 512, time.Minute
	task := &Task{Config: config, Checkpoint: &Checkpoint{ReservedChildIterations: 1}}
	if err := reserveSyntheticProbeBudget(task, 513, time.Second); err == nil {
		t.Fatalf("probe crossed child output reservation: %+v", task.Checkpoint)
	}
	if err := reserveSyntheticProbeBudget(task, 1, time.Minute+time.Second); err == nil {
		t.Fatalf("probe crossed child time reservation: %+v", task.Checkpoint)
	}
}

func TestStopCancelsInFlightSyntheticProbeWithoutGrantingInput(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	runner := NewRunner(manager, &runTestTools{}, nil)
	config := DefaultAgentConfig()
	config.TaskFirst = true
	config.RequestedComputerScope, config.RequestedComputerApprovalMode = "desktop", "full"
	task, err := runner.Create("inspect test desktop", "fixture", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	probeCtx, finish, err := runner.BeginSyntheticProbe(context.Background(), task.ID, "alice", 384, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	runner.mu.Lock()
	_, active := runner.active[task.ID]
	runner.mu.Unlock()
	if !active {
		t.Fatal("synthetic probe did not join the runner's Stop lifecycle")
	}
	if _, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("job started while synthetic probe was active: %v", err)
	}
	stopped, err := runner.Stop(task.ID, "alice", "cancel", "")
	if err != nil || stopped.Status != TaskCancelled {
		t.Fatalf("Stop did not cancel saved task: %+v, %v", stopped, err)
	}
	select {
	case <-probeCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel the in-flight model check")
	}
	finish()
	finish()
	stored, ok := NewManagerWithPersistence(nil, nil, nil, dir).GetTask(task.ID)
	if !ok || stored.Status != TaskCancelled || stored.Config.ComputerSession != "" || stored.Checkpoint.SyntheticProbeCalls != 1 {
		t.Fatalf("probe Stop changed desktop authority or lost budget: %+v", stored)
	}
}

func TestLateSuccessfulModelResultCannotCrossStop(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &runTestTools{}
	started := make(chan struct{})
	runner := NewRunner(manager, tools, func(ctx context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		close(started)
		<-ctx.Done()
		response := runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: `{}`}}}})
		response.Usage = api.Usage{CompletionTokens: 1}
		return response, nil // A backend that ignores cancellation and returns success.
	})
	config := DefaultAgentConfig()
	config.MaxTokens, config.TimeoutPerStep = 4, time.Second
	task, err := runner.Create("write", "fixture", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("model call did not start")
	}
	if _, err := runner.Stop(task.ID, "alice", "cancel", ""); err != nil {
		t.Fatal(err)
	}
	shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Shutdown(shutdown); err != nil {
		t.Fatal(err)
	}
	stored, ok := manager.GetTask(task.ID)
	if !ok || stored.Status != TaskCancelled || tools.calls.Load() != 0 || stored.Checkpoint.ChargedCompletionTokens != 4 {
		t.Fatalf("late response crossed Stop or escaped charge: %+v calls=%d", stored, tools.calls.Load())
	}
}
