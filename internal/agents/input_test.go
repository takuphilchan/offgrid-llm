package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func waitingInputFixture(t *testing.T) (*Manager, *Runner, *Task, *runTestTools, string) {
	t.Helper()
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	tools := &runTestTools{authorize: func(name string, _ json.RawMessage, _ ToolExecution) error {
		if name == "request_computer_access" {
			return &InputRequired{Request: InputRequest{Kind: "computer", Mode: "app", Target: "Notepad"}}
		}
		return nil
	}}
	r := NewRunner(m, tools, func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(messages) == 2 {
			return runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "request_computer_access", Arguments: `{"mode":"app","target":"Notepad","url":""}`}}}}), nil
		}
		return runAnswer(api.ChatMessage{Role: "assistant", Content: "No changes requested."}), nil
	})
	config := DefaultAgentConfig()
	config.TaskFirst = true
	task, err := r.Create("Read Notepad", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskInput || task.PendingInput == nil || task.Checkpoint.ExecutingCall != "" || tools.calls.Load() != 0 {
		t.Fatalf("access request dispatched or lost: %+v", task)
	}
	return m, r, task, tools, dir
}

func TestInputSurvivesRestartAndResolvesOnce(t *testing.T) {
	_, _, task, tools, dir := waitingInputFixture(t)
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := m.StorageError(); err != nil {
		t.Fatal(err)
	}
	saved, ok := m.GetTask(task.ID)
	if !ok || saved.Status != TaskInput || saved.PendingInput.ID != task.PendingInput.ID {
		t.Fatal("saved access interruption lost")
	}
	r := NewRunner(m, tools, nil)
	config := saved.Config
	config.ComputerSession = "session"
	config.ComputerDriver = "windows-uia"
	config.ComputerApprovalMode = "scoped_changes"
	if _, err := r.ResolveComputerInput(task.ID, "bob", task.PendingInput.ID, config); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("cross-owner resolution: %v", err)
	}
	if _, err := r.ResolveComputerInput(task.ID, "alice", "wrong", config); !errors.Is(err, ErrRunConflict) {
		t.Fatalf("wrong input: %v", err)
	}
	resolved, err := r.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != TaskPending || resolved.PendingInput != nil || len(resolved.Checkpoint.Calls) != 0 || len(resolved.Steps) != 1 || resolved.Config.ComputerStepOffset != 1 {
		t.Fatalf("invalid resolution: %+v", resolved)
	}
	again, err := r.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, config)
	if err != nil || len(again.Steps) != 1 || len(again.Checkpoint.Messages) != 4 {
		t.Fatalf("duplicated input: %+v %v", again, err)
	}
	if tools.calls.Load() != 0 {
		t.Fatal("consent resolution executed a tool")
	}
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	expired, _ := m.GetTask(task.ID)
	if !expired.ComputerSessionExpired || expired.Status != TaskInterrupted {
		t.Fatal("restart retained active authority")
	}
}

func TestResolvedInputDoesNotCountAsAComputerObservation(t *testing.T) {
	manager, _, task, tools, _ := waitingInputFixture(t)
	runner := NewRunner(manager, tools, func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		return runAnswer(api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "computer_replace_text", Arguments: `{"text":"must not dispatch"}`}}}}), nil
	})
	config := task.Config
	config.ComputerSession, config.ComputerDriver, config.ComputerApprovalMode = "session", "windows-uia", "full_task"
	if _, err := runner.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, config); err != nil {
		t.Fatal(err)
	}
	result, err := runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != TaskFailed || tools.calls.Load() != 0 {
		t.Fatalf("access was mistaken for observation: %+v", result)
	}
}

func TestInputCancelAndUncertainCannotResolve(t *testing.T) {
	m, r, task, _, _ := waitingInputFixture(t)
	config := task.Config
	config.ComputerSession = "session"
	config.ComputerDriver = "browser"
	config.ComputerApprovalMode = "full_task"
	_, err := m.updateTask(task.ID, func(next *Task) error {
		next.Status = TaskUncertain
		next.Checkpoint.ExecutingCall = next.PendingInput.CallID
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, config); !errors.Is(err, ErrRunConflict) {
		t.Fatal("executing call was reconciled as access")
	}
	_, err = m.updateTask(task.ID, func(next *Task) error { next.Status = TaskInput; next.Checkpoint.ExecutingCall = ""; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Stop(task.ID, "alice", "cancel", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ResolveComputerInput(task.ID, "alice", task.PendingInput.ID, config); !errors.Is(err, ErrRunConflict) {
		t.Fatal("cancelled input resumed")
	}
}

func TestRequestDeduplicationConcurrentAndRestart(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, nil, nil)
	config := DefaultAgentConfig()
	ids := make(chan string, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, _, err := r.CreateRequest("task", "model", "alice", config, "client-request", "digest")
			if err != nil {
				t.Error(err)
				return
			}
			ids <- task.ID
		}()
	}
	wg.Wait()
	close(ids)
	id := ""
	for got := range ids {
		if id != "" && got != id {
			t.Fatal("duplicate submission created second task")
		}
		id = got
	}
	m = NewManagerWithPersistence(nil, nil, nil, dir)
	r = NewRunner(m, nil, nil)
	again, created, err := r.CreateRequest("task", "model", "alice", config, "client-request", "digest")
	if err != nil || created || again.ID != id {
		t.Fatalf("restart dedupe: %+v %v", again, err)
	}
	if _, _, err = r.CreateRequest("changed", "model", "alice", config, "client-request", "different"); !errors.Is(err, ErrRunConflict) {
		t.Fatal("changed payload reused ID")
	}
	other, created, err := r.CreateRequest("task", "model", "bob", config, "client-request", "digest")
	if err != nil || !created || other.ID == id {
		t.Fatal("request IDs not actor scoped")
	}
	if _, err = r.Stop(id, "alice", "cancel", ""); err != nil {
		t.Fatal(err)
	}
	if err = r.Delete(id, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = r.CreateRequest("task", "model", "alice", config, "client-request", "digest"); !errors.Is(err, ErrRunConflict) {
		t.Fatal("deleted submission resurrected")
	}
}

func TestInputMigrationKeepsVerifiedVersionTwoBackup(t *testing.T) {
	_, _, task, _, dir := waitingInputFixture(t)
	db, err := openTaskDatabase(dir)
	if err != nil {
		t.Fatal(err)
	}
	// A schema-2 fixture has the same tables but no waiting_for_input snapshots.
	snapshot, _ := json.Marshal(&Task{ID: task.ID, Actor: "alice", Status: TaskCompleted, Prompt: "original"})
	if _, err = db.Exec(`UPDATE agent_tasks SET snapshot=? WHERE id=?`, snapshot, task.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DELETE FROM agent_task_events`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE agent_schema SET version=2`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DELETE FROM agent_migrations WHERE version=?`, taskSchemaVersion); err != nil {
		t.Fatal(err)
	}
	if err = initializeTaskDatabase(db, dir); err != nil {
		t.Fatal(err)
	}
	var manifest []byte
	var version int
	if err = db.QueryRow(`SELECT manifest FROM agent_migrations WHERE version=?`, taskSchemaVersion).Scan(&manifest); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "sha256") {
		t.Fatal("missing recovery manifest")
	}
	var record struct{ Backup string }
	if err = json.Unmarshal(manifest, &record); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, record.Backup)); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT version FROM agent_schema`).Scan(&version); err != nil || version != taskSchemaVersion {
		t.Fatalf("wrong schema: %d %v", version, err)
	}
	db.Close()
	restored := NewManagerWithPersistence(nil, nil, nil, dir)
	if err = restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	got, _ := restored.GetTask(task.ID)
	if got.Prompt != "original" || got.Actor != "alice" {
		t.Fatal("migration altered ownership or data")
	}
}
