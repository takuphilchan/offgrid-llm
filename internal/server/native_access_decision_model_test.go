package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Decision-only qualification: never dispatch the model's proposed call. Compare
// the same effective catalog and conversation through both response adapters.
func TestNativeAccessDecisionInstalledModel(t *testing.T) {
	engine, model := installedConversationEngine(t)
	for _, history := range []bool{false, true} {
		t.Run(fmt.Sprintf("history-%t", history), func(t *testing.T) {
			f := newConversationFixture(t)
			ref, _ := approvedExecutionPermissionsFixture(t, f.s, execution.Permissions{Read: true})
			resource, err := f.s.executionHosts.Resolve(context.Background(), "local-admin", f.s.workspaceID, ref)
			if err != nil {
				t.Fatal(err)
			}
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst, cfg.ExecutionRequestVersion = true, 1
			cfg.ContextWindow, cfg.MaxTokens, cfg.MaxIterations = 8192, 512, 1
			cfg.Execution, cfg.ExecutionWorkspace = &ref, &resource
			cfg.SystemPrompt = conversationTaskProtocol + "\n" + taskFirstPrompt + serviceFilePrompt
			cfg.AllowedCapabilities = map[string]capabilities.Descriptor{execution.AccessRequestTool: execution.AccessRequestDescriptor()}
			for _, tool := range execution.Tools() {
				if execution.Permitted(tool.Function.Name, resource.Permissions) {
					d, _ := execution.Descriptor(tool.Function.Name)
					cfg.AllowedCapabilities[d.Name] = d
				}
			}
			cfg.Profile = &agents.AgentProfile{Settings: agents.ProfileSettings{Model: model}}
			if history {
				cfg.ConversationContext = []api.ChatMessage{
					{Role: "user", Content: "hello"}, {Role: "assistant", Content: "Hello!"},
					{Role: "user", Content: "List files in this directory."}, {Role: "assistant", Content: "alpha-cafe.txt, reports/"},
					{Role: "user", Content: "Read alpha-cafe.txt and return its complete exact text."}, {Role: "assistant", Content: "Fixture value — café 日本語 12345.\n"},
				}
			}
			called := 0
			runner := agents.NewRunner(f.s.agentManager, &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				called++
				names := []string{}
				found := false
				for _, tool := range tools {
					names = append(names, tool.Function.Name)
					found = found || tool.Function.Name == execution.AccessRequestTool
				}
				if !found {
					t.Fatal("requestable access missing from effective catalog", names)
				}
				t.Logf("effective_tools=%v", names)
				for _, constrained := range []bool{true, false} {
					t.Run(fmt.Sprintf("constrained-%t", constrained), func(t *testing.T) {
						zero, max, no := float32(0), 512, false
						req := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max, SingleStepTools: constrained, ParallelToolCalls: &no}
						acc := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
						started := time.Now()
						err := engine.ChatCompletionStreamRaw(ctx, req, func(raw json.RawMessage) error { _, _, err := acc.add(raw); return err })
						response := acc.response()
						data, _ := json.Marshal(response)
						t.Logf("model=%s elapsed_ms=%d response=%s error=%v", model, time.Since(started).Milliseconds(), data, err)
						if err != nil || response == nil || len(response.Choices) != 1 {
							t.Error("no complete decision")
							return
						}
						calls := response.Choices[0].Message.ToolCalls
						var args struct {
							Access string `json:"access"`
						}
						if len(calls) != 1 || calls[0].Function.Name != execution.AccessRequestTool || json.Unmarshal([]byte(calls[0].Function.Arguments), &args) != nil || args.Access != "edit" {
							t.Error("did not request required edit access")
						}
					})
				}
				return nil, errors.New("decision-only fixture: dispatch intentionally disabled")
			})
			defer runner.Shutdown(context.Background())
			task, err := runner.Create("Create a new result.txt containing exactly the text between these markers, excluding the markers: <content>OffGrid native test — café 日本語 12345</content>. Read it back to verify exact equality. Leave all other files unchanged.", model, "local-admin", cfg)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			result, _ := runner.Continue(ctx, task.ID, task.Actor, "start", "", false)
			if called != 1 || result == nil {
				t.Fatal("missing decision inspection")
			}
			for _, step := range result.Steps {
				if step.Type == "action" {
					t.Fatal("decision-only fixture dispatched an action")
				}
			}
		})
	}
}
