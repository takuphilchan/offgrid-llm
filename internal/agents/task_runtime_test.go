package agents

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func taskConfig() AgentConfig {
	c := DefaultAgentConfig()
	c.TaskFirst = true
	c.ContextWindow = 16384
	c.MaxIterations = 20
	return c
}
func taskCall(name, args string) *api.ChatCompletionResponse {
	return runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}})
}
func awaitTask(t *testing.T, m *Manager, id string, status TaskStatus) *Task {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		task, _ := m.GetTask(id)
		if task.Status == status {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	task, _ := m.GetTask(id)
	t.Fatalf("expected %s: %+v", status, task)
	return nil
}
func settleTask(t *testing.T, r *Runner, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		_, active := r.active[id]
		r.mu.Unlock()
		if !active {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("worker did not settle")
}

func TestPauseSteerAndResumeSameTask(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	started := make(chan struct{})
	var calls atomic.Int32
	r := NewRunner(m, &runTestTools{}, func(ctx context.Context, task *Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		if messages[len(messages)-1].StringContent() != "Use the revised outline" {
			t.Error("instruction absent")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Revised answer"}), nil
	})
	defer r.Shutdown(context.Background())
	task, _ := r.Create("Draft an outline", "model", "alice", taskConfig())
	if _, err := r.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	<-started
	if _, err := r.Pause(task.ID, "bob", false); !errors.Is(err, ErrTaskNotFound) {
		t.Fatal("foreign pause")
	}
	if _, err := r.Pause(task.ID, "alice", false); err != nil {
		t.Fatal(err)
	}
	settleTask(t, r, task.ID)
	saved, err := r.Steer(task.ID, "alice", "instruction-1", "Use the revised outline")
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.Steer(task.ID, "alice", "instruction-1", "Use the revised outline")
	if err != nil || len(again.Instructions) != 1 || len(again.Checkpoint.Messages) != len(saved.Checkpoint.Messages) {
		t.Fatal("duplicate instruction")
	}
	if _, err := r.Steer(task.ID, "alice", "instruction-1", "Changed"); !errors.Is(err, ErrRunConflict) {
		t.Fatal("conflicting instruction accepted")
	}
	result, err := r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || result.Status != TaskCompleted || calls.Load() != 2 {
		t.Fatalf("resume %+v %v", result, err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	got, _ := restored.GetTask(task.ID)
	if restored.StorageError() != nil || len(got.Instructions) != 1 {
		t.Fatal("instruction not durable")
	}
}

func TestPauseInFlightNeverReplays(t *testing.T) {
	tools := &runTestTools{started: make(chan struct{}), release: make(chan struct{})}
	tools.authorize = func(string, json.RawMessage, ToolExecution) error { return nil }
	tools.execute = func(_ string, _ json.RawMessage, _ ToolExecution) (string, error) {
		close(tools.started)
		<-tools.release
		return "changed", nil
	}
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("write_file", `{}`), nil
	})
	task, _ := r.Create("change", "model", "alice", taskConfig())
	r.Continue(context.Background(), task.ID, "alice", "start", "", true)
	<-tools.started
	paused, err := r.Pause(task.ID, "alice", true)
	if err != nil || paused.Status != TaskUncertain {
		t.Fatalf("effect forgotten: %+v %v", paused, err)
	}
	close(tools.release)
	settleTask(t, r, task.ID)
	if _, err := r.Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("uncertain action replay")
	}
	if tools.calls.Load() != 1 {
		t.Fatal("duplicate side effect")
	}
	r.Shutdown(context.Background())
}

func TestContextArchivesWholeExchangesAndSurvivesRestart(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	cfg := taskConfig()
	cfg.ContextWindow = 4096
	cfg.MaxTokens = 512
	task, _ := r.Create("Keep original instruction", "model", "alice", cfg)
	task, _ = m.updateTask(task.ID, func(x *Task) error {
		x.Status = TaskRunning
		for i := 0; i < 10; i++ {
			x.Checkpoint.Messages = append(x.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "history-call", Type: "function", Function: api.FunctionCall{Name: "read", Arguments: `{}`}}}}, api.ChatMessage{Role: "tool", Name: "read", ToolCallID: "history-call", Content: strings.Repeat("evidence α", 250)})
		}
		return nil
	})
	messages, err := r.prepareContext(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.ContextArchive) == 0 || messages[1].StringContent() != "Keep original instruction" {
		t.Fatal("missing archive or instruction")
	}
	for _, archive := range task.ContextArchive {
		if len(archive.Messages) != 2 || archive.Messages[1].ToolCallID != archive.Messages[0].ToolCalls[0].ID {
			t.Fatal("split call/result pair")
		}
	}
	page, err := contextPage(task, task.ContextArchive[0].ID, 0)
	if err != nil || !strings.Contains(page, "evidence") {
		t.Fatal("exact history unavailable")
	}
	if _, err := contextPage(task, "another-task-ref", 0); err == nil {
		t.Fatal("foreign archive")
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	got, _ := restored.GetTask(task.ID)
	if len(got.ContextArchive) != len(task.ContextArchive) {
		t.Fatal("lost archived context")
	}
	task.Config.ContextWindow = 128
	if _, err := r.prepareContext(task, nil); err == nil {
		t.Fatal("oversized input accepted")
	}
}

func TestDurableDelegationDependenciesAndNarrowScope(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	m.maxParallel = 1
	var first, second atomic.Int32
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, msg []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID != "" {
			for _, tool := range tools {
				if !isRuntimeTool(tool.Function.Name) {
					t.Errorf("child acquired unexpected tool: %s", tool.Function.Name)
				}
			}
			if task.Prompt == "first" {
				first.Add(1)
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "First finding"}), nil
			}
			second.Add(1)
			if first.Load() != 1 || !strings.Contains(msg[len(msg)-1].StringContent(), "First finding") {
				t.Error("dependency not complete or not attached")
			}
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "Second finding"}), nil
		}
		if len(task.ChildHistory) == 0 {
			return taskCall("delegate_tasks", `{"tasks":[{"key":"first","goal":"first","depends_on":[],"tools":[]},{"key":"second","goal":"second","depends_on":["first"],"tools":[]}]}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Combined findings"}), nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer r.Shutdown(context.Background())
	r.StartCoordinator(ctx)
	task, _ := r.Create("compare", "model", "alice", taskConfig())
	if _, err := r.Continue(ctx, task.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	result := awaitTask(t, m, task.ID, TaskCompleted)
	if len(result.ChildHistory) != 2 || first.Load() != 1 || second.Load() != 1 {
		t.Fatal("incomplete graph")
	}
	for _, link := range result.ChildHistory {
		child, _ := m.GetTask(link.ID)
		if child.Actor != "alice" || child.ParentID != task.ID || child.Config.ComputerSession != "" || child.Config.MaxIterations != 10 {
			t.Fatal("child scope widened")
		}
		if err := r.Delete(child.ID, "alice"); !errors.Is(err, ErrRunConflict) {
			t.Fatal("referenced child deleted")
		}
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	settleTask(t, r, task.ID)
	if err := r.Delete(task.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	for _, link := range result.ChildHistory {
		if err := r.Delete(link.ID, "alice"); err != nil {
			t.Fatal(err)
		}
	}
	restored = NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	if err := restored.StorageError(); err != nil {
		t.Fatalf("deleted graph cannot reopen: %v", err)
	}
}

func TestDelegationRestartRequiresExplicitResume(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	caller := func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID != "" || len(task.ChildHistory) > 0 {
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "done"}), nil
		}
		return taskCall("delegate_tasks", `{"tasks":[{"key":"one","goal":"one","depends_on":[],"tools":[]}]}`), nil
	}
	r := NewRunner(m, NewToolRegistry(), caller)
	task, _ := r.Create("work", "model", "alice", taskConfig())
	task, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskChildren {
		t.Fatalf("no graph: %+v %v", task, err)
	}
	r.Shutdown(context.Background())
	restored := NewManagerWithPersistence(nil, nil, nil, dir)
	if err = restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	next := NewRunner(restored, NewToolRegistry(), caller)
	defer next.Shutdown(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	next.StartCoordinator(ctx)
	parent, _ := restored.GetTask(task.ID)
	if parent.Status != TaskInterrupted {
		t.Fatal("parent silently resumed")
	}
	child, _ := restored.GetTask(parent.ChildHistory[0].ID)
	if child.Status != TaskInterrupted {
		t.Fatal("child silently resumed")
	}
	if _, err = next.Continue(ctx, child.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatal("child bypassed parent pause")
	}
	if _, err = next.Continue(ctx, parent.ID, "alice", "resume", "", true); err != nil {
		t.Fatal(err)
	}
	awaitTask(t, restored, parent.ID, TaskCompleted)
}

func TestDelegationRejectsCyclesAndAuthorityExpansion(t *testing.T) {
	for _, specs := range [][]ChildSpec{
		{{Key: "a", Goal: "one", DependsOn: []string{"a"}}},
		{{Key: "a", Goal: "one", DependsOn: []string{"b"}}},
		{{Key: "a", Goal: "one", Tools: []string{"write_file"}}},
		{{Key: "a", Goal: "one", Tools: []string{"request_computer_access"}}},
	} {
		if err := validateChildSpecs(specs); err == nil {
			t.Fatalf("unsafe graph accepted: %+v", specs)
		}
	}
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, NewToolRegistry(), nil)
	task, _ := r.Create("test", "model", "alice", taskConfig())
	task.ParentID = "parent"
	if err := r.beginDelegation(task, api.ToolCall{}); err == nil {
		t.Fatal("child spawned another agent")
	}
	task.ParentID = ""
	task.Config.ComputerSession = "local-session"
	if err := r.beginDelegation(task, api.ToolCall{}); err == nil {
		t.Fatal("computer authority inherited by child")
	}
	if err := validateTaskGraph([]*Task{{ID: "parent", Actor: "alice", ChildHistory: []ChildLink{{ID: "child"}}}, {ID: "child", Actor: "bob", ParentID: "parent"}}); err == nil {
		t.Fatal("cross-user graph restored")
	}
}
