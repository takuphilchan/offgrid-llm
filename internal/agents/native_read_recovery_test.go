package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type nativeReadTestTools struct {
	*runTestTools
	source, revision string
	read             func(context.Context) (string, error)
}

func (n *nativeReadTestTools) ExecuteWithPolicy(ctx context.Context, name string, args json.RawMessage, e ToolExecution) (string, error) {
	if n.read != nil {
		n.calls.Add(1)
		return n.read(ctx)
	}
	return n.runTestTools.ExecuteWithPolicy(ctx, name, args, e)
}

func newNativeReadTestTools() *nativeReadTestTools {
	return &nativeReadTestTools{runTestTools: &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error { return nil }}, source: "local-companion", revision: capabilities.NativeEffectsRevision}
}

func (n *nativeReadTestTools) Capability(name string) (capabilities.Descriptor, bool) {
	effect, known := capabilities.NativeOperationEffect(name, capabilities.NativeEffectsRevision)
	risk := capabilities.RiskHigh
	if effect == capabilities.NativeRead {
		risk = capabilities.RiskLow
	}
	return capabilities.Descriptor{Name: name, Namespace: "computer", Source: n.source, Revision: n.revision, Kind: capabilities.Computer, Risk: risk}, known
}

func TestNativeReadFailuresNeverRequireMutationReconciliation(t *testing.T) {
	for _, name := range []string{"computer_observe", "computer_verify_application", "computer_applications", "computer_windows", "computer_verify", "computer_verify_checked", "computer_find_controls", "computer_inspect_control"} {
		t.Run(name, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := newNativeReadTestTools()
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				return "", errors.New("private provider details")
			}
			r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall(name, `{}`), nil
			})
			task, err := r.Create("observe", "model", "alice", taskConfig())
			if err != nil {
				t.Fatal(err)
			}
			task, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || task.Status != TaskFailed || task.Checkpoint.ExecutingCall != "" || tools.calls.Load() != 1 {
				t.Fatalf("read left uncertain: %+v %v", task, err)
			}
			if len(task.Steps) != 1 || !strings.Contains(task.Steps[0].ToolResult, `"effects":"none"`) || strings.Contains(task.Error, "private provider") {
				t.Fatal("missing or unredacted read failure evidence")
			}
			if _, err := r.Reconcile(task.ID, "alice", "invented", "done"); !errors.Is(err, ErrRunConflict) {
				t.Fatal("read allowed reconciliation")
			}
		})
	}
}

func TestNativeNamesAndUnknownRevisionsDoNotSetReadIntent(t *testing.T) {
	for _, variant := range []string{"mcp", "old-revision", "mutation"} {
		t.Run(variant, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := newNativeReadTestTools()
			name := "computer_observe"
			switch variant {
			case "mcp":
				tools.source = "mcp"
			case "old-revision":
				tools.revision = ""
			case "mutation":
				name = "computer_open_application"
			}
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				return "", errors.New("lost acknowledgment")
			}
			r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall(name, `{}`), nil
			})
			task, err := r.Create("work", "model", "alice", taskConfig())
			if err != nil {
				t.Fatal(err)
			}
			_, _ = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			task, _ = m.GetTask(task.ID)
			if task.Status != TaskUncertain || !uncertainEffect(task.Checkpoint) || tools.calls.Load() != 1 {
				t.Fatalf("uncertainty lost: %+v", task)
			}
		})
	}
}

func TestNativeReadRecoveryBudgetSurvivesUnrelatedReads(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	tools := newNativeReadTestTools()
	tools.execute = func(name string, _ json.RawMessage, _ ToolExecution) (string, error) {
		if name == "computer_applications" {
			return `{"applications":[]}`, nil
		}
		return "", &ToolFailure{Code: "computer_target_not_selected", Message: "This read made no changes; an earlier launch remains recorded.", Recovery: "Verify the earlier application; do not relaunch."}
	}
	modelCalls := 0
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		modelCalls++
		if modelCalls%2 == 0 {
			return taskCall("computer_applications", `{}`), nil
		}
		return taskCall("computer_observe", `{}`), nil
	})
	cfg := taskConfig()
	cfg.MaxIterations = 20
	task, err := r.Create("inspect the opened application", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != TaskInterrupted || task.ErrorCode != proposalRepairLimitCode || modelCalls != 7 || task.Checkpoint.ProposalRepairs != 4 || task.Checkpoint.ExecutingCall != "" {
		t.Fatalf("unbounded or uncertain read recovery: %+v %v calls=%d", task, err, modelCalls)
	}
	if task.Checkpoint.NativeReadRepair == nil {
		t.Fatal("lost outstanding repair")
	}
	restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
	loaded, _ := restored.GetTask(task.ID)
	if loaded.Status != TaskInterrupted || loaded.Checkpoint.ProposalRepairs != 4 || loaded.Checkpoint.NativeReadRepair == nil {
		t.Fatal("restart lost repair budget")
	}
}

func TestNativeReadRecoveryRequiresMatchingProgress(t *testing.T) {
	cp := &Checkpoint{NativeReadRepair: &NativeReadRepair{Tool: "computer_verify_application", Application: "requested"}}
	for _, tc := range []struct{ tool, result string }{
		{"computer_applications", `{"applications":[{"id":"requested"}]}`},
		{"computer_verify_application", `{"verified":false,"check":"application_open","application_id":"requested"}`},
		{"computer_verify_application", `{"verified":true,"check":"application_open","application_id":"other","target":{"id":"window"}}`},
		{"computer_observe", `{"observation":{"id":"o","target":{"id":"window"}}}`},
	} {
		resolveNativeReadRepair(cp, tc.tool, tc.result)
		if cp.NativeReadRepair == nil {
			t.Fatal("unrelated or negative readiness resolved recovery")
		}
	}
	resolveNativeReadRepair(cp, "computer_verify_application", `{"verified":true,"check":"application_open","application_id":"requested","target":{"id":"window"}}`)
	if cp.NativeReadRepair != nil {
		t.Fatal("matching readiness did not resolve recovery")
	}
}

func TestMissingObservationRepairPinsEarlierLaunch(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, newNativeReadTestTools(), nil)
	task, err := r.Create("inspect the opened editor", "model", "alice", taskConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(current *Task) error {
		current.Status = TaskRunning
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	task.Config.ComputerScope = "desktop"
	task.Steps = []Step{{ID: 1, Type: "action", ToolName: "computer_open_application", ToolArgs: `{"application_id":"editor"}`, ToolResult: `{"actions":[{"outcome":"dispatched"}]}`}}
	call := api.ToolCall{ID: "read", Type: "function", Function: api.FunctionCall{Name: "computer_observe", Arguments: `{}`}}
	if err := r.repairNativeRead(task, call); err != nil {
		t.Fatal(err)
	}
	if task.Checkpoint.NativeReadRepair == nil || task.Checkpoint.NativeReadRepair.Application != "editor" {
		t.Fatal("launch identity missing from observation repair")
	}
	resolveNativeReadRepair(task.Checkpoint, "computer_verify_application", `{"verified":true,"check":"application_open","application_id":"other","target":{"id":"window"}}`)
	if task.Checkpoint.NativeReadRepair == nil {
		t.Fatal("another application's readiness reset the budget")
	}
	resolveNativeReadRepair(task.Checkpoint, "computer_verify_application", `{"verified":true,"check":"application_open","application_id":"editor","target":{"id":"window"}}`)
	if task.Checkpoint.NativeReadRepair != nil {
		t.Fatal("matching readiness did not resolve repair")
	}
}

func TestNativeReadRestartPreservesEarlierLaunchAndLegacyUncertainty(t *testing.T) {
	for _, proven := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "classified"}[proven], func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := newNativeReadTestTools()
			r := NewRunner(m, tools, nil)
			task, err := r.Create("observe launched editor", "model", "alice", taskConfig())
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.updateTask(task.ID, func(task *Task) error {
				task.Config.ComputerScope, task.Config.ComputerSession = "desktop", "fixture-host"
				call := api.ToolCall{ID: "observe-intent", Type: "function", Function: api.FunctionCall{Name: "computer_observe", Arguments: `{}`}}
				task.Status = TaskRunning
				task.Steps = []Step{{ID: 1, Type: "action", ToolName: "computer_open_application", ToolArgs: `{"application_id":"editor"}`, ToolResult: `{"actions":[{"outcome":"dispatched"}]}`}}
				task.Checkpoint.Calls = []api.ToolCall{call}
				task.Checkpoint.ExecutingCall = call.ID
				task.Checkpoint.Messages = append(task.Checkpoint.Messages, api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{call}})
				if proven {
					d, _ := tools.Capability(call.Function.Name)
					task.Checkpoint.ReadOnlyCall = call.ID
					task.Checkpoint.ReadOnlyCapability = &d
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			restored := NewManagerWithPersistence(nil, nil, nil, m.dataDir)
			if err := restored.StorageError(); err != nil {
				t.Fatal(err)
			}
			got, _ := restored.GetTask(task.ID)
			want := TaskUncertain
			if proven {
				want = TaskInterrupted
			}
			if got.Status != want || len(got.Steps) != 1 || len(PendingApplicationLaunches(got)) != 1 || tools.calls.Load() != 0 {
				t.Fatalf("restart changed work: %+v", got)
			}
			if proven && (got.Checkpoint.ExecutingCall != "" || got.Checkpoint.ReadOnlyCapability != nil || len(got.Checkpoint.Calls) != 1) {
				t.Fatal("classified read not safely interrupted")
			}
		})
	}
}

func TestNativeReadPersistenceFailureDoesNotDispatchOrInventMutation(t *testing.T) {
	for _, phase := range []string{"intent", "result"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithPersistence(nil, nil, nil, dir)
			tools := newNativeReadTestTools()
			blocked := filepath.Join(t.TempDir(), "not-directory")
			if err := os.WriteFile(blocked, []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			breakStorage := func() { m.mu.Lock(); m.dataDir = blocked; m.mu.Unlock() }
			tools.authorize = func(string, json.RawMessage, ToolExecution) error {
				if phase == "intent" {
					breakStorage()
				}
				return nil
			}
			tools.execute = func(string, json.RawMessage, ToolExecution) (string, error) {
				if phase == "result" {
					breakStorage()
				}
				return `{"observation":{"id":"view","target":{"id":"editor"}}}`, nil
			}
			r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall("computer_observe", `{}`), nil
			})
			task, err := r.Create("observe", "model", "alice", taskConfig())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false); !errors.Is(err, ErrRunStorage) {
				t.Fatalf("storage error hidden: %v", err)
			}
			want := int32(0)
			if phase == "result" {
				want = 1
			}
			if tools.calls.Load() != want {
				t.Fatal("dispatch count", tools.calls.Load())
			}
			restored := NewManagerWithPersistence(nil, nil, nil, dir)
			if err := restored.StorageError(); err != nil {
				t.Fatal(err)
			}
			got, _ := restored.GetTask(task.ID)
			if got.Status != TaskInterrupted || got.Checkpoint.ExecutingCall != "" {
				t.Fatalf("read persistence became uncertain: %+v", got)
			}
		})
	}
}

func TestNativeReadStopAndPauseDoNotReplay(t *testing.T) {
	for _, action := range []string{"pause", "cancel"} {
		t.Run(action, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := newNativeReadTestTools()
			started := make(chan struct{})
			tools.read = func(ctx context.Context) (string, error) { close(started); <-ctx.Done(); return "", ctx.Err() }
			r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return taskCall("computer_observe", `{}`), nil
			})
			defer r.Shutdown(context.Background())
			task, err := r.Create("observe", "model", "alice", taskConfig())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = r.Continue(context.Background(), task.ID, "alice", "start", "", true); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("read never started")
			}
			if action == "pause" {
				task, err = r.Pause(task.ID, "alice", false)
			} else {
				task, err = r.Stop(task.ID, "alice", "cancel", "")
			}
			if err != nil || task.Status == TaskUncertain || task.Checkpoint.ExecutingCall != "" || task.Checkpoint.ReadOnlyCapability != nil {
				t.Fatalf("read stopped as mutation: %+v %v", task, err)
			}
			settleTask(t, r, task.ID)
			if tools.calls.Load() != 1 {
				t.Fatal("read replayed")
			}
		})
	}
}
