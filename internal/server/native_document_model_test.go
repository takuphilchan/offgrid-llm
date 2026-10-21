package server

import (
	"context"
	"encoding/json"
	"errors"
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

// Optional installed-model test: unlike declaration-only coverage this continues
// through actual executor input against an in-memory fixture and final read-back.
// It cannot launch apps, contact tools outside the fixture, or download weights.
func TestDocumentWorkflowInstalledModel(t *testing.T) {
	endpoint, model := os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("requires explicit installed local model endpoint; no native input")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || net.ParseIP(parsed.Hostname()) == nil || !net.ParseIP(parsed.Hostname()).IsLoopback() {
		t.Fatal("installed model test requires a literal loopback URL without credentials, query or fragment")
	}
	base, _ := url.Parse(strings.TrimSuffix(endpoint, "/v1/chat/completions"))
	// Exercise the production engine's direct runtime transport for counting and
	// generation. A tracing reverse proxy introduces a separate connection pool
	// and can turn transport failures into HTTP 502, changing the tested path.
	engine := inference.NewLlamaHTTPEngine(base.String())
	t.Logf("synthetic prompt counts and generation use the installed runtime directly: %s; no diagnostic proxy or generation retry", base.String())
	for repetition := 1; repetition <= 3; repetition++ {
		passed := t.Run(fmt.Sprintf("repeat-%d", repetition), func(t *testing.T) {
			calls, tokens := 0, 0
			modelDuration := time.Duration(0)
			runConversationQualificationFixture(t, "none", func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
				zero, max := float32(0), 512
				request := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max}
				request.StreamOptions = &api.StreamOptions{IncludeUsage: true}
				configureComputerRequest(request, task)
				if calls == 0 {
					for _, message := range messages {
						if message.Role == "user" {
							t.Logf("fixture user: %s", message.StringContent())
						}
					}
				}
				started := time.Now()
				count, err := engine.CountPromptTokens(ctx, request)
				countDuration := time.Since(started)
				if err != nil || count+max+512 > 8192 {
					return nil, fmt.Errorf("single-step prompt count unavailable or exceeds 8192")
				}
				accumulator := agentStreamAccumulator{calls: make(map[int]*api.ToolCall)}
				err = engine.ChatCompletionStreamRaw(ctx, request, func(data json.RawMessage) error { _, _, err := accumulator.add(data); return err })
				modelDuration += time.Since(started)
				calls++
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						return nil, fmt.Errorf("local model deadline exceeded; no successful inference measurement")
					}
					return nil, fmt.Errorf("synthetic local model request failed: %w", err)
				}
				result := accumulator.response()
				if result.Usage.PromptTokens <= 0 {
					return nil, fmt.Errorf("actual prompt token measurement unavailable")
				}
				if result.Usage.PromptTokens+max+512 > 8192 {
					return nil, fmt.Errorf("actual model context exceeds 8192 tokens")
				}
				tokens += result.Usage.PromptTokens
				t.Logf("step %d: prompt=%d output=%d tools=%d result_calls=%d finish=%s count_ms=%d generation_ms=%d", calls, count, result.Usage.CompletionTokens, len(tools), len(result.Choices[0].Message.ToolCalls), result.Choices[0].FinishReason, countDuration.Milliseconds(), (time.Since(started) - countDuration).Milliseconds())
				for _, call := range result.Choices[0].Message.ToolCalls {
					t.Logf("fixture proposal: %s %s", call.Function.Name, call.Function.Arguments)
				}
				return result, nil
			}, true, "document-action", func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
				zero, max := float32(0), 512
				request := &api.ChatCompletionRequest{Messages: messages, Tools: tools, ToolChoice: "auto", Temperature: &zero, MaxTokens: &max}
				configureComputerRequest(request, task)
				return engine.CountPromptTokens(ctx, request)
			})
			t.Logf("installed-model synthetic workflow: calls=%d prompt_tokens=%d model_ms=%.3f", calls, tokens, float64(modelDuration)/float64(time.Millisecond))
		})
		if !passed {
			break
		} // A failed repetition is not recovered by more inference.
	}
}
