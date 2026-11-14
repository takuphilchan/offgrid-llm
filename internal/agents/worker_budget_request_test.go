package agents

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func workerBudgetRequestFixture(t *testing.T) (*Manager, *Runner, ControlScope, *Task, *Task, api.ToolCall) {
	t.Helper()
	m, r, scope, root, child := workerFixture(t)
	call := api.ToolCall{ID: "budget-once", Type: "function", Function: api.FunctionCall{Name: "worker_request_budget", Arguments: `{"model_calls":2,"reason":"Validate the remaining comparisons"}`}}
	child, err := m.updateTask(child.ID, func(task *Task) error {
		task.Status = TaskRunning
		task.Config.TaskFirst = true
		task.Checkpoint.Calls = []api.ToolCall{call}
		task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, r, scope, root, child, call
}

func TestWorkerBudgetRequestConcurrentRetryAndRestart(t *testing.T) {
	m, r, scope, root, child, call := workerBudgetRequestFixture(t)
	before := root.Checkpoint.ReservedChildIterations
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy, _ := copyTask(child)
			data, err := r.requestWorkerBudget(copy, call)
			var receipt WorkerBudgetGrant
			if err != nil || json.Unmarshal([]byte(data), &receipt) != nil || receipt.Granted != 2 {
				t.Errorf("grant: %s %v", data, err)
			}
		}()
	}
	wg.Wait()
	parent, _ := m.GetTask(root.ID)
	current, _ := m.GetTask(child.ID)
	if parent.Checkpoint.ReservedChildIterations != before+2 || current.Config.MaxIterations != child.Config.MaxIterations+2 || parent.Config.MaxIterations != root.Config.MaxIterations || len(current.Checkpoint.WorkerBudgetRequests) != 1 {
		t.Fatal("retry reopened root budget")
	}
	worker, err := m.WorkerSession(scope, child.Worker.SessionID)
	if err != nil || worker.Assignment.ModelCalls != current.Config.MaxIterations {
		t.Fatal("public allocation stale", worker, err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	other := NewRunner(reloaded, NewToolRegistry(), nil)
	restored, _ := reloaded.GetTask(child.ID)
	if restored.Status != TaskInterrupted {
		t.Fatal("restart resumed worker", restored.Status)
	}
	if _, err := other.requestWorkerBudget(restored, call); err != nil {
		t.Fatal("lost receipt not retrievable", err)
	}
	parent, _ = reloaded.GetTask(root.ID)
	if parent.Checkpoint.ReservedChildIterations != before+2 {
		t.Fatal("restart duplicated allocation")
	}
	changed := call
	changed.Function.Arguments = `{"model_calls":3,"reason":"Different request"}`
	if _, err := other.requestWorkerBudget(restored, changed); !errors.Is(err, ErrRunConflict) {
		t.Fatal("conflicting request identity", err)
	}
}

func TestWorkerBudgetRequestHonorsParentReserveAndControls(t *testing.T) {
	for _, kind := range []string{"reserve", "stop", "correction", "profile"} {
		t.Run(kind, func(t *testing.T) {
			m, r, _, root, child, call := workerBudgetRequestFixture(t)
			if kind == "profile" {
				var err error
				child, err = m.updateTask(child.ID, func(task *Task) error {
					task.Config.Profile = &AgentProfile{Settings: ProfileSettings{MaxIterations: task.Config.MaxIterations}}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			} else {
				_, err := m.updateTask(root.ID, func(task *Task) error {
					switch kind {
					case "reserve":
						task.Config.MaxIterations = task.Checkpoint.ReservedChildIterations + max(task.Checkpoint.Iteration, task.Checkpoint.BudgetedModelCalls) + 2
					case "stop":
						task.ControlAction = "stop"
						task.ControlPending = true
					case "correction":
						task.PendingInstructions = append(task.PendingInstructions, Instruction{RequestID: "correction", Digest: "fixture", Text: "Change direction"})
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := r.requestWorkerBudget(child, call); err == nil {
				t.Fatal("budget granted despite", kind)
			}
			current, _ := m.GetTask(child.ID)
			if current.Config.MaxIterations != child.Config.MaxIterations || len(current.Checkpoint.WorkerBudgetRequests) != 0 {
				t.Fatal("denial mutated budget")
			}
		})
	}
}

func TestWorkerBudgetRequestReceiptFailureRollsBackBothLedgers(t *testing.T) {
	m, r, _, root, child, call := workerBudgetRequestFixture(t)
	db, err := m.controlDatabase()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TRIGGER reject_budget BEFORE UPDATE ON agent_tasks WHEN OLD.id='` + child.ID + `' BEGIN SELECT RAISE(ABORT,'budget receipt failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = r.requestWorkerBudget(child, call); err == nil {
		t.Fatal("injected failure ignored")
	}
	parent, _ := m.GetTask(root.ID)
	if parent.Checkpoint.ReservedChildIterations != root.Checkpoint.ReservedChildIterations {
		t.Fatal("uncommitted reservation published")
	}
	if _, err = db.Exec(`DROP TRIGGER reject_budget`); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	parent, _ = reloaded.GetTask(root.ID)
	current, _ := reloaded.GetTask(child.ID)
	if parent.Checkpoint.ReservedChildIterations != root.Checkpoint.ReservedChildIterations || current.Config.MaxIterations != child.Config.MaxIterations || len(current.Checkpoint.WorkerBudgetRequests) != 0 {
		t.Fatal("partial budget commit survived recovery")
	}
}

func TestWorkerBudgetRequestRunsThroughNormalToolCheckpoint(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	if _, err := m.updateTask(child.ID, func(task *Task) error { task.Config.TaskFirst = true; return nil }); err != nil {
		t.Fatal(err)
	}
	calls := 0
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			found := false
			for _, tool := range tools {
				found = found || tool.Function.Name == "worker_request_budget"
			}
			if !found {
				t.Error("budget request missing")
			}
			return runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "normal-budget", Type: "function", Function: api.FunctionCall{Name: "worker_request_budget", Arguments: `{"model_calls":2,"reason":"Verify remaining evidence"}`}}}}), nil
		}
		if task.Config.MaxIterations != child.Config.MaxIterations+2 {
			t.Error("next generation lost allocation")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Verified comparison"}), nil
	}
	done, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false)
	if err != nil || done.Status != TaskCompleted || calls != 2 {
		t.Fatalf("worker: %v %v calls=%d", done, err, calls)
	}
	parent, _ := m.GetTask(root.ID)
	if parent.Config.MaxIterations != root.Config.MaxIterations {
		t.Fatal("root budget increased")
	}
}
