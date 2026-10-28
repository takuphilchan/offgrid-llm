package agents

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type processProposalTools struct{ *runTestTools }

// This fixture records dispatch only; it never starts an OS process.
func (*processProposalTools) StopOwnedExecution(context.Context, *Task) error { return nil }

func TestRepeatedProgramRecoveryBoundaries(t *testing.T) {
	for _, state := range []string{"safe", "dispatched", "child", "legacy", "no-host", "no-checkpoint", "other-tool"} {
		t.Run(state, func(t *testing.T) {
			task := &Task{Config: AgentConfig{TaskFirst: true, Execution: &execution.Reference{}}, Checkpoint: &Checkpoint{}}
			tool := "workspace_process_start"
			switch state {
			case "dispatched":
				task.Checkpoint.ExecutingCall = "persisted-intent"
			case "child":
				task.ParentID = "parent"
			case "legacy":
				task.Config.TaskFirst = false
			case "no-host":
				task.Config.Execution = nil
			case "no-checkpoint":
				task.Checkpoint = nil
			case "other-tool":
				tool = "workspace_write"
			}
			got := proposalRecovery(task, tool, &ToolAuthorizationError{Code: "execution_program_repeated"})
			if (got != "") != (state == "safe") {
				t.Fatalf("unsafe recovery for %s: %s", state, got)
			}
		})
	}
}

func TestRepeatedProgramRepairIsBoundedAndDoesNotReplay(t *testing.T) {
	for _, correct := range []bool{false, true} {
		t.Run(map[bool]string{false: "limit", true: "corrected"}[correct], func(t *testing.T) {
			calls := 0
			tools := &runTestTools{authorize: func(_ string, args json.RawMessage, _ ToolExecution) error {
				if strings.Contains(string(args), `["go",`) {
					return &ToolAuthorizationError{Code: "execution_program_repeated", Message: "No command dispatched"}
				}
				return nil
			}, execute: func(_ string, args json.RawMessage, _ ToolExecution) (string, error) {
				if string(args) != `{"arguments":["test","."],"program":"go"}` {
					t.Fatalf("unexpected dispatched arguments: %s", args)
				}
				return `{"state":"exited","exit_code":0}`, nil
			}}
			r := NewRunner(NewManagerWithPersistence(nil, nil, nil, t.TempDir()), &processProposalTools{tools}, func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if correct && calls == 3 {
					return runAnswer(api.ChatMessage{Content: "Tests exited successfully."}), nil
				}
				args := `{"program":"go","arguments":["go","test","."]}`
				if correct && calls == 2 {
					if !strings.Contains(messages[len(messages)-1].StringContent(), "No command was dispatched") {
						t.Fatal("missing correction feedback")
					}
					args = `{"program":"go","arguments":["test","."]}`
				}
				return runAnswer(api.ChatMessage{ToolCalls: []api.ToolCall{{Function: api.FunctionCall{Name: "workspace_process_start", Arguments: args}}}}), nil
			})
			t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
			cfg := DefaultAgentConfig()
			cfg.TaskFirst = true
			cfg.ContextWindow = 16384
			cfg.Execution = &execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 1}
			cfg.ExecutionWorkspace = &execution.Resource{Reference: *cfg.Execution, Backend: "local", Root: "/fixture", Directory: ".", Permissions: execution.Permissions{Read: true, Write: true, Processes: true}}
			task, err := r.Create("Run go test .", "model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil {
				t.Fatal(err)
			}
			if correct {
				if got.Status != TaskCompleted || tools.calls.Load() != 1 || calls != 3 {
					t.Fatalf("corrected proposal failed: status=%s code=%s dispatched=%d model=%d", got.Status, got.ErrorCode, tools.calls.Load(), calls)
				}
			} else if got.Status != TaskInterrupted || got.ErrorCode != proposalRepairLimitCode || tools.calls.Load() != 0 || calls != 4 {
				t.Fatalf("repair unbounded: %+v calls=%d", got, calls)
			}
			if got.PendingApproval != nil || got.Checkpoint.ExecutingCall != "" {
				t.Fatal("retained executable rejection")
			}
		})
	}
}
