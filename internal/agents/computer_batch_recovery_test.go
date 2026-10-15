package agents

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func computerBatchAnswer(names ...string) *api.ChatCompletionResponse {
	message := api.ChatMessage{}
	for _, name := range names {
		message.ToolCalls = append(message.ToolCalls, api.ToolCall{ID: "reused-model-id", Function: api.FunctionCall{Name: name, Arguments: `{}`}})
	}
	return runAnswer(message)
}

func newComputerBatchTask(t *testing.T, r *Runner) *Task {
	t.Helper()
	cfg := DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 16384
	cfg.ComputerSession, cfg.ComputerDriver, cfg.ComputerScope = "fixture-session", "windows-uia", "desktop"
	task, err := r.Create("Read the fixture, one operation at a time.", "fixture-model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestComputerBatchRepairedBeforeAnyDispatch(t *testing.T) {
	for _, correction := range []string{"computer_applications", "task_plan"} {
		t.Run(correction, func(t *testing.T) {
			authorized, calls := 0, 0
			tools := &runTestTools{authorize: func(name string, _ json.RawMessage, _ ToolExecution) error {
				authorized++
				if name != correction {
					t.Fatalf("rejected call authorized: %s", name)
				}
				return nil
			}, execute: func(string, json.RawMessage, ToolExecution) (string, error) { return `{}`, nil }}
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if calls == 1 {
					return computerBatchAnswer(correction, "computer_replace_text", "mcp__write"), nil
				}
				if calls == 2 {
					if authorized != 0 || tools.calls.Load() != 0 || len(task.Checkpoint.Calls) != 0 || task.Checkpoint.ExecutingCall != "" || task.PendingApproval != nil {
						t.Fatal("batch was queued, authorized or dispatched")
					}
					messages := task.Checkpoint.Messages
					proposal := messages[len(messages)-4]
					ids := map[string]bool{}
					for i, call := range proposal.ToolCalls {
						reply := messages[len(messages)-3+i]
						if ids[call.ID] || call.ID == "reused-model-id" || reply.ToolCallID != call.ID || reply.Role != "tool" || !strings.Contains(reply.StringContent(), `"effects":"none"`) {
							t.Fatal("rejected tool messages are not uniquely paired")
						}
						ids[call.ID] = true
					}
					answer := computerBatchAnswer(correction)
					if correction == "task_plan" {
						answer.Choices[0].Message.ToolCalls[0].Function.Arguments = `{"items":[{"id":"read","title":"Read fixture","state":"pending"}]}`
					}
					return answer, nil
				}
				return runAnswer(api.ChatMessage{Content: "Fixture checked."}), nil
			})
			task := newComputerBatchTask(t, r)
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskCompleted || calls != 3 || got.Checkpoint.ProposalUnresolved || got.Checkpoint.ProposalRepairs != 0 {
				t.Fatalf("repair failed: task=%+v calls=%d err=%v", got, calls, err)
			}
			want := int32(1)
			if correction == "task_plan" {
				want = 0
			}
			if tools.calls.Load() != want {
				t.Fatalf("executed %d tools, want %d", tools.calls.Load(), want)
			}
		})
	}
}

func TestComputerBatchRepairBoundedAndPersisted(t *testing.T) {
	for _, prose := range []bool{false, true} {
		t.Run(map[bool]string{false: "repeated-batch", true: "false-completion"}[prose], func(t *testing.T) {
			dir := t.TempDir()
			calls := 0
			tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error { t.Fatal("batch reached authorization"); return nil }}
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, dir), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if prose && calls > 1 {
					return runAnswer(api.ChatMessage{Content: "All done."}), nil
				}
				return computerBatchAnswer("computer_open_application", "computer_replace_text"), nil
			})
			task := newComputerBatchTask(t, r)
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || calls != 4 || tools.calls.Load() != 0 || !CanSteerTask(got) {
				t.Fatalf("repair not bounded: %+v calls=%d err=%v", got, calls, err)
			}
			if err := r.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			reloaded := NewManagerWithPersistence(nil, nil, nil, dir)
			saved, ok := reloaded.GetTask(task.ID)
			if !ok || saved.Checkpoint.ProposalRepairs != 4 || !saved.Checkpoint.ProposalUnresolved || len(saved.Checkpoint.Calls) != 0 || saved.Checkpoint.ExecutingCall != "" || saved.PendingApproval != nil || !saved.ComputerSessionExpired {
				t.Fatalf("restart lost repair or retained dispatch: %+v", saved)
			}
		})
	}
}

func TestComputerBatchKeepsEarlierActionAndOutstandingRepair(t *testing.T) {
	for _, guard := range []string{"", "task_expectations", "computer_open_application", "request_computer_access", "read", "edit"} {
		t.Run("guard="+guard, func(t *testing.T) {
			tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error { return nil }, execute: func(string, json.RawMessage, ToolExecution) (string, error) { return `{}`, nil }}
			calls := 0
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				switch calls {
				case 1:
					return computerBatchAnswer("computer_open_application"), nil
				case 2:
					if guard != "" {
						task.Checkpoint.ProposalTool = guard
						task.Checkpoint.ProposalRepairs = 1
						task.Checkpoint.ProposalUnresolved = true
					}
					if guard == "read" {
						task.Checkpoint.NativeReadRepair = &NativeReadRepair{Tool: "computer_observe"}
					}
					if guard == "edit" {
						task.Checkpoint.WorkflowEditPending = true
					}
					return computerBatchAnswer("computer_open_application", "computer_replace_text"), nil
				case 3:
					if tools.calls.Load() != 1 {
						t.Fatal("earlier action replayed")
					}
					if guard != "" && task.Checkpoint.ProposalTool != guard {
						t.Fatal("batch replaced existing repair guard")
					}
					return nil, context.Canceled
				}
				t.Fatal("unexpected inference")
				return nil, context.Canceled
			})
			task := newComputerBatchTask(t, r)
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if !errors.Is(err, context.Canceled) || got.Status != TaskInterrupted || tools.calls.Load() != 1 || len(got.Checkpoint.Calls) != 0 {
				t.Fatalf("invalid recovery: %+v %v", got, err)
			}
			resolveComputerToolSequence(got.Checkpoint, "computer_open_application")
			if guard != "" && !got.Checkpoint.ProposalUnresolved {
				t.Fatal("sequencing repair cleared a stronger guard")
			}
		})
	}
}

func TestComputerBatchPersistenceFailureNeverDispatches(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	tools := &runTestTools{}
	r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		m.mu.Lock()
		m.dataDir = blocked
		m.mu.Unlock()
		return computerBatchAnswer("computer_open_application", "computer_replace_text"), nil
	})
	task := newComputerBatchTask(t, r)
	_, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if !errors.Is(err, ErrRunStorage) || tools.calls.Load() != 0 {
		t.Fatalf("storage failure dispatched: %v", err)
	}
}

func TestNonComputerBatchUnchanged(t *testing.T) {
	tools := &runTestTools{authorize: func(string, json.RawMessage, ToolExecution) error { return nil }, execute: func(string, json.RawMessage, ToolExecution) (string, error) { return `{}`, nil }}
	calls := 0
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		if calls == 1 {
			return computerBatchAnswer("first", "second"), nil
		}
		return runAnswer(api.ChatMessage{Content: "Done."}), nil
	})
	task, err := r.Create("fixture", "model", "alice", DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || got.Status != TaskCompleted || tools.calls.Load() != 2 {
		t.Fatalf("legacy batch changed: %+v %v", got, err)
	}
}

func TestComputerBatchDoesNotHideDeclarationRecovery(t *testing.T) {
	calls := 0
	tools := &workflowTestTools{}
	r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), tools, func(_ context.Context, task *Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
		calls++
		switch calls {
		case 1:
			return taskCall("task_expectations", prematureControlExpectation), nil
		case 2:
			return computerBatchAnswer("task_expectations", "computer_open_application"), nil
		case 3:
			return taskCall("task_expectations", `{"expectations":[{"id":"open","title":"Editor ready","kind":"application_open","application_id":"editor"}]}`), nil
		case 4:
			if task.Checkpoint.ProposalUnresolved || task.Checkpoint.ProposalRepairs != 0 || len(task.Workflow.Checks) != 1 || task.Workflow.Checks[0].State != "pending" {
				t.Fatal("sequencing feedback hid the corrected declaration, or claimed completion")
			}
			return nil, context.Canceled
		default:
			t.Fatal("unexpected inference")
			return nil, context.Canceled
		}
	})
	task, err := r.Create("Open editor", "model", "alice", workflowConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if !errors.Is(err, context.Canceled) || tools.calls.Load() != 0 || calls != 4 {
		t.Fatalf("invalid declaration recovery: %v calls=%d effects=%d", err, calls, tools.calls.Load())
	}
}

func TestComputerBatchDoesNotHideVerifiedApplicationProgress(t *testing.T) {
	task := &Task{Steps: []Step{
		{Type: "rejected", ToolName: "computer_open_application", ToolResult: `{"error":"computer_application_already_ready"}`},
		{Type: "rejected", ToolName: "computer_open_application", ToolResult: `{"error":"computer_multiple_tool_calls"}`},
	}}
	if !advancedVerifiedApplicationRepair(task, "computer_create_document") || advancedVerifiedApplicationRepair(task, "computer_applications") {
		t.Fatal("batch rejection changed what counts as application progress")
	}
}
