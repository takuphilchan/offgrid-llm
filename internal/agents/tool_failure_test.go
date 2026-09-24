package agents

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestReadFailuresNeverAskForInventedOutcome(t *testing.T) {
	for _, name := range []string{"list_files", "read_file", "calculator"} {
		t.Run(name, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			args, _ := json.Marshal(map[string]string{"path": filepath.Join(t.TempDir(), "missing"), "expression": "not valid math"})
			calls := 0
			r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				return taskCall(name, string(args)), nil
			})
			task, err := r.Create("read unavailable input", "model", "alice", taskConfig())
			if err != nil {
				t.Fatal(err)
			}
			task, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || task.Status != TaskFailed || task.Checkpoint.ExecutingCall != "" || calls != 1 {
				t.Fatalf("not a settled read failure: %+v %v calls=%d", task, err, calls)
			}
			if len(task.Steps) != 1 || task.Steps[0].Type != "tool_error" || !strings.Contains(task.Steps[0].ToolResult, `"effects":"none"`) {
				t.Fatal("failure evidence missing")
			}
			if _, err = r.Reconcile(task.ID, "alice", "invented", "done"); !errors.Is(err, ErrRunConflict) {
				t.Fatal("read failure accepted human outcome")
			}
			if !CanDeleteTask(task) {
				t.Fatal("settled failure cannot be deleted")
			}
		})
	}
}

type blockedReadTools struct {
	*ToolRegistry
	started chan struct{}
}

func (b *blockedReadTools) ExecuteWithPolicy(ctx context.Context, _ string, _ json.RawMessage, _ ToolExecution) (string, error) {
	close(b.started)
	<-ctx.Done()
	return "", ctx.Err()
}

func TestPauseAndCancelReadDoNotBecomeUncertain(t *testing.T) {
	for _, action := range []string{"pause", "cancel"} {
		t.Run(action, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := &blockedReadTools{NewToolRegistry(), make(chan struct{})}
			r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall("list_files", `{"path":"."}`), nil
			})
			defer r.Shutdown(context.Background())
			task, _ := r.Create("read", "model", "alice", taskConfig())
			if _, err := r.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
				t.Fatal(err)
			}
			select {
			case <-tools.started:
			case <-time.After(5 * time.Second):
				t.Fatal("not started")
			}
			var err error
			if action == "pause" {
				task, err = r.Pause(task.ID, "alice", false)
			} else {
				task, err = r.Stop(task.ID, "alice", "cancel", "")
			}
			if err != nil || task.Status == TaskUncertain || task.Checkpoint.ExecutingCall != "" {
				t.Fatalf("read needs reconciliation: %+v %v", task, err)
			}
			settleTask(t, r, task.ID)
		})
	}
}

func TestReadOnlyEvidenceIsNotInferredFromToolNameOrLowRisk(t *testing.T) {
	for _, d := range []capabilities.Descriptor{
		{Name: "list_files", Source: "user", Namespace: "tools", Kind: capabilities.Read, Risk: capabilities.RiskLow},
		{Name: "http_get", Source: "builtin", Namespace: "tools", Kind: capabilities.Network},
		{Name: "shell", Source: "builtin", Namespace: "tools", Kind: capabilities.Read},
	} {
		if readOnlyBuiltin(d) {
			t.Fatalf("untrusted read classification: %+v", d)
		}
	}
	if !uncertainEffect(&Checkpoint{ExecutingCall: "old"}) {
		t.Fatal("legacy intent lost uncertainty")
	}
	if !uncertainEffect(&Checkpoint{ExecutingCall: "new", ReadOnlyCall: "old"}) {
		t.Fatal("different call lost uncertainty")
	}
}

func TestRestartedReadRequiresExplicitResumeNotHumanOutcome(t *testing.T) {
	dir := t.TempDir()
	m := NewManagerWithPersistence(nil, nil, nil, dir)
	r := NewRunner(m, NewToolRegistry(), nil)
	task, err := r.Create("read", "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.updateTask(task.ID, func(task *Task) error {
		call := api.ToolCall{ID: "read-intent", Type: "function", Function: api.FunctionCall{Name: "list_files", Arguments: `{"path":"."}`}}
		task.Status = TaskRunning
		task.Checkpoint.Calls = []api.ToolCall{call}
		task.Checkpoint.ExecutingCall = call.ID
		task.Checkpoint.ReadOnlyCall = call.ID
		task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	restored := NewManagerWithPersistence(nil, nil, nil, dir)
	if err := restored.StorageError(); err != nil {
		t.Fatal(err)
	}
	got, ok := restored.GetTask(task.ID)
	if !ok || got.Status != TaskInterrupted || got.Checkpoint.ExecutingCall != "" || got.Checkpoint.ReadOnlyCall != "" || len(got.Checkpoint.Calls) != 1 {
		t.Fatalf("read restart needs manual outcome: %+v", got)
	}
}
