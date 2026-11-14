package agents

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func partialSynthesisFixture(t *testing.T) (*Manager, *Runner, ControlScope, *Task) {
	t.Helper()
	m, r, scope, c := controlFixture(t)
	t.Cleanup(func() { r.Shutdown(context.Background()) })
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("delegate_tasks", `{"tasks":[{"key":"source","goal":"Find missing evidence","depends_on":[],"tools":[]},{"key":"independent","goal":"Review available evidence","depends_on":[],"tools":[]}]}`), nil
	}
	work := admitControlTestWork(t, r, scope, c)
	root, err := r.Continue(context.Background(), work.RunID, scope.Actor, "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range root.ChildHistory {
		if _, err = m.updateTask(link.ID, func(task *Task) error {
			if link.Spec.Key == "source" {
				finishTask(task, TaskFailed, "The missing source is unavailable.")
			} else {
				task.Result = "Independent evidence [1]."
				finishTask(task, TaskCompleted, "")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	return m, r, scope, root
}

func TestPartialSynthesisRetainsFailuresAndUsesRootBudget(t *testing.T) {
	m, r, _, root := partialSynthesisFixture(t)
	var calls atomic.Int32
	r.caller = func(_ context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		calls.Add(1)
		if !task.Checkpoint.PartialSynthesis || task.Config.MaxIterations != root.Config.MaxIterations || task.Checkpoint.BudgetedModelCalls != root.Checkpoint.BudgetedModelCalls+1 {
			t.Error("synthesis reopened or lost the original allowance")
		}
		if len(tools) != 1 || tools[0].Function.Name != "task_history" {
			t.Error("partial synthesis can dispatch new work", tools)
		}
		found := false
		for _, message := range messages {
			found = found || strings.Contains(message.StringContent(), "Independent evidence [1].")
		}
		if !found {
			t.Error("independent output missing from synthesis context")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Useful findings [1]; the missing source prevents a complete comparison."}), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	done := awaitTask(t, m, root.ID, TaskFailed)
	settleTask(t, r, root.ID)
	if calls.Load() != 1 || done.Checkpoint.PartialSynthesis || done.ErrorCode != "delegation_partial" || !strings.Contains(done.Result, "Useful findings [1]") || !strings.Contains(done.Result, "source (failed)") || !strings.Contains(done.Result, "Saved result: result:") {
		t.Fatal("synthesis lost evidence or claimed completion", done.Result, done.ErrorCode, calls.Load())
	}
}

func TestPartialSynthesisFallbackWithoutBudgetOrModel(t *testing.T) {
	for _, mode := range []string{"budget", "model", "provider", "revoked"} {
		t.Run(mode, func(t *testing.T) {
			m, r, _, root := partialSynthesisFixture(t)
			var calls atomic.Int32
			r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls.Add(1)
				return nil, errors.New("fixture provider unavailable")
			}
			switch mode {
			case "budget":
				if _, err := m.updateTask(root.ID, func(task *Task) error {
					task.Checkpoint.BudgetedModelCalls = task.Config.MaxIterations
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			case "model":
				r.caller = nil
			case "revoked":
				if _, err := m.updateTask(root.ChildHistory[1].ID, func(task *Task) error {
					task.Config.KnowledgeDependencies = []string{"revoked-source"}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				r.SourceAccess = func(task *Task) bool { return !containsTool(task.Config.KnowledgeDependencies, "revoked-source") }
			}
			r.advanceDelegation(context.Background(), root.ID)
			done := awaitTask(t, m, root.ID, TaskFailed)
			settleTask(t, r, root.ID)
			if !strings.Contains(done.Result, "Independent evidence [1].") || !strings.Contains(done.Result, "source (failed)") {
				t.Fatal("failed synthesis erased the deterministic outcome", done.Result)
			}
			if mode != "provider" && calls.Load() != 0 || mode == "provider" && calls.Load() != 1 {
				t.Fatal("fallback retried inference or bypassed admission", calls.Load())
			}
			if mode == "revoked" && (done.ErrorCode != "knowledge_unavailable" || !containsTool(done.Config.KnowledgeDependencies, "revoked-source")) {
				t.Fatal("parent output dropped the child's source-access dependency")
			}
			if mode != "revoked" && done.ErrorCode != "delegation_partial" {
				t.Fatal("fallback lost its partial-outcome classification", done.ErrorCode)
			}
		})
	}
}

func TestPartialSynthesisStopRejectsLateCompletion(t *testing.T) {
	m, r, scope, root := partialSynthesisFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		close(entered)
		<-release
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Late synthesis"}), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	<-entered
	work, _ := m.WorkItem(scope, root.WorkID)
	_, err := r.ControlWork(scope, api.WorkCommand{RequestID: "stop-partial", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "stop"})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	settleTask(t, r, root.ID)
	stopped, _ := m.GetTask(root.ID)
	if stopped.Status != TaskCancelled || strings.Contains(stopped.Result, "Late synthesis") || !strings.Contains(stopped.Result, "Independent evidence") {
		t.Fatal("late response replaced Stop or erased settled evidence")
	}
}

func TestPartialSynthesisRestartRequiresExplicitContinuation(t *testing.T) {
	m, r, scope, root := partialSynthesisFixture(t)
	entered := make(chan struct{})
	r.caller = func(ctx context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	r.advanceDelegation(context.Background(), root.ID)
	<-entered
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := reloaded.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, _ := reloaded.GetTask(root.ID)
	if saved.Status != TaskInterrupted || !saved.Checkpoint.PartialSynthesis || !strings.Contains(saved.Result, "Independent evidence") {
		t.Fatal("restart lost the partial result or resumed synthesis")
	}
	restarted := NewRunner(reloaded, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Recovered partial findings."}), nil
	})
	defer restarted.Shutdown(context.Background())
	done, err := restarted.Continue(context.Background(), saved.ID, scope.Actor, "resume", "", false)
	if err != nil || done.Status != TaskFailed || !strings.Contains(done.Result, "Recovered partial findings.") || done.Checkpoint.BudgetedModelCalls != saved.Checkpoint.BudgetedModelCalls+1 {
		t.Fatal("explicit continuation lost outcome or reopened allowance", done, err)
	}
}

func TestPartialSynthesisRejectsNewWorkAtDispatch(t *testing.T) {
	m, r, _, root := partialSynthesisFixture(t)
	r.caller = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("delegate_tasks", `{"tasks":[{"key":"retry","goal":"Retry failed work","depends_on":[],"tools":[]}]}`), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	done := awaitTask(t, m, root.ID, TaskFailed)
	settleTask(t, r, root.ID)
	if len(m.ListTasks()) != 3 || done.Delegation != nil || !strings.Contains(done.Result, "Independent evidence") {
		t.Fatal("partial synthesis dispatched new work or erased the saved result")
	}
}

func TestPartialSynthesisCorrectionEndsRestrictedStage(t *testing.T) {
	m, r, scope, root := partialSynthesisFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var turns atomic.Int32
	r.caller = func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if turns.Add(1) == 1 {
			close(entered)
			<-release
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "Obsolete partial synthesis."}), nil
		}
		if task.Checkpoint.PartialSynthesis || task.InstructionVersion != root.InstructionVersion+1 {
			t.Error("correction did not supersede partial synthesis")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Independent evidence only."}), nil
	}
	r.advanceDelegation(context.Background(), root.ID)
	<-entered
	work, _ := m.WorkItem(scope, root.WorkID)
	_, err := r.ControlWork(scope, api.WorkCommand{RequestID: "correct-partial", TargetID: work.ID, ExpectedRevision: work.Revision, Kind: "correct", Payload: api.WorkCommandPayload{Instruction: "Only answer from the independent evidence."}})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	done := awaitTask(t, m, root.ID, TaskCompleted)
	settleTask(t, r, root.ID)
	if done.Result != "Independent evidence only." || turns.Load() != 2 {
		t.Fatal("corrected work stayed trapped in partial synthesis", done.Result, turns.Load())
	}
}
