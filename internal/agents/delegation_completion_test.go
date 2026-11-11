package agents

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Exercise the complete failure sequence: children finish, the parent repeats
// the graph, then proposes an incomplete answer. Neither correction may rerun
// the children or lose their dependency results.
func TestCompletedGraphReuseThenAnswerRepairSurvivesReload(t *testing.T) {
	store := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, store)
	const graph = `{"tasks":[{"key":"a","goal":"Check Source A","tools":[]},{"key":"b","goal":"Check Source B","tools":[]},{"key":"writer","goal":"Write the briefing","depends_on":["a","b"],"tools":[]}]}`
	const prompt = "Check 96 of 120 tasks completed, and 72 of those 96 passed review. Check 94 of 120 tasks completed, and 72 of those 94 passed review. Show each completion and pass rate. Use only these supplied numbers."
	const final = "Source A: completion 80%, review pass rate 75%. Source B: completion 78.33%, review pass rate 76.60%. The completed count (96 versus 94) needs confirmation. The supplied numbers do not explain the cause of the discrepancy."
	var parentCalls, childCalls atomic.Int32
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID != "" {
			childCalls.Add(1)
			var contextText strings.Builder
			for _, message := range messages {
				contextText.WriteString(message.StringContent())
			}
			if !strings.Contains(contextText.String(), prompt) {
				t.Error("child lost original user facts or restrictions")
			}
			if task.Prompt == "Write the briefing" && (!strings.Contains(contextText.String(), "Results from completed dependency tasks") || !strings.Contains(contextText.String(), "Source A checked") || !strings.Contains(contextText.String(), "Source B checked")) {
				t.Error("writer started without both saved analyst results")
			}
			answer := final
			if task.Prompt == "Check Source A" {
				answer = "Source A checked: 80% completion, 75% review pass rate."
			} else if task.Prompt == "Check Source B" {
				answer = "Source B checked: 78.33% completion, 76.60% review pass rate."
			}
			return runAnswer(api.ChatMessage{Role: "assistant", Content: answer}), nil
		}
		switch parentCalls.Add(1) {
		case 1, 2:
			return taskCall("delegate_tasks", graph), nil
		case 3:
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "The completed counts disagree."}), nil
		default:
			if len(tools) != 0 {
				t.Error("answer correction exposed executable tools")
			}
			return runAnswer(api.ChatMessage{Role: "assistant", Content: final}), nil
		}
	})
	defer r.Shutdown(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartCoordinator(ctx)
	cfg := taskConfig()
	cfg.MaxIterations = 32
	root, err := r.Create(prompt, "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Continue(ctx, root.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, m, root.ID, TaskCompleted)
	if err = r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	reloaded := NewManagerWithPersistence(nil, nil, nil, store)
	result, _ := reloaded.GetTask(root.ID)
	if reloaded.StorageError() != nil || result == nil {
		t.Fatal("could not reload saved completion", reloaded.StorageError())
	}
	if result.Status != TaskCompleted || result.Result != final || len(result.ChildHistory) != 3 || len(reloaded.ListTasks()) != 4 || childCalls.Load() != 3 || parentCalls.Load() != 4 || result.Checkpoint.DelegationAnswerRepairs != 1 {
		t.Fatalf("completion or no-replay invariant failed: status=%s result=%q children=%d child_calls=%d parent_calls=%d repairs=%d", result.Status, result.Result, len(result.ChildHistory), childCalls.Load(), parentCalls.Load(), result.Checkpoint.DelegationAnswerRepairs)
	}
}
