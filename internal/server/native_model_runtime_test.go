//go:build !llama

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Opt in only against an installed test model. All observations are synthetic;
// returned calls are inspected, NEVER dispatched to a companion or tool registry.
func TestNativeModelContextRuntimeSmoke(t *testing.T) {
	countURL, completionURL, model := os.Getenv("OFFGRID_TEST_TOKEN_COUNT_URL"), os.Getenv("OFFGRID_TEST_COMPLETION_URL"), os.Getenv("OFFGRID_TEST_CONTEXT_MODEL")
	if countURL == "" || completionURL == "" || model == "" {
		t.Skip("requires explicit installed-runtime URLs/model; no download or desktop control")
	}
	for _, tc := range []struct{ app, query string }{{"Microsoft Edge", "tourism"}, {"LibreOffice Writer", "orchard notes"}} {
		t.Run(tc.app, func(t *testing.T) {
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst, cfg.ComputerScope, cfg.ComputerDriver = true, "desktop", "windows-uia"
			configureDesktopTask(&cfg)
			view := desktopContextFixture(computer.TargetIdentity{ID: "fixture-target", OSSession: "fixture-os", ProcessGeneration: "fixture-process", Surface: "fixture-window", Driver: "windows-uia"}, tc.app)
			raw, _ := json.Marshal(struct {
				computer.NativeView
				Step int `json:"offgrid_step_id"`
			}{view, 1})
			task := &agents.Task{Config: cfg, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(raw)}}}
			projection := &browserRunTools{}
			messages := []api.ChatMessage{{Role: "system", Content: cfg.SystemPrompt}, {Role: "user", Content: "In " + tc.app + ", enter \"" + tc.query + "\" in the observed search field. Do not submit it yet."}, {Role: "assistant", ToolCalls: []api.ToolCall{{ID: "fixture-observe", Type: "function", Function: api.FunctionCall{Name: "computer_observe", Arguments: `{}`}}}}, {Role: "tool", ToolCallID: "fixture-observe", Name: "computer_observe", Content: string(raw)}}
			messages = projection.ProjectModelContext(task, messages)
			zero, max := float32(0), 256
			request := &api.ChatCompletionRequest{Model: model, Messages: messages, Tools: append(nativeComputerTools(), desktopComputerTools()...), ToolChoice: "auto", Temperature: &zero, MaxTokens: &max}
			configureComputerRequest(request, &agents.Task{Config: cfg})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			start := time.Now()
			count, err := inference.NewLlamaHTTPEngine(countURL).CountPromptTokens(ctx, request)
			if err != nil || count+max+512 > 8192 {
				t.Fatalf("count=%d err=%v", count, err)
			}
			countElapsed := time.Since(start)
			body, _ := json.Marshal(request)
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, completionURL, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != 200 {
				t.Fatalf("completion HTTP %d", response.StatusCode)
			}
			var result api.ChatCompletionResponse
			if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if len(result.Choices) != 1 || len(result.Choices[0].Message.ToolCalls) != 1 {
				t.Fatal("model did not return one structured call")
			}
			call := result.Choices[0].Message.ToolCalls[0].Function
			call.Arguments = projection.ResolveModelArguments(task, call.Name, call.Arguments)
			var args struct {
				Observation string `json:"observation_id"`
				Element     string `json:"element"`
				Text        string `json:"text"`
			}
			if json.Unmarshal([]byte(call.Arguments), &args) != nil || call.Name != "computer_replace_text" || args.Observation != view.Observation.ID || args.Element != "search-field" || args.Text != tc.query {
				t.Fatalf("wrong next operation: %s %s", call.Name, call.Arguments)
			}
			if result.Usage.PromptTokens <= 0 || result.Usage.PromptTokens > count+128 || count > result.Usage.PromptTokens+128 {
				t.Fatalf("preflight=%d generation=%d", count, result.Usage.PromptTokens)
			}
			if strings.Contains(result.Choices[0].Message.StringContent(), "completed") {
				t.Fatal("premature completion")
			}
			t.Logf("raw=%d projected=%d counted=%d actual=%d count_time=%s total=%s; no tool dispatched", len(raw), len(messages[3].StringContent()), count, result.Usage.PromptTokens, countElapsed, time.Since(start))
		})
	}
}
