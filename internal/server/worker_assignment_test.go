package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Native side-effect scope: this test binary reads one fixture input and writes
// one proof file in its assigned temporary copy, then exits. No apps or network.
func TestWorkerCommandFixture(t *testing.T) {
	edited := slices.Contains(os.Args, "offgrid-worker-edited-fixture")
	if !edited && !slices.Contains(os.Args, "offgrid-worker-command-fixture") {
		return
	}
	data, err := os.ReadFile("input.txt")
	want := "user's dirty input"
	if edited {
		want = "sealed editor result"
	}
	if slices.Contains(os.Args, "offgrid-worker-followup-fixture") {
		want = "follow-up retained result"
	}
	if err != nil || string(data) != want {
		os.Exit(2)
	}
	if err = os.WriteFile("validation.txt", []byte("input verified"), 0600); err != nil {
		os.Exit(3)
	}
	os.Exit(0)
}

func TestAutomaticExecutionWorkerUsesPrivateCopyAndCanonicalResult(t *testing.T) {
	for _, mode := range []string{"edit", "test"} {
		t.Run(mode, func(t *testing.T) {
			s := profileFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			ref, source := approvedExecutionCopiesFixture(t, s, execution.Permissions{Read: true, Write: true, Processes: true}, execution.WorkingCopyPolicy{Version: 1, MaxTotalBytes: 4096, MaxCopyBytes: 1024, MaxEntries: 100, MaxCopies: 4})
			if err := os.WriteFile(filepath.Join(source, "input.txt"), []byte("user's dirty input"), 0600); err != nil {
				t.Fatal(err)
			}
			resource, err := s.executionHosts.Resolve(ctx, "local-admin", s.workspaceID, ref)
			if err != nil {
				t.Fatal(err)
			}
			tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
			name, arguments := "workspace_write", `{"path":"proposal.txt","content":"worker proposal"}`
			names := []string{name}
			if mode == "test" {
				name = "workspace_process_start"
				args, _ := json.Marshal(execution.ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestWorkerCommandFixture$", "--", "offgrid-worker-command-fixture"}, TimeoutSeconds: 10})
				arguments = string(args)
				names = []string{name, "workspace_process_wait"}
			}
			s.agentRunner = agents.NewRunner(s.agentManager, tools, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, _ []api.Tool) (*api.ChatCompletionResponse, error) {
				message := api.ChatMessage{Role: "assistant", Content: "Retained worker evidence; no original files were integrated."}
				reason := "stop"
				if task.ParentID == "" && len(task.ChildHistory) == 0 {
					args, _ := json.Marshal(map[string]any{"tasks": []agents.ChildSpec{{Key: "worker", Mode: mode, Goal: "Inspect the private input and produce the bounded result", Tools: names, DependsOn: []string{}}}})
					message.Content = ""
					message.ToolCalls = []api.ToolCall{{ID: "delegate", Type: "function", Function: api.FunctionCall{Name: "delegate_tasks", Arguments: string(args)}}}
					reason = "tool_calls"
				} else if task.ParentID != "" {
					if !agents.HasWorkerEnvironment(task) || task.Config.ExecutionWorkspace.Root == source || *task.Config.Execution == ref {
						t.Error("child ran without independently bound copy")
					}
					actions := 0
					var process execution.ProcessResult
					for _, step := range task.Steps {
						if step.Type == "action" {
							actions++
							if mode == "test" {
								var result struct {
									Receipt execution.Receipt `json:"receipt"`
								}
								if json.Unmarshal([]byte(step.ToolResult), &result) != nil || json.Unmarshal(result.Receipt.Result, &process) != nil {
									t.Error("invalid process receipt")
								}
							}
						}
					}
					if actions == 0 {
						message.Content = ""
						message.ToolCalls = []api.ToolCall{{ID: "private-action", Type: "function", Function: api.FunctionCall{Name: name, Arguments: arguments}}}
						reason = "tool_calls"
					} else if mode == "test" && process.State == "running" {
						args, _ := json.Marshal(map[string]any{"handle": process.Handle, "wait_ms": 1000})
						message.Content = ""
						message.ToolCalls = []api.ToolCall{{ID: fmt.Sprintf("wait-%d", actions), Type: "function", Function: api.FunctionCall{Name: "workspace_process_wait", Arguments: string(args)}}}
						reason = "tool_calls"
					} else if mode == "test" && (process.State != "exited" || process.ExitCode == nil || *process.ExitCode != 0) {
						t.Errorf("worker command did not verify input: %+v", process)
					}
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: reason, Message: message}}}, nil
			})
			defer s.agentRunner.Shutdown(context.Background())
			scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
			conversation, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
			if err != nil {
				t.Fatal(err)
			}
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst, cfg.ScopedCapabilities, cfg.MaxIterations, cfg.ContextWindow = true, true, 30, 32768
			cfg.Execution, cfg.ExecutionWorkspace = &ref, &resource
			cfg.AllowedCapabilities = map[string]capabilities.Descriptor{}
			for _, tool := range names {
				descriptor, _ := execution.Descriptor(tool)
				cfg.AllowedCapabilities[tool] = descriptor
			}
			accepted, err := s.agentRunner.AdmitConversationWork(scope, agents.ConversationCommand{RequestID: "work", TargetID: conversation.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Delegate the bounded private workspace assignment"}}}, "fixture-model", cfg)
			if err != nil {
				t.Fatal(err)
			}
			root, err := s.agentRunner.Continue(ctx, accepted.Work.RunID, scope.Actor, "start", "", false)
			if err != nil || len(root.ChildHistory) != 1 {
				t.Fatal(root, err)
			}
			if _, err = s.agentRunner.Continue(ctx, root.ChildHistory[0].ID, scope.Actor, "start", "", false); err == nil {
				t.Fatal("unprepared worker dispatched")
			}
			s.agentRunner.StartCoordinator(ctx)
			approved := false
			for root.Status != agents.TaskCompleted {
				if ctx.Err() != nil {
					child, _ := s.agentManager.GetTask(root.ChildHistory[0].ID)
					t.Fatalf("worker did not complete: root=%s child=%s error=%s steps=%+v", root.Status, child.Status, child.Error, child.Steps)
				}
				if root.Status == agents.TaskFailed {
					t.Fatal(root.Error)
				}
				child, _ := s.agentManager.GetTask(root.ChildHistory[0].ID)
				if child.Status == agents.TaskWaiting && child.PendingApproval != nil && !approved {
					if mode != "test" || child.PendingApproval.Tool != "workspace_process_start" {
						t.Fatal("unexpected worker approval")
					}
					// This fixture explicitly approves only its owned test helper.
					_, approvalErr := s.agentRunner.Continue(ctx, child.ID, scope.Actor, "approve", child.PendingApproval.ID, true)
					approved = approvalErr == nil
				}
				time.Sleep(10 * time.Millisecond)
				root, _ = s.agentManager.GetTask(root.ID)
			}
			child, _ := s.agentManager.GetTask(root.ChildHistory[0].ID)
			if child.Status != agents.TaskCompleted || len(child.Steps) < 1 || child.Steps[0].Type != "action" || child.Steps[0].ToolName != name {
				t.Fatalf("missing canonical receipt: %+v", child.Steps)
			}
			if mode == "edit" {
				data, err := os.ReadFile(filepath.Join(child.Config.ExecutionWorkspace.Root, "proposal.txt"))
				if err != nil || string(data) != "worker proposal" {
					t.Fatal(string(data), err)
				}
			} else {
				if !approved {
					t.Fatal("custom executable bypassed exact approval")
				}
				data, err := os.ReadFile(filepath.Join(child.Config.ExecutionWorkspace.Root, "validation.txt"))
				if err != nil || string(data) != "input verified" {
					t.Fatal("missing real worker-process evidence", err)
				}
				if _, err = os.Stat(filepath.Join(source, "validation.txt")); !os.IsNotExist(err) {
					t.Fatal("test process wrote original folder")
				}
				if _, err = os.Stat(filepath.Join(source, "proposal.txt")); !os.IsNotExist(err) {
					t.Fatal("worker mutated original", err)
				}
			}
			data, _ := os.ReadFile(filepath.Join(source, "input.txt"))
			if string(data) != "user's dirty input" {
				t.Fatal("original input changed")
			}
			if root.Checkpoint.StorageBudget == nil || root.Checkpoint.StorageBudget.ChargedBytes != 1024 {
				t.Fatal("copy was not charged once to root")
			}
			for _, reservation := range root.Checkpoint.StorageBudget.Reservations {
				if reservation.Capture == nil || reservation.Capture.State != "sealed" || reservation.Capture.Result == nil || reservation.Capture.Result.OperationCount != 1 {
					t.Fatal("parent synthesized before sealed output", reservation.Capture)
				}
				if reservation.Capture.Request.MaxReservedBytes != 2048 {
					t.Fatal("capture capacity was not reserved before dispatch")
				}
			}
		})
	}
}
