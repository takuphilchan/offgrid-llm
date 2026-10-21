package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// The real hub error envelope passes through service execution and the durable
// runner. A fake saved {error:...} bypassed both boundaries in the earlier tests.
// This provider only returns observations; it has no OS input capability.
func TestDocumentUnavailableContinuesToFreshControlsThroughDurableRunner(t *testing.T) {
	for _, hostError := range []error{computer.ErrDocumentUnavailable, computer.ErrDocumentIncomplete, computer.ErrControlScope} {
		t.Run(hostError.Error(), func(t *testing.T) {
			s, adapter, _, _, _ := compositionFixture(t)
			code, _ := s.browserHub.PairCode("alice")
			target := computer.TargetIdentity{ID: "fixture-window", OSSession: "fixture-os", ProcessGeneration: "fixture-process", Surface: "fixture-surface", Driver: "windows-uia"}
			session, token, err := s.browserHub.PairNativeFeatures(code, "Owned ordinary form", target, 2, computer.ApprovalFullTask, computer.NativeFeatures{Documents: 1})
			if err != nil {
				t.Fatal(err)
			}
			cfg := compositionConfig(target.Driver, true)
			cfg.ComputerSession, cfg.NativeWorkflowVersion = session.ID, agents.DocumentWorkflowEvidenceVersion
			calls := 0
			runner := agents.NewRunner(s.agentManager, adapter, func(_ context.Context, task *agents.Task, _ []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				names := availableNames(tools)
				name := "computer_observe"
				switch calls {
				case 1:
					if !names[name] {
						t.Fatal("initial observation missing")
					}
				case 2:
					name = "computer_documents"
				case 3:
					if hostError != computer.ErrDocumentUnavailable {
						t.Fatal("non-capability error was treated as ordinary form")
					}
					if names["computer_replace_text"] || names["computer_activate"] || !names["computer_observe"] {
						t.Fatal("fallback reused the pre-discovery view", names)
					}
					last := task.Steps[len(task.Steps)-1]
					if !isUnavailableDocumentCapability(last) || last.Type != "action" || task.Checkpoint.NativeReadRepair != nil || task.Checkpoint.ProposalRepairs != 0 {
						t.Fatal("negative capability became a failed/retrying task", last, task.Checkpoint.NativeReadRepair)
					}
				case 4:
					if !names["computer_replace_text"] || !names["computer_activate"] || !availableNames(adapter.ToolsForTask(task))["mcp__docs__docs"] {
						t.Fatal("fresh ordinary controls or independent tools unavailable", names)
					}
					// The regression ends at the recovered capability boundary. No
					// mutation is requested, and this is not native qualification.
					return nil, context.Canceled
				default:
					t.Fatal("unexpected retry", calls)
				}
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: fmt.Sprintf("read-%d", calls), Type: "function", Function: api.FunctionCall{Name: name, Arguments: `{}`}}}}}}}, nil
			})
			defer runner.Shutdown(context.Background())
			task, err := runner.Create("Inspect the current document surface without changing it", "fixture-model", "alice", cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.browserHub.Reserve("alice", session.ID, task.ID); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				observations := 0
				for {
					select {
					case <-ctx.Done():
						done <- nil
						return
					case <-time.After(time.Millisecond):
					}
					action, pollErr := s.browserHub.Poll(token)
					if pollErr != nil {
						// Scope violations intentionally revoke the host.
						if hostError == computer.ErrControlScope {
							done <- nil
						} else {
							done <- pollErr
						}
						return
					}
					if action == nil {
						continue
					}
					reply := computer.BrowserReply{ID: action.ID}
					switch action.Kind {
					case "computer_documents":
						reply.Error = hostError.Error()
					case "computer_observe":
						observations++
						value := "initial value"
						view := computer.NativeView{Observation: computer.ControlObservation{ID: fmt.Sprintf("fresh-view-%d", observations), Target: target, CapturedAt: time.Now()}, Elements: []computer.NativeElement{{ID: "field", Identity: "stable-field", Name: "Search", Role: "edit", Text: &value, Writable: true}, {ID: "button", Identity: "stable-button", Name: "Search", Role: "button", Invokable: true}}}
						encoded, _ := json.Marshal(view)
						reply.Result = string(encoded)
					default:
						done <- fmt.Errorf("unexpected native dispatch %s", action.Kind)
						return
					}
					if err := s.browserHub.Reply(token, reply); err != nil {
						done <- err
						return
					}
				}
			}()
			result, err := runner.Continue(ctx, task.ID, "alice", "start", "", false)
			cancel()
			if hostErr := <-done; hostErr != nil {
				t.Fatal(hostErr)
			}
			if hostError == computer.ErrDocumentUnavailable {
				if !errors.Is(err, context.Canceled) || calls != 4 || result.Status == agents.TaskFailed {
					t.Fatalf("capability fallback failed: calls=%d err=%v status=%s", calls, err, result.Status)
				}
			} else if err != nil || calls != 2 || result.Status != agents.TaskFailed {
				t.Fatalf("unsafe failure recovery: calls=%d err=%v status=%s", calls, err, result.Status)
			}
		})
	}
}
