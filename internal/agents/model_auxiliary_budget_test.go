package agents

import (
	"context"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestModelAuxiliaryMixedRejectionsStopBeforeFourthInference(t *testing.T) {
	for _, kind := range []string{"skill", "metadata_delegation"} {
		t.Run(kind, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := &runTestTools{}
			calls := 0
			r := NewRunner(m, tools, func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls == 1 {
					return nil, rejectedDecision()
				}
				if calls < 4 {
					if kind == "skill" {
						return runAnswer(api.ChatMessage{Content: "incomplete"}), nil
					}
					if calls == 2 {
						return taskCall("task_plan", `{"items":[]}`), nil
					}
					return taskCall("delegate_tasks", `{"tasks":[{"key":"one","goal":"Review","mode":"edit","depends_on":[],"tools":[]}]}`), nil
				}
				return runAnswer(api.ChatMessage{Content: "DONE\nNext: café 日本語"}), nil
			})
			config := switchFixtureConfig()
			config.DisableDelegation = false
			task, err := r.Create("Keep the original request", "primary", "alice", config)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "skill" {
				task, err = m.updateTask(task.ID, func(current *Task) error { attachContractFixture(current); return nil })
				if err != nil {
					t.Fatal(err)
				}
			}
			result, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil {
				t.Fatal(err)
			}
			if calls != 3 || result.Status != TaskInterrupted || result.ErrorCode != "model_decision_repair_limit" || tools.calls.Load() != 0 {
				t.Fatalf("mixed %s bypass: calls=%d status=%s code=%s effects=%d", kind, calls, result.Status, result.ErrorCode, tools.calls.Load())
			}
			if result.Checkpoint.ModelBudget.Switches != 1 {
				t.Fatal("auxiliary rejection triggered a model switch")
			}
		})
	}
}

func TestModelAuxiliaryDelegatedAnswersShareTheCorrectionLimit(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	for _, child := range children {
		if _, err := m.updateTask(child.ID, func(next *Task) error {
			next.Result = "Reviewed supplied figures"
			finishTask(next, TaskCompleted, "")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	completed := map[string]*Task{}
	for _, link := range root.ChildHistory {
		completed[link.Spec.Key], _ = m.GetTask(link.ID)
	}
	root, err := m.updateTask(root.ID, func(next *Task) error {
		prepareDelegationSynthesis(next, completed)
		next.Status, next.Delegation = TaskRunning, nil
		next.Prompt = "Have Source A check 96 of 120 tasks completed, and 72 of those 96 passed review. Have Source B check 94 of 120 tasks completed, and 72 of those 94 passed review. Write a 150-word briefing showing each completion and pass rate."
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		return runAnswer(api.ChatMessage{Content: "The figures differ."}), nil
	}
	result, err := r.executeUntilBoundary(context.Background(), root, nil)
	if err != nil || calls != 3 || result.Status != TaskInterrupted || result.Checkpoint.DelegationAnswerRepairs != 2 || auxiliaryCount(t, m, root.ID) != 3 {
		t.Fatalf("delegation answer budget bypass: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestModelAuxiliaryPendingWorkspaceCompletionStopsWithoutReplay(t *testing.T) {
	m, r, task := auxiliaryFixture(t)
	r.tools = &auxiliaryCompletionTools{pending: true}
	task.Config.ExecutionRequestVersion = execution.AccessRequestVersion
	if err := r.persist(task); err != nil {
		t.Fatal(err)
	}
	calls := 0
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		return runAnswer(api.ChatMessage{Content: "Done"}), nil
	}
	result, err := r.executeUntilBoundary(context.Background(), task, nil)
	if err != nil || calls != 3 || result.Status != TaskInterrupted || result.Checkpoint.WorkspaceCompletionRepairs != 2 || len(result.Checkpoint.Calls) != 0 || auxiliaryCount(t, m, task.ID) != 3 {
		t.Fatalf("premature completion budget bypass: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestModelAuxiliaryUnverifiedWorkflowSharesTheCorrectionLimit(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := &assessedTestTools{states: map[string]string{"open": "pending"}}
	calls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		return runAnswer(api.ChatMessage{Content: "Done"}), nil
	})
	config := workflowConfig()
	policy := switchFixtureConfig()
	config.ModelSelection, config.ToolAdapter, config.ContextWindow = policy.ModelSelection, policy.ToolAdapter, policy.ContextWindow
	config.NativeExpectations = []OutcomeExpectation{{ID: "open", Title: "Open", Kind: "application_open", ApplicationID: "fixture"}}
	task, err := r.Create("Open and verify the requested application", "primary", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || calls != 3 || result.Status != TaskInterrupted || result.Checkpoint.CompletionRepairs != 2 || tools.calls.Load() != 0 || auxiliaryCount(t, m, task.ID) != 3 {
		t.Fatalf("unverified workflow budget bypass: calls=%d result=%+v err=%v", calls, result, err)
	}
}

func TestModelAuxiliaryConcurrentSiblingChargesBlockAStaleInferenceReservation(t *testing.T) {
	m, r, root, children := modelGraphFixture(t)
	for i := range children {
		var err error
		children[i], err = m.updateTask(children[i].ID, func(next *Task) error { next.Status = TaskRunning; attachContractFixture(next); return nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, child := range children {
		wg.Add(1)
		go func(child *Task) { defer wg.Done(); _, err := r.repairSkillAnswer(child, "incomplete"); results <- err }(child)
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count := auxiliaryCount(t, m, root.ID); count != 2 {
		t.Fatal("lost sibling charge", count)
	}
	stale, _ := copyTask(children[1])
	if _, err := r.repairSkillAnswer(children[0], "still incomplete"); err != nil {
		t.Fatal(err)
	}
	stale.Checkpoint.BudgetedModelCalls++
	if err := r.persist(stale); err == nil {
		t.Fatal("stale sibling reserved inference after third rejection")
	}
	saved, _ := m.GetTask(children[1].ID)
	if saved.Checkpoint.BudgetedModelCalls != 0 || auxiliaryCount(t, m, root.ID) != 3 {
		t.Fatal("failed reservation changed canonical budget")
	}
}
