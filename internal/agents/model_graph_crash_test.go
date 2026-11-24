package agents

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestModelGraphProcessExitAfterChildSwitch(t *testing.T) {
	const variable = "OFFGRID_MODEL_GRAPH_CRASH_FIXTURE"
	if directory := os.Getenv(variable); directory != "" {
		m := NewManagerWithPersistence(nil, nil, nil, directory)
		r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
			if task.ParentID != "" {
				return nil, rejectedDecision()
			}
			return taskCall("delegate_tasks", `{"tasks":[{"key":"one","goal":"Review one","depends_on":[],"tools":[]},{"key":"two","goal":"Review two","depends_on":[],"tools":[]}]}`), nil
		})
		config := switchFixtureConfig()
		config.DisableDelegation, config.MaxIterations = false, 30
		root, err := r.Create("Isolated crash qualification", "primary", "alice", config)
		if err != nil {
			t.Fatal(err)
		}
		root, err = r.Continue(context.Background(), root.ID, "alice", "start", "", false)
		if err != nil || len(root.ChildHistory) != 2 {
			t.Fatal("delegation failed", err)
		}
		r.Observer = func(task *Task) {
			if task.ParentID != "" && len(task.Checkpoint.ModelSwitches) == 1 {
				os.Exit(74)
			}
		}
		_, _ = r.Continue(context.Background(), root.ChildHistory[0].ID, "alice", "start", "", false)
		t.Fatal("committed child switch not reached")
	}
	directory := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestModelGraphProcessExitAfterChildSwitch$", "-test.timeout=30s")
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "OFFGRID_") {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, variable+"="+directory)
	output, err := command.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 74 {
		t.Fatalf("crash helper: %v %s", err, output)
	}
	m := NewManagerWithPersistence(nil, nil, nil, directory)
	if err = m.StorageError(); err != nil {
		t.Fatal(err)
	}
	var root, switched *Task
	for _, task := range m.ListTasks() {
		if task.ParentID == "" {
			root = task
		} else if len(task.Checkpoint.ModelSwitches) == 1 {
			switched = task
		}
	}
	if root == nil || switched == nil || len(m.ListTasks()) != 3 {
		t.Fatal("partial graph after crash")
	}
	if root.Checkpoint.ModelBudget.Switches != 1 || root.Checkpoint.ModelBudget.Repairs[switched.ID] != 1 || switched.Model != "fallback-one" || switched.Checkpoint.BudgetedModelCalls != 1 || switched.Status != TaskInterrupted {
		t.Fatal("crash lost atomic selection or charges")
	}
	calls := 0
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if task.Model != "fallback-one" || task.Checkpoint.BudgetedModelCalls != 2 {
			t.Fatal("restart replayed prior model or refunded generation")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Reviewed the supplied information"}), nil
	})
	resumed, err := r.Continue(context.Background(), root.ID, "alice", "resume", "", false)
	if err != nil || resumed.Status != TaskChildren || calls != 0 {
		t.Fatalf("parent resume dispatched model or lost graph: %+v %v calls=%d", resumed, err, calls)
	}
	finished, err := r.Continue(context.Background(), switched.ID, "alice", "start", "", false)
	if err != nil || finished.Status != TaskCompleted || calls != 1 {
		t.Fatalf("recovery failed: %+v %v", finished, err)
	}
	saved, _ := m.GetTask(root.ID)
	if saved.Checkpoint.ModelBudget.Switches != 1 || len(saved.Checkpoint.ModelBudget.Repairs) != 0 {
		t.Fatal("recovery refunded switch or lost resolution")
	}
	for _, link := range saved.ChildHistory {
		if link.ID == switched.ID {
			continue
		}
		other, _ := m.GetTask(link.ID)
		if other.Checkpoint.BudgetedModelCalls != 0 || other.Model != "primary" {
			t.Fatal("recovery automatically ran sibling")
		}
	}
}

func TestModelGraphRejectsInvalidPersistedBudget(t *testing.T) {
	for _, mutate := range []func(*ModelGraphBudget){
		func(b *ModelGraphBudget) { b.Version++ },
		func(b *ModelGraphBudget) { b.Switches = 3 },
		func(b *ModelGraphBudget) { b.Repairs = map[string]int{"": 1} },
		func(b *ModelGraphBudget) { b.Repairs = map[string]int{"run": -1} },
	} {
		m, _, root, _ := modelGraphFixture(t)
		_, err := m.updateTask(root.ID, func(task *Task) error { mutate(task.Checkpoint.ModelBudget); return nil })
		if err == nil {
			t.Fatal("invalid accounting published")
		}
		saved, _ := m.GetTask(root.ID)
		if _, err := saved.Checkpoint.ModelBudget.corrections(); err != nil {
			t.Fatal("failed save changed live budget", err)
		}
	}
}

func TestModelGraphRepeatedResumeCannotRefundProposalDebt(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	calls := 0
	r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		return runAnswer(api.ChatMessage{Content: "must not run"}), nil
	})
	root, err := r.Create("Retained unresolved correction", "primary", "alice", switchFixtureConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(root.ID, func(task *Task) error {
		task.Status, task.ErrorCode = TaskInterrupted, proposalRepairLimitCode
		task.Checkpoint.ProposalRepairs = 3
		task.Checkpoint.ModelBudget = &ModelGraphBudget{Version: 1, Repairs: map[string]int{task.ID: 3}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		stopped, err := r.Continue(context.Background(), root.ID, root.Actor, "resume", "", false)
		if err != nil || stopped.Status != TaskInterrupted || calls != 0 {
			t.Fatalf("resume reset shared correction limit: %+v %v calls=%d", stopped, err, calls)
		}
		saved, _ := m.GetTask(root.ID)
		if saved.Checkpoint.ModelBudget.Repairs[root.ID] != 3 {
			t.Fatal("local proposal cleanup refunded shared debt")
		}
	}
}
