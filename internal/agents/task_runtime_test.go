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
	aliasPage, _ := contextPage(task, "archive:1", 0)
	if restoredPage, err := contextPage(got, "archive:1", 0); err != nil || restoredPage != aliasPage {
		t.Fatal("task-local archive reference changed after restart", err)
	}
	task.Config.ContextWindow = 128
	if _, err := r.prepareContext(task, nil); err == nil {
		t.Fatal("oversized input accepted")
	}
}

func TestContextPreservesLatestExchangeAndSteering(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, &runTestTools{}, nil)
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.ContextWindow, cfg.MaxTokens = 4096, 512
	task, err := r.Create("Original task", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(x *Task) error { x.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	exchange := func(id, text string) []api.ChatMessage {
		return []api.ChatMessage{{Role: "assistant", ToolCalls: []api.ToolCall{{ID: id, Type: "function", Function: api.FunctionCall{Name: "read", Arguments: `{}`}}}}, {Role: "tool", ToolCallID: id, Content: text}}
	}
	old := exchange("previous", strings.Repeat("x", 9000))
	latest := exchange("current", strings.Repeat("y", 3000))
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, old...)
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "user", Content: "Do not submit any forms"})
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, latest...)
	messages, err := r.prepareContext(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(task.ContextArchive) != 1 || task.ContextArchive[0].Messages[1].StringContent() != old[1].StringContent() {
		t.Fatal("completed evidence not archived exactly")
	}
	if len(messages) != 6 || messages[2].StringContent() != "Do not submit any forms" || messages[5].StringContent() != latest[1].StringContent() || messages[5].Role != "tool" || messages[4].Role != "assistant" {
		t.Fatal("current observation or instructions lost")
	}
	if messages[0].StringContent() != task.Checkpoint.Messages[0].StringContent() || !strings.Contains(messages[3].StringContent(), "archive:1") {
		t.Fatal("archive index invalidated stable prompt prefix or disappeared")
	}
	// Incomplete effects cannot be discarded to make an oversized prompt fit.
	task.Checkpoint.Messages = append(task.Checkpoint.Messages[:2], old[0])
	task.Checkpoint.Messages[2].ToolCalls[0].Function.Arguments = `{"text":"` + strings.Repeat("x", 15000) + `"}`
	task.Checkpoint.Messages = append(task.Checkpoint.Messages, latest...)
	_, err = r.prepareContext(task, nil)
	if err == nil || strings.Contains(strings.ToLower(err.Error()), "no tool was dispatched") {
		t.Fatal("unsafe or misleading context failure", err)
	}
}

func TestIncompleteContextGroupsAreNotArchived(t *testing.T) {
	call := api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "one"}, {ID: "two"}}}
	for _, group := range [][]api.ChatMessage{
		{call},
		{call, {Role: "tool", ToolCallID: "one"}},
		{call, {Role: "tool", ToolCallID: "one"}, {Role: "tool", ToolCallID: "one"}},
		{call, {Role: "user", Content: "correction"}},
	} {
		if completedContextGroup(group) {
			t.Fatal("incomplete group considered complete")
		}
	}
	if !completedContextGroup([]api.ChatMessage{call, {Role: "tool", ToolCallID: "one"}, {Role: "tool", ToolCallID: "two"}}) {
		t.Fatal("complete group rejected")
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
	if len(result.ChildHistory) != 2 || first.Load() != 1 || second.Load() != 1 || result.Checkpoint.Iteration+result.Checkpoint.ReservedChildIterations > result.Config.MaxIterations {
		t.Fatal("incomplete graph")
	}
	for _, link := range result.ChildHistory {
		child, _ := m.GetTask(link.ID)
		if child.Actor != "alice" || child.ParentID != task.ID || child.Config.ComputerSession != "" || child.Config.MaxIterations != 8 {
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

func TestDelegationKeepsIndependentResultAfterPrerequisiteFailure(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID != "" {
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "Independent evidence"}), nil
		}
		return taskCall("delegate_tasks", `{"tasks":[{"key":"source","goal":"source","tools":[]},{"key":"dependent","goal":"dependent","depends_on":["source"],"tools":[]},{"key":"independent","goal":"independent","tools":[]}]}`), nil
	})
	defer r.Shutdown(context.Background())
	parent, _ := r.Create("research", "model", "alice", taskConfig())
	parent, err := r.Continue(context.Background(), parent.ID, "alice", "start", "", false)
	if err != nil || parent.Status != TaskChildren {
		t.Fatalf("delegation was not persisted: %v", err)
	}
	var sourceID, dependentID, independentID string
	for _, link := range parent.ChildHistory {
		switch link.Spec.Key {
		case "source":
			sourceID = link.ID
		case "dependent":
			dependentID = link.ID
		case "independent":
			independentID = link.ID
		}
	}
	if _, err := m.updateTask(sourceID, func(task *Task) error {
		finishTask(task, TaskFailed, "source unavailable")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartCoordinator(ctx)
	result := awaitTask(t, m, parent.ID, TaskFailed)
	if result.ErrorCode != "delegation_partial" || !strings.Contains(result.Steps[len(result.Steps)-1].ToolResult, "Independent evidence") {
		t.Fatalf("partial result was lost: %+v", result)
	}
	var outcomes []childOutcome
	if err := json.Unmarshal([]byte(result.Steps[len(result.Steps)-1].ToolResult), &outcomes); err != nil {
		t.Fatalf("partial result is not structured: %v", err)
	}
	if len(outcomes) != 3 {
		t.Fatalf("expected every branch outcome, got %+v", outcomes)
	}
	for _, outcome := range outcomes {
		if outcome.IndependentlyVerified {
			t.Fatalf("child result falsely claims independent verification: %+v", outcome)
		}
	}
	for _, link := range result.ChildHistory {
		if link.Spec.Key == "dependent" && link.Status != TaskFailed {
			t.Fatalf("blocked descendant status was not persisted on parent: %+v", link)
		}
		if link.Spec.Key == "independent" && link.Status != TaskCompleted {
			t.Fatalf("independent result status was not persisted on parent: %+v", link)
		}
	}
	dependent, _ := m.GetTask(dependentID)
	independent, _ := m.GetTask(independentID)
	if dependent.ErrorCode != "dependency_failed" || dependent.StartedAt != nil || independent.Status != TaskCompleted {
		t.Fatalf("incorrect branch outcomes: dependent=%+v independent=%+v", dependent, independent)
	}
}

func TestDelegationAdmitsIndependentReadyChildrenWithinRunnerCapacity(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	m.maxParallel = 2
	started := make(chan string, 2)
	release := make(chan struct{})
	r := NewRunner(m, NewToolRegistry(), func(ctx context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if task.ParentID != "" {
			select {
			case started <- task.Prompt:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return runAnswer(api.ChatMessage{Role: "assistant", Content: task.Prompt + " done"}), nil
		}
		if len(task.ChildHistory) == 0 {
			return taskCall("delegate_tasks", `{"tasks":[{"key":"a","goal":"a","tools":[]},{"key":"b","goal":"b","tools":[]}]}`), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "both done"}), nil
	})
	defer r.Shutdown(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	parent, _ := r.Create("two independent tasks", "model", "alice", taskConfig())
	if _, err := r.Continue(ctx, parent.ID, "alice", "start", "", false); err != nil {
		t.Fatal(err)
	}
	r.StartCoordinator(ctx)
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			close(release)
			t.Fatal("independent ready child was not admitted within runner capacity")
		}
	}
	close(release)
	result := awaitTask(t, m, parent.ID, TaskCompleted)
	if len(result.ChildHistory) != 2 {
		t.Fatal("duplicate or missing children")
	}
}

func TestDelegationPersistsOriginalFactsForEveryChild(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		return taskCall("delegate_tasks", `{"tasks":[{"key":"a","goal":"Source A analysis","tools":[]},{"key":"b","goal":"Source B analysis","tools":[]},{"key":"writer","goal":"Brief writer synthesis","depends_on":["a","b"],"tools":[]}]}`), nil
	})
	defer r.Shutdown(context.Background())
	prompt := "Source A: 96 of 120 completed, 72 of 96 passed. Source B: 94 of 120 completed, 72 of 94 passed. Use only these numbers; do not change files."
	task, err := r.Create(prompt, "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	parent, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || parent.Status != TaskChildren || len(parent.ChildHistory) != 3 {
		t.Fatalf("delegation was not saved: %+v %v", parent, err)
	}
	reopened := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := reopened.StorageError(); err != nil {
		t.Fatal(err)
	}
	for _, link := range parent.ChildHistory {
		child, ok := reopened.GetTask(link.ID)
		if !ok || child.Checkpoint == nil {
			t.Fatalf("missing child checkpoint: %s", link.ID)
		}
		var hasFacts, hasScope bool
		for _, message := range child.Checkpoint.Messages {
			content := message.StringContent()
			hasFacts = hasFacts || strings.Contains(content, prompt)
			hasScope = hasScope || strings.Contains(content, "Assigned subtask:\n"+link.Spec.Goal)
		}
		if !hasFacts || !hasScope || len(child.Config.AllowedTools) != 0 || child.Config.ComputerSession != "" {
			t.Fatalf("child lost source facts or widened authority: %+v", child)
		}
	}
}

func TestDelegationCarriesSuppliedFiguresThroughThreeSpecialistGraph(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	m.maxParallel = 2
	prompt := "Have Source A analyst check 96 of 120 completed and 72 of 96 passed. Have Source B analyst check 94 of 120 completed and 72 of 94 passed. Have Brief writer compare both. Use only these figures."
	var aCalls, bCalls, writerCalls atomic.Int32
	r := NewRunner(m, NewToolRegistry(), func(_ context.Context, task *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		joined := ""
		for _, message := range messages {
			joined += message.StringContent() + "\n"
		}
		if task.ParentID != "" {
			if !strings.Contains(joined, prompt) {
				t.Errorf("%s did not receive the source figures", task.Prompt)
			}
			switch task.Prompt {
			case "Source A analysis":
				aCalls.Add(1)
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "A: 96/120 = 80%; 72/96 = 75%"}), nil
			case "Source B analysis":
				bCalls.Add(1)
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "B: 94/120 = 78.33%; 72/94 = 76.60%"}), nil
			case "Brief writer synthesis":
				writerCalls.Add(1)
				if !strings.Contains(joined, "96/120 = 80%") || !strings.Contains(joined, "94/120 = 78.33%") {
					t.Error("writer started without both completed analyses")
				}
				return runAnswer(api.ChatMessage{Role: "assistant", Content: "A: 80%, 75%. B: 78.33%, 76.60%. Confirm the completed-task count."}), nil
			}
			t.Errorf("unexpected child: %s", task.Prompt)
			return runAnswer(api.ChatMessage{Role: "assistant", Content: "unexpected child"}), nil
		}
		if len(task.ChildHistory) == 0 {
			return taskCall("delegate_tasks", `{"tasks":[{"key":"a","goal":"Source A analysis","tools":[]},{"key":"b","goal":"Source B analysis","tools":[]},{"key":"writer","goal":"Brief writer synthesis","depends_on":["a","b"],"tools":[]}]}`), nil
		}
		if !strings.Contains(joined, "Confirm the completed-task count") {
			t.Error("parent did not receive the completed writer result")
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "A: 80%, 75%. B: 78.33%, 76.60%. Confirm the completed-task count."}), nil
	})
	defer r.Shutdown(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartCoordinator(ctx)
	task, err := r.Create(prompt, "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Continue(ctx, task.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	result := awaitTask(t, m, task.ID, TaskCompleted)
	if aCalls.Load() != 1 || bCalls.Load() != 1 || writerCalls.Load() != 1 || len(result.ChildHistory) != 3 {
		t.Fatalf("graph did not complete once per specialist: a=%d b=%d writer=%d children=%d", aCalls.Load(), bCalls.Load(), writerCalls.Load(), len(result.ChildHistory))
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
	// Simulate an older persisted parent checkpoint whose child reservation was
	// absent even though the child job was durably created.
	if _, err = m.updateTask(task.ID, func(t *Task) error {
		t.Checkpoint.ReservedChildIterations = 0
		return nil
	}); err != nil {
		t.Fatal(err)
	}
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
	if parent.Checkpoint.ReservedChildIterations != child.Config.MaxIterations {
		t.Fatalf("restored parent regained allocated child decisions: reserved=%d child=%d", parent.Checkpoint.ReservedChildIterations, child.Config.MaxIterations)
	}
	if remaining, err := childDecisionBudget(parent, 1); err == nil || remaining != 0 {
		t.Fatalf("restored parent could oversubscribe child budget: remaining=%d error=%v", remaining, err)
	}
	reopened := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := reopened.StorageError(); err != nil {
		t.Fatal(err)
	}
	persisted, _ := reopened.GetTask(parent.ID)
	if persisted.Checkpoint.ReservedChildIterations != child.Config.MaxIterations {
		t.Fatal("repaired reservation was not persisted")
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
