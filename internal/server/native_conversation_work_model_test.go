package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeConversationWorkInstalledModel(t *testing.T) {
	if os.Getenv("OFFGRID_TEST_DISPOSABLE_EXECUTION") != "1" {
		t.Skip("requires explicit disposable file/process fixture opt-in")
	}
	engine, model := installedConversationEngine(t)
	adapter := os.Getenv("OFFGRID_TEST_DECISION_ADAPTER")
	if adapter != "" && adapter != "constrained" && adapter != "native" {
		t.Fatal("unknown test-only decision adapter")
	}
	t.Logf("decision_adapter=%s (no production configuration changed)", adapter)
	for _, attached := range []bool{false, true} {
		t.Run(fmt.Sprintf("attached-%t", attached), func(t *testing.T) {
			f := newConversationFixture(t)
			root := t.TempDir()
			private := t.TempDir()
			const original = "Exact source — café 日本語 12345.\n"
			const wanted = "Updated fixture — café 日本語 67890.\n"
			expectedFiles := map[string]string{"source.txt": original, "neighbor.txt": "Preserve me.\n", "go.mod": "module offgrid-disposable\n\ngo 1.24\n", "fixture_test.go": "package fixture\nimport \"testing\"\nfunc TestFixture(t *testing.T) { if 2+2!=4 {t.Fatal(\"arithmetic\")} }\n"}
			for name, text := range expectedFiles {
				if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			binding := execution.Binding{Actor: "local-admin", Workspace: f.s.workspaceID, Host: "disposable-fixture", Session: "fixture-session", Grant: "fixture-grant", Revision: 1, Job: execution.BootstrapJob}
			hostConfig := execution.Context{Version: 1, Binding: binding, Backend: "local", Root: root, Directory: ".", Permissions: execution.Permissions{Read: true, Write: true, Processes: true}}
			host, err := execution.OpenSession(hostConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer host.Close()
			journal, err := execution.OpenJournal(private)
			if err != nil {
				t.Fatal(err)
			}
			defer journal.Close()
			worker := &execution.Worker{Session: host, Journal: journal, Processes: execution.NewProcessManager(host), Token: strings.Repeat("a", 64)}
			worker.AuthorizeProcess = execution.TrustedProcessPolicy
			endpoint := execution.NewLocalEndpoint(worker)
			defer endpoint.Close()
			f.s.executionHosts = execution.NewHosts()
			if err = f.s.executionHosts.RegisterApproved(context.Background(), hostConfig, endpoint); err != nil {
				t.Fatal(err)
			}
			ref := execution.Reference{Host: binding.Host, Session: binding.Session, Grant: binding.Grant, Revision: 1}
			resource, err := f.s.executionHosts.Resolve(context.Background(), binding.Actor, binding.Workspace, ref)
			if err != nil {
				t.Fatal(err)
			}
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst = true
			cfg.ExecutionRequestVersion = 1
			cfg.ContextWindow = 16384
			cfg.MaxIterations = 20
			cfg.MaxTokens = 1024
			cfg.Temperature = 0
			cfg.SystemPrompt = conversationTaskProtocol + "\n" + taskFirstPrompt + serviceFilePrompt
			cfg.AllowedCapabilities = map[string]capabilities.Descriptor{execution.AccessRequestTool: execution.AccessRequestDescriptor()}
			if attached {
				cfg.Execution = &ref
				cfg.ExecutionWorkspace = &resource
				cfg.InputWriteVersion = execution.InputWriteVersion
				for _, tool := range execution.Tools() {
					d, _ := execution.Descriptor(tool.Function.Name)
					cfg.AllowedCapabilities[d.Name] = d
				}
			}
			calls := 0
			applyNativeWorkspaceTestScope(t, &cfg, adapter)
			runner := agents.NewRunner(f.s.agentManager, &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				names := make([]string, 0, len(tools))
				for _, tool := range tools {
					names = append(names, tool.Function.Name)
				}
				t.Logf("decision=%d effective_tools=%v", calls, names)
				if task.Config.Execution == nil {
					found := false
					for _, tool := range tools {
						found = found || tool.Function.Name == execution.AccessRequestTool
					}
					if !found {
						t.Fatal("workspace request missing from effective model catalog")
					}
				}
				zero, max, no := float32(0), 1024, false
				req := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max, SingleStepTools: adapter != "native", ParallelToolCalls: &no, StreamOptions: &api.StreamOptions{IncludeUsage: true}}
				acc := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
				err := engine.ChatCompletionStreamRaw(ctx, req, func(raw json.RawMessage) error { _, _, err := acc.add(raw); return err })
				if err != nil {
					t.Logf("isolated decision=%d validation_error=%T: %v", calls, err, err)
					return nil, err
				}
				return acc.response(), nil
			})
			defer runner.Shutdown(context.Background())
			approvals := 0
			for _, item := range []struct{ name, prompt string }{
				{"list", "List files in this directory."},
				{"read", "Read source.txt in the same folder and return its complete text."},
				{"edit", "Replace source.txt with exactly the text inside the following unique content markers. Read it back and verify. Do not change any other file. <content>" + wanted + "</content>"},
				{"test", "Run go test . in this same folder. Wait until it ends and report its actual exit code and output. Do not edit files."},
			} {
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
				start := time.Now()
				prior := calls
				task, err := runner.Create(item.prompt, model, binding.Actor, cfg)
				if err != nil {
					cancel()
					t.Fatal(err)
				}
				result, runErr := runner.Continue(ctx, task.ID, task.Actor, "start", "", false)
				if result != nil && result.Status == agents.TaskInput && result.PendingInput.Kind == "workspace" {
					approvals++
					result, runErr = runner.ResolveExecutionInput(result.ID, result.Actor, result.PendingInput.ID, resource)
					if runErr == nil {
						result, runErr = runner.Continue(ctx, result.ID, result.Actor, "start", "", false)
					}
				}
				cancel()
				if result == nil {
					t.Fatal(runErr)
				}
				t.Logf("model=%s attached=%t case=%s status=%s decisions=%d access_requests=%d elapsed_ms=%d answer=%q error=%q", model, attached, item.name, result.Status, calls-prior, approvals, time.Since(start).Milliseconds(), result.Result, result.Error)
				if result.PendingApproval != nil {
					t.Logf("isolated_pending_approval tool=%s arguments=%s", result.PendingApproval.Tool, result.PendingApproval.ArgumentsJSON)
				}
				for _, step := range result.Steps {
					if step.ToolName != "" {
						t.Logf("tool=%s type=%s args=%s result=%s", step.ToolName, step.Type, step.ToolArgs, step.ToolResult)
					} else if step.Type == "rejected" || step.Type == "completion_rejected" {
						t.Logf("isolated rejection type=%s feedback=%s", step.Type, step.Content)
					}
				}
				if runErr != nil || result.Status != agents.TaskCompleted {
					t.Fatalf("workflow failed: %v", runErr)
				}
				if item.name == "list" && !strings.Contains(result.Result, "source.txt") {
					t.Fatal("listing omitted observed fixture")
				}
				if item.name == "read" && !strings.Contains(result.Result, strings.TrimSpace(original)) {
					t.Fatal("read answer did not preserve source text")
				}
				if item.name == "edit" {
					expectedFiles["source.txt"] = wanted
					actual, _ := os.ReadFile(filepath.Join(root, "source.txt"))
					if string(actual) != wanted {
						t.Fatal("independent read-back mismatch")
					}
					if !strings.Contains(result.Result, strings.TrimSpace(wanted)) {
						t.Fatal("answer did not report the requested verified replacement text")
					}
				}
				if item.name == "test" {
					verified := false
					var observedOutput string
					for _, step := range result.Steps {
						if strings.HasPrefix(step.ToolName, "workspace_process_") {
							var receipt struct {
								Receipt struct {
									Result execution.ProcessResult `json:"result"`
								} `json:"receipt"`
							}
							if json.Unmarshal([]byte(step.ToolResult), &receipt) == nil && receipt.Receipt.Result.ExitCode != nil && *receipt.Receipt.Result.ExitCode == 0 {
								verified = true
								if len(receipt.Receipt.Result.Output) != 0 {
									observedOutput = string(receipt.Receipt.Result.Output)
								}
							}
						}
					}
					if !verified {
						t.Fatal("no independent successful process receipt")
					}
					if !reportsObservedOutput(result.Result, observedOutput) {
						t.Fatal("answer did not report actual observed command output")
					}
				}
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != len(expectedFiles) {
					t.Fatal("unexpected workspace entries", err)
				}
				for name, expected := range expectedFiles {
					actual, err := os.ReadFile(filepath.Join(root, name))
					if err != nil || string(actual) != expected {
						t.Fatal("unexpected file change", name, err)
					}
				}
				cfg = result.Config
				cfg.ConversationContext = append(cfg.ConversationContext, api.ChatMessage{Role: "user", Content: item.prompt}, api.ChatMessage{Role: "assistant", Content: result.Result})
			}
			if approvals != map[bool]int{true: 0, false: 1}[attached] {
				t.Fatal("unexpected access count", approvals)
			}
		})
	}
}

// A quoted JSON string is a lossless representation, not a different output.
// Decode only syntactically complete quoted strings, never arbitrary backslash
// text. This is an evaluator, not a transformation of the user's answer.
func reportsObservedOutput(answer, output string) bool {
	if strings.TrimSpace(output) == "" {
		return false
	}
	normalized := strings.Join(strings.Fields(output), " ")
	var plain strings.Builder
	for offset := 0; offset < len(answer); {
		next := strings.IndexByte(answer[offset:], '"')
		if next < 0 {
			plain.WriteString(answer[offset:])
			break
		}
		plain.WriteString(answer[offset : offset+next])
		offset += next
		decoder := json.NewDecoder(strings.NewReader(answer[offset:]))
		var quoted string
		if decoder.Decode(&quoted) == nil {
			if strings.Join(strings.Fields(quoted), " ") == normalized {
				return true
			}
			offset += int(decoder.InputOffset())
		} else {
			plain.WriteByte('"')
			offset++
		}
	}
	return strings.Contains(strings.Join(strings.Fields(plain.String()), " "), normalized)
}
