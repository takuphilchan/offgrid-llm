package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type runTestTools struct {
	calls   atomic.Int32
	fail    bool
	before  func()
	started chan struct{}
	release chan struct{}
}

func TestShutdownPersistsInterruptedRunAndStopsAdmission(t *testing.T) {
	manager := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	started := make(chan struct{})
	caller := func(ctx context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	runner := NewRunner(manager, &runTestTools{}, caller)
	config := AgentConfig{MaxIterations: 2, TimeoutPerStep: time.Minute}
	task, err := runner.Create("test", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
		t.Fatal(err)
	}
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := runner.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	got, ok := manager.GetTask(task.ID)
	if !ok || got.Status != TaskInterrupted {
		t.Fatalf("checkpoint not persisted: %+v", got)
	}
	if _, err := runner.Continue(context.Background(), task.ID, "alice", "resume", "", true); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("admitted work during shutdown: %v", err)
	}
}

func (t *runTestTools) GetTools() []api.Tool { return nil }
func (t *runTestTools) Capability(name string) (capabilities.Descriptor, bool) {
	return capabilities.Descriptor{Name: name, Source: "test", Kind: capabilities.Write, Risk: capabilities.RiskHigh}, true
}
func (t *runTestTools) Authorize(_ context.Context, _ string, _ json.RawMessage, grant ToolExecution) error {
	if !grant.Approved {
		return capabilities.ErrApprovalRequired
	}
	if t.before != nil {
		t.before()
	}
	return nil
}
func (t *runTestTools) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, grant ToolExecution) (string, error) {
	if !grant.Approved {
		return "", capabilities.ErrApprovalRequired
	}
	t.calls.Add(1)
	if t.started != nil {
		close(t.started)
		select {
		case <-t.release:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if t.fail {
		return "", errors.New("tool may have partially changed the target")
	}
	return "saved", nil
}
func runAnswer(message api.ChatMessage) *api.ChatCompletionResponse {
	reason := "stop"
	if len(message.ToolCalls) > 0 {
		reason = "tool_calls"
	}
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: reason}}}
}

func testRunCaller(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
	if len(messages) == 2 {
		call := api.ToolCall{ID: "upstream-reused", Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: `{"path":"notes","content":"hello"}`}}
		return runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call, call}}), nil
	}
	return runAnswer(api.ChatMessage{Role: "assistant", Content: "Finished"}), nil
}
func beginTestRun(t *testing.T, dir string, tools *runTestTools) (*Manager, *Runner, *Task) {
	t.Helper()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, tools, testRunCaller)
	task, err := r.Create("write notes", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskWaiting || task.PendingApproval == nil {
		t.Fatalf("not waiting: %+v", task)
	}
	return m, r, task
}

func TestDurableApprovalsResumeSameRunWithoutReplayingTools(t *testing.T) {
	dir := t.TempDir()
	tools := &runTestTools{}
	_, _, task := beginTestRun(t, dir, tools)
	id, approval := task.ID, task.PendingApproval.ID
	firstCallID := task.PendingApproval.CallID
	// A waiting run must be resumable from disk, without calling the model again.
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, tools, testRunCaller)
	if _, err := r.Continue(context.Background(), id, "bob", "approve", approval, false); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("actor isolation: %v", err)
	}
	if _, err := r.Continue(context.Background(), id, "alice", "approve", "forged", false); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("forged grant: %v", err)
	}
	task, err := r.Continue(context.Background(), id, "alice", "approve", approval, false)
	if err != nil {
		t.Fatal(err)
	}
	if task.ID != id || task.Status != TaskWaiting || tools.calls.Load() != 1 {
		t.Fatalf("first approval: %+v calls=%d", task, tools.calls.Load())
	}
	if task.PendingApproval.ID == approval || task.PendingApproval.CallID == firstCallID {
		t.Fatal("grant identity reused")
	}
	if _, err := r.Continue(context.Background(), id, "alice", "approve", approval, false); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("replayed grant: %v", err)
	}
	// Restart again after one successful tool. It must not run again.
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	r = NewRunner(m, tools, testRunCaller)
	task, err = r.Continue(context.Background(), id, "alice", "approve", task.PendingApproval.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskCompleted || task.Result != "Finished" || tools.calls.Load() != 2 || len(task.Checkpoint.Messages) != 6 {
		t.Fatalf("completion: %+v calls=%d", task, tools.calls.Load())
	}
}

func TestExpiredAndDeniedApprovalsCannotExecute(t *testing.T) {
	m, r, task := beginTestRun(t, t.TempDir(), &runTestTools{})
	old := task.PendingApproval.ID
	_, err := m.updateTask(task.ID, func(task *Task) error { task.PendingApproval.ExpiresAt = time.Now().Add(-time.Minute); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Continue(context.Background(), task.ID, "alice", "approve", old, false); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("expired approval: %v", err)
	}
	task, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if task.PendingApproval.ID == old {
		t.Fatal("expired approval was reused")
	}
	task, err = r.Stop(task.ID, "alice", "deny", task.PendingApproval.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskCancelled || task.PendingApproval != nil {
		t.Fatalf("denial not recorded: %+v", task)
	}
	if _, err := r.Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("resumed denied run: %v", err)
	}
}

func TestConcurrentApprovalExecutesOnce(t *testing.T) {
	tools := &runTestTools{started: make(chan struct{}), release: make(chan struct{})}
	_, r, task := beginTestRun(t, t.TempDir(), tools)
	firstDone := make(chan error, 1)
	go func() {
		_, err := r.Continue(context.Background(), task.ID, "alice", "approve", task.PendingApproval.ID, false)
		firstDone <- err
	}()
	select {
	case <-tools.started:
	case <-time.After(5 * time.Second):
		t.Fatal("tool not started")
	}
	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := r.Continue(context.Background(), task.ID, "alice", "approve", task.PendingApproval.ID, false); !errors.Is(err, ErrRunConflict) {
				t.Errorf("duplicate approval: %v", err)
			}
		}()
	}
	group.Wait()
	close(tools.release)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if tools.calls.Load() != 1 {
		t.Fatalf("executed %d times", tools.calls.Load())
	}
}

func TestUncertainOutcomeRequiresReconciliation(t *testing.T) {
	dir := t.TempDir()
	tools := &runTestTools{fail: true}
	_, r, task := beginTestRun(t, dir, tools)
	task, err := r.Continue(context.Background(), task.ID, "alice", "approve", task.PendingApproval.ID, false)
	if err == nil || task.Status != TaskUncertain {
		t.Fatalf("uncertainty lost: %+v %v", task, err)
	}
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r = NewRunner(m, tools, testRunCaller)
	if _, err := r.Continue(context.Background(), task.ID, "alice", "resume", "", false); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("uncertain run replayed: %v", err)
	}
	task, err = r.Reconcile(task.ID, "alice", task.Checkpoint.ExecutingCall, "Verified notes were saved")
	if err != nil || task.Status != TaskInterrupted {
		t.Fatalf("reconcile: %+v %v", task, err)
	}
	task, err = r.Continue(context.Background(), task.ID, "alice", "resume", "", false)
	if err != nil || task.Status != TaskWaiting || tools.calls.Load() != 1 {
		t.Fatalf("replayed resolved tool: %+v %v", task, err)
	}
}

func TestPersistenceFailurePreventsSideEffects(t *testing.T) {
	dir := t.TempDir()
	tools := &runTestTools{}
	m, r, task := beginTestRun(t, dir, tools)
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	tools.before = func() { m.mu.Lock(); m.dataDir = blocked; m.mu.Unlock() }
	if _, err := r.Continue(context.Background(), task.ID, "alice", "approve", task.PendingApproval.ID, false); !errors.Is(err, ErrRunStorage) {
		t.Fatalf("persistence failure hidden: %v", err)
	}
	if tools.calls.Load() != 0 || m.StorageError() == nil {
		t.Fatal("tool ran without a durable checkpoint")
	}
}

func TestStartupReconcilesInFlightRunsAndPreservesSnapshots(t *testing.T) {
	dir := t.TempDir()
	m, _, task := beginTestRun(t, dir, &runTestTools{})
	_, err := m.updateTask(task.ID, func(task *Task) error {
		task.Status = TaskRunning
		task.Checkpoint.ExecutingCall = task.Checkpoint.Calls[0].ID
		task.PendingApproval = nil
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	restored, ok := m.GetTask(task.ID)
	if !ok || restored.Status != TaskUncertain {
		t.Fatalf("startup status: %+v", restored)
	}
	restored.Prompt = "changed"
	restored.Checkpoint.Messages[0].Content = "changed"
	copy, _ := m.GetTask(task.ID)
	if copy.Prompt == "changed" || copy.Checkpoint.Messages[0].Content == "changed" {
		t.Fatal("returned mutable stored state")
	}
}

func TestCanonicalArgumentsPreserveLargeIntegers(t *testing.T) {
	args, err := CanonicalArguments(json.RawMessage(`{"id":9007199254740993,"a":true}`))
	if err != nil || string(args) != `{"a":true,"id":9007199254740993}` {
		t.Fatalf("canonical: %s %v", args, err)
	}
	if _, err := CanonicalArguments(json.RawMessage(`{} {}`)); err == nil {
		t.Fatal("accepted trailing JSON")
	}
}

func TestAsyncCancellationKeepsUncertainCheckpoint(t *testing.T) {
	tools := &runTestTools{started: make(chan struct{}), release: make(chan struct{})}
	m, runner, pending := beginTestRun(t, t.TempDir(), tools)
	initial, err := runner.Continue(context.Background(), pending.ID, "alice", "approve", pending.PendingApproval.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-tools.started:
	case <-time.After(5 * time.Second):
		t.Fatal("tool not started")
	}
	// Concurrent polling/HTTP marshaling must see detached snapshots, never the
	// mutable checkpoint the worker is advancing.
	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for range 40 {
				snapshot, ok := m.GetTask(pending.ID)
				if !ok {
					t.Error("task vanished")
					return
				}
				if _, err := json.Marshal(snapshot); err != nil {
					t.Error(err)
				}
				if _, err := json.Marshal(initial); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	stopped, err := runner.Stop(pending.ID, "alice", "cancel", "")
	if err != nil || stopped.Status != TaskUncertain {
		t.Fatalf("cancel: %+v %v", stopped, err)
	}
	readers.Wait()
	deadline := time.Now().Add(5 * time.Second)
	for {
		runner.mu.Lock()
		active := len(runner.active)
		runner.mu.Unlock()
		if active == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cancelled worker did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	saved, _ := m.GetTask(pending.ID)
	if saved.Status != TaskUncertain || saved.Checkpoint.ExecutingCall == "" || tools.calls.Load() != 1 {
		t.Fatalf("lost uncertain state: %+v", saved)
	}
	if initial.Status != TaskRunning || initial.Checkpoint.ExecutingCall != "" {
		t.Fatal("async return aliased worker state")
	}
}

func TestInterruptedModelCallResumesWithoutToolReplay(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	var attempts int
	runner := NewRunner(m, &runTestTools{}, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		attempts++
		if attempts == 1 {
			return nil, context.DeadlineExceeded
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "Recovered"}), nil
	})
	run, err := runner.Create("test", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	run, err = runner.Continue(context.Background(), run.ID, "alice", "start", "", false)
	if !errors.Is(err, context.DeadlineExceeded) || run.Status != TaskInterrupted {
		t.Fatalf("interruption: %+v %v", run, err)
	}
	run, err = runner.Continue(context.Background(), run.ID, "alice", "resume", "", false)
	if err != nil || run.Status != TaskCompleted || run.Result != "Recovered" {
		t.Fatalf("resume: %+v %v", run, err)
	}
}
