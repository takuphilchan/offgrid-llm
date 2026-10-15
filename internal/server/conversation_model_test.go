package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Frozen, isolated workflow matrix. Only the already installed inference endpoint
// is external; all tools target in-memory apps and an owned local MCP fixture.
// This never controls the user's desktop or qualifies an OS driver.
func TestConversationWorkflowsInstalledModel(t *testing.T) {
	endpoint, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("requires explicitly selected installed loopback model; no downloads or native input")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		t.Fatal("requires literal loopback endpoint")
	}
	engine := inference.NewLlamaHTTPEngine(strings.TrimSuffix(endpoint, "/v1/chat/completions"))
	for _, scenario := range []string{"greeting", "obs", "settings", "mixed", "mcp-native", "document", "recovery"} {
		for repetition := 1; repetition <= 3; repetition++ {
			t.Run(fmt.Sprintf("%s/%d", scenario, repetition), func(t *testing.T) {
				calls, totalTokens, peakTokens := 0, 0, 0
				modelTime, countTime := time.Duration(0), time.Duration(0)
				makeRequest := func(task *agents.Task, messages []api.ChatMessage, tools []api.Tool) *api.ChatCompletionRequest {
					zero, max := float32(0), 512
					r := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max, StreamOptions: &api.StreamOptions{IncludeUsage: true}}
					configureComputerRequest(r, task)
					return r
				}
				counter := func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
					start := time.Now()
					n, e := engine.CountPromptTokens(ctx, makeRequest(task, messages, tools))
					if e != nil {
						t.Logf("fixture tokenizer failure: %v", e)
					}
					countTime += time.Since(start)
					return n, e
				}
				caller := func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
					r := makeRequest(task, messages, tools)
					if len(task.Steps) > 0 && calls < 9 {
						names := make([]string, 0, len(tools))
						for _, tool := range tools {
							names = append(names, tool.Function.Name)
						}
						t.Logf("fixture available tools: %v", names)
						if len(messages) > 0 {
							t.Logf("fixture last context: %s", messages[len(messages)-1].StringContent())
						}
					}
					n, e := counter(ctx, task, messages, tools)
					if e != nil {
						return nil, e
					}
					if n+1024 > 8192 {
						return nil, fmt.Errorf("context overflow %d", n)
					}
					start := time.Now()
					a := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
					e = engine.ChatCompletionStreamRaw(ctx, r, func(data json.RawMessage) error { _, _, e := a.add(data); return e })
					modelTime += time.Since(start)
					calls++
					if e != nil {
						return nil, e
					}
					result := a.response()
					if result.Usage.PromptTokens <= 0 {
						return nil, fmt.Errorf("missing measured context usage")
					}
					totalTokens += result.Usage.PromptTokens
					peakTokens = max(peakTokens, result.Usage.PromptTokens)
					for _, choice := range result.Choices {
						for _, call := range choice.Message.ToolCalls {
							t.Logf("fixture proposal: %s %s", call.Function.Name, call.Function.Arguments)
						}
					}
					return result, nil
				}
				defer func() {
					t.Logf("OFFGRID_MODEL_METRICS calls=%d prompt_total=%d prompt_peak=%d model_ms=%d tokenize_ms=%d queue_ms=unmeasured ui_ms=not_applicable", calls, totalTokens, peakTokens, modelTime.Milliseconds(), countTime.Milliseconds())
				}()
				fault := "none"
				if scenario == "recovery" {
					fault = "lost-ack"
				}
				runConversationQualificationFixture(t, fault, caller, true, scenario, counter)
			})
		}
	}
}
