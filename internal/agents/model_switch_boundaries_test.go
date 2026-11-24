package agents

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestModelSwitchProcessExitAfterCommit(t *testing.T) {
	if directory := os.Getenv("OFFGRID_MODEL_SWITCH_CRASH_FIXTURE"); directory != "" {
		manager := NewManagerWithPersistence(nil, nil, nil, directory)
		runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
			return nil, rejectedDecision()
		})
		runner.Observer = func(task *Task) {
			if len(task.Checkpoint.ModelSwitches) == 1 {
				os.Exit(73)
			}
		}
		task, err := runner.Create("crash fixture", "primary", "alice", switchFixtureConfig())
		if err != nil {
			t.Fatal(err)
		}
		_, _ = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
		t.Fatal("crash checkpoint not reached")
	}
	directory := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestModelSwitchProcessExitAfterCommit$", "-test.timeout=30s")
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "OFFGRID_") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, "OFFGRID_MODEL_SWITCH_CRASH_FIXTURE="+directory)
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 73 {
		t.Fatalf("crash helper: %v %s", err, output)
	}
	manager := NewManagerWithPersistence(nil, nil, nil, directory)
	tasks := manager.ListTasks()
	if len(tasks) != 1 {
		t.Fatal("lost task", len(tasks))
	}
	saved := tasks[0]
	if saved.Model != "fallback-one" || saved.Status != TaskInterrupted || len(saved.Checkpoint.ModelSwitches) != 1 || saved.Checkpoint.DecisionRepairs != 1 || saved.Checkpoint.BudgetedModelCalls != 1 {
		t.Fatal("crash lost switch or cost", saved)
	}
	calls := 0
	runner := NewRunner(manager, &runTestTools{}, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if task.Model != "fallback-one" || task.Checkpoint.BudgetedModelCalls != 2 {
			t.Fatal("crash replayed selection or refunded cost")
		}
		return runAnswer(api.ChatMessage{Content: "Recovered"}), nil
	})
	finished, err := runner.Continue(context.Background(), saved.ID, "alice", "resume", "", false)
	if err != nil || finished.Status != TaskCompleted || calls != 1 || len(finished.Checkpoint.ModelSwitches) != 1 {
		t.Fatalf("crash recovery: %+v %v", finished, err)
	}
}

func TestModelSwitchFailedPersistenceCannotStartFallback(t *testing.T) {
	dir := t.TempDir()
	manager := NewManagerWithPersistence(nil, nil, nil, dir)
	calls := 0
	runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		db, err := openTaskDatabase(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err = db.Exec(`CREATE TRIGGER fixture_reject_switch BEFORE UPDATE ON agent_tasks BEGIN SELECT RAISE(ABORT, 'fixture persistence failure'); END`); err != nil {
			t.Fatal(err)
		}
		return nil, rejectedDecision()
	})
	task, err := runner.Create("fixture", "primary", "alice", switchFixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err == nil || calls != 1 {
		t.Fatal("failed switch write did not stop inference", err, calls)
	}
	saved, _ := manager.GetTask(task.ID)
	if saved.Model != "primary" || len(saved.Checkpoint.ModelSwitches) != 0 || saved.Checkpoint.BudgetedModelCalls != 1 {
		t.Fatal("uncommitted switch published")
	}
}

func TestModelSwitchLargerWindowPreservesAggregateInputCeiling(t *testing.T) {
	task := inputBudgetFixture() // Four 4096-token units, fixed aggregate 16384.
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	task.Checkpoint.InputBudget.ModelWindow = 8192
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
	budget := task.Checkpoint.InputBudget
	if budget.Window != 4096 || task.Config.ContextWindow != 8192 || budget.ChargedTokens != 12288 {
		t.Fatal("model allocation widened root budget", budget)
	}
	before := *budget
	if err := reserveModelBudget(task); err == nil || *task.Checkpoint.InputBudget != before {
		t.Fatal("larger model escaped original aggregate input ceiling")
	}
	// Settling only this call refunds its actual active window, not the old one.
	if err := recordDecisionUsage(task, api.Usage{PromptTokens: 1024, CompletionTokens: 1, TotalTokens: 1025}); err != nil {
		t.Fatal(err)
	}
	if budget.ChargedTokens != 5120 || budget.Window != 4096 {
		t.Fatal("active allocation refunded incorrect amount", budget)
	}
	if err := reserveModelBudget(task); err != nil {
		t.Fatal(err)
	}
}

func TestModelSwitchStopWinsBeforePublication(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	calls := 0
	runner := NewRunner(manager, &runTestTools{}, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if err := manager.CancelTask(task.ID); err != nil {
			t.Fatal(err)
		}
		return nil, rejectedDecision()
	})
	task, err := runner.Create("fixture", "primary", "alice", switchFixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	saved, _ := manager.GetTask(task.ID)
	if calls != 1 || saved.Status != TaskCancelled || saved.Model != "primary" || len(saved.Checkpoint.ModelSwitches) != 0 {
		t.Fatal("late switch escaped Stop", saved, calls)
	}
}

func TestModelSwitchDelegationKeepsRootInputUnit(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	calls := 0
	runner := NewRunner(manager, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return nil, rejectedDecision()
		}
		return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: `{"tasks":[{"key":"research","goal":"Review supplied facts","tools":[],"depends_on":[]}]}`}}}}), nil
	})
	config := switchFixtureConfig()
	config.DisableDelegation, config.MaxIterations = false, 30
	task, err := runner.Create("Review the supplied facts.", "primary", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	root, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || root.Status != TaskChildren || len(root.ChildHistory) != 1 {
		t.Fatalf("delegation failed: %+v %v", root, err)
	}
	child, ok := manager.GetTask(root.ChildHistory[0].ID)
	if !ok || child.Checkpoint.InputBudget == nil || child.Checkpoint.InputBudget.Window != 8192 || child.Checkpoint.InputBudget.modelWindow() != 16384 {
		t.Fatal("child widened root token unit", child)
	}
	limit := child.Config.MaxIterations * 8192
	for i := 0; i < child.Config.MaxIterations; i++ {
		if err := reserveModelBudget(child); err != nil {
			break
		}
		if child.Checkpoint.InputBudget.ChargedTokens > int64(limit) {
			t.Fatal("child exceeded original root allocation")
		}
	}
	if child.Checkpoint.InputBudget.ModelCalls >= child.Config.MaxIterations {
		t.Fatal("larger context consumed no extra budget")
	}
}
