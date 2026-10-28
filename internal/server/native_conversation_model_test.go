package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Explicit opt-in: isolated task stores and disposable file hosts only. The
// supplied loopback runtime provides inference; no native desktop is attached.
func installedConversationEngine(t *testing.T) (*inference.LlamaHTTPEngine, string) {
	t.Helper()
	endpoint, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("requires explicit local runtime/model; no downloads")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || net.ParseIP(u.Hostname()) == nil || !net.ParseIP(u.Hostname()).IsLoopback() {
		t.Fatal("literal loopback runtime required")
	}
	// Production llama transport omits the model for a single-model runtime.
	// This isolated bridge restores the explicitly selected ID when testing
	// through the installed service; no provider configuration is changed.
	base, _ := url.Parse(strings.TrimSuffix(endpoint, "/v1/chat/completions"))
	if os.Getenv("OFFGRID_TEST_DIRECT_RUNTIME") == "1" {
		return inference.NewLlamaHTTPEngine(base.String()), model
	}
	proxy := httputil.NewSingleHostReverseProxy(base)
	director := proxy.Director
	proxy.FlushInterval = -1
	proxy.Director = func(r *http.Request) {
		director(r)
		// Preserve nested JSON byte order: the constrained-decoding schema
		// deliberately emits the tool name before its arguments. A generic map
		// round trip here would silently reverse that production contract.
		var payload map[string]json.RawMessage
		if json.NewDecoder(r.Body).Decode(&payload) == nil {
			r.Body.Close()
			payload["model"], _ = json.Marshal(model)
			raw, _ := json.Marshal(payload)
			r.Body = io.NopCloser(bytes.NewReader(raw))
			r.ContentLength = int64(len(raw))
		}
	}
	bridge := httptest.NewServer(proxy)
	t.Cleanup(bridge.Close)
	return inference.NewLlamaHTTPEngine(bridge.URL), model
}

func TestNativeConversationInstalledModel(t *testing.T) {
	engine, model := installedConversationEngine(t)
	adapter := os.Getenv("OFFGRID_TEST_DECISION_ADAPTER")
	if adapter != "" && adapter != "constrained" && adapter != "native" {
		t.Fatal("unknown test-only decision adapter")
	}
	t.Logf("decision_adapter=%s (no production configuration changed)", adapter)
	for _, attached := range []bool{false, true} {
		for _, item := range []struct {
			name, prompt string
			min          int
		}{
			{"greeting", "hello", 2}, {"identity", "who are you?", 10}, {"capabilities", "what can you do? what capabilities do you have", 30}, {"story", "tell me a story", 100}, {"explanation", "Explain how rainbows form in a few sentences.", 80},
		} {
			t.Run(fmt.Sprintf("attached-%t/%s", attached, item.name), func(t *testing.T) {
				f := newConversationFixture(t)
				ref, _ := approvedExecutionFixture(t, f.s)
				f.s.config.EnableExecutionPreview = true
				cfg := agents.DefaultAgentConfig()
				cfg.TaskFirst = true
				cfg.ExecutionRequestVersion = 1
				cfg.MaxIterations = 4
				cfg.MaxTokens = 768
				cfg.ContextWindow = 8192
				cfg.Temperature = 0
				cfg.SystemPrompt = conversationTaskProtocol + "\n" + taskFirstPrompt + serviceFilePrompt
				cfg.AllowedCapabilities = map[string]capabilities.Descriptor{execution.AccessRequestTool: execution.AccessRequestDescriptor()}
				if attached {
					resource, err := f.s.executionHosts.Resolve(context.Background(), "local-admin", f.s.workspaceID, ref)
					if err != nil {
						t.Fatal(err)
					}
					cfg.Execution = &ref
					cfg.ExecutionWorkspace = &resource
					for _, tool := range execution.Tools() {
						if execution.Permitted(tool.Function.Name, resource.Permissions) {
							d, _ := execution.Descriptor(tool.Function.Name)
							cfg.AllowedCapabilities[d.Name] = d
						}
					}
				}
				calls := 0
				applyNativeWorkspaceTestScope(t, &cfg, adapter)
				started := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()
				runner := agents.NewRunner(f.s.agentManager, &browserRunTools{RunTools: f.s.toolRegistry, server: f.s}, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
					calls++
					zero, max := float32(0), 768
					no := false
					request := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max, SingleStepTools: adapter != "native", ParallelToolCalls: &no, StreamOptions: &api.StreamOptions{IncludeUsage: true}}
					acc := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
					err := engine.ChatCompletionStreamRaw(ctx, request, func(raw json.RawMessage) error { _, _, err := acc.add(raw); return err })
					if err != nil {
						return nil, err
					}
					return acc.response(), nil
				})
				defer runner.Shutdown(context.Background())
				task, err := runner.Create(item.prompt, model, "local-admin", cfg)
				if err != nil {
					t.Fatal(err)
				}
				result, runErr := runner.Continue(ctx, task.ID, task.Actor, "start", "", false)
				if result == nil {
					t.Fatal(runErr)
				}
				t.Logf("model=%s case=%s attached=%t status=%s decisions=%d elapsed_ms=%d answer=%q error=%q", model, item.name, attached, result.Status, calls, time.Since(started).Milliseconds(), result.Result, result.Error)
				for _, step := range result.Steps {
					if step.ToolName != "" {
						t.Logf("tool=%s type=%s", step.ToolName, step.Type)
					}
				}
				if runErr != nil || result.Status != agents.TaskCompleted || len([]rune(result.Result)) < item.min {
					t.Fatalf("conversation did not produce a sufficient answer: %v", runErr)
				}
				for _, step := range result.Steps {
					if step.ToolName != "" {
						t.Errorf("unnecessary tool for direct response: %s", step.ToolName)
					}
				}
				if item.name == "identity" && !strings.Contains(result.Result, "OffGrid") {
					t.Error("answer did not identify the neutral OffGrid assistant")
				}
				for _, bad := range []string{"The task needs a file observation", "Correct the read", "root directory of the execution workspace", "last model decision was rejected", "provide more context", "complete answer or a valid tool call"} {
					if strings.Contains(result.Result, bad) {
						t.Errorf("returned operational guidance instead of requested answer")
					}
				}
			})
		}
	}
}
