package agents

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkerRuntimeInspectContinueAndSynthesize(t *testing.T) {
	m, r, scope, root, child := workerFixture(t)
	defer r.Shutdown(context.Background())
	if _, err := r.Continue(context.Background(), child.ID, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	var turns atomic.Int32
	r.caller = func(_ context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID != "" {
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "Refined result"}), nil
		}
		switch turns.Add(1) {
		case 1:
			offered := false
			for _, tool := range tools {
				offered = offered || tool.Function.Name == "worker_inspect"
			}
			if !offered {
				t.Error("worker inspection absent from model tools")
			}
			return taskCall("worker_inspect", `{}`), nil
		case 2:
			found := false
			for _, message := range messages {
				found = found || message.Name == "worker_inspect" && strings.Contains(message.StringContent(), child.Worker.SessionID)
			}
			if !found {
				t.Error("worker identity was not returned to the primary assistant")
			}
			worker, err := m.WorkerSession(scope, child.Worker.SessionID)
			if err != nil {
				return nil, err
			}
			data, _ := json.Marshal(map[string]any{"worker_id": worker.ID, "expected_revision": worker.Revision, "content": "Check the unresolved part"})
			return taskCall("worker_continue", string(data)), nil
		default:
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "Refined result"}), nil
		}
	}
	r.advanceDelegation(context.Background(), root.ID)
	waiting := awaitTask(t, m, root.ID, TaskChildren)
	settleTask(t, r, root.ID)
	if len(waiting.ChildHistory) != 2 || waiting.Delegation == nil || waiting.Checkpoint.Calls[0].Function.Name != "worker_continue" {
		t.Fatal("follow-up did not yield through the existing coordinator", waiting)
	}
	r.mu.Lock()
	active := len(r.active)
	r.mu.Unlock()
	if active != 0 {
		t.Fatal("waiting parent held a runner slot")
	}
	continued := waiting.ChildHistory[1].ID
	if _, err := r.Continue(context.Background(), continued, scope.Actor, "start", "", false); err != nil {
		t.Fatal(err)
	}
	r.advanceDelegation(context.Background(), root.ID)
	done := awaitTask(t, m, root.ID, TaskCompleted)
	settleTask(t, r, root.ID)
	if done.Result != "Refined result" || workerSessionCount(done) != 1 {
		t.Fatal(done)
	}
}

func TestWorkerToolsAreProgressiveAndDoNotWidenQualifiedScope(t *testing.T) {
	task := &Task{WorkID: "work", Origin: &ConversationOrigin{Workspace: "workspace"}, Config: taskConfig()}
	for _, tool := range runtimeTools(task) {
		if workerRuntimeTool(tool.Function.Name) {
			t.Fatal("empty conversation loaded worker management schemas")
		}
	}
	task.ChildHistory = []ChildLink{{ID: "child", WorkerID: "worker"}}
	task.Config.ToolAdapter = &inference.AgentToolPin{Adapter: inference.NativeAgentToolAdapter, Scope: "unknown", CapabilitySHA256: "unqualified"}
	for _, tool := range runtimeTools(task) {
		if workerRuntimeTool(tool.Function.Name) {
			t.Fatal("worker tool bypassed model qualification")
		}
	}
}
