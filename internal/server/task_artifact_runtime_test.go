package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// The service API takes ordinary tools, not the private response_format sent to
// llama.cpp. Exercise the deployed adapter without dispatching any returned call.
func TestArtifactDeployedServiceSchema(t *testing.T) {
	base := os.Getenv("OFFGRID_TEST_ARTIFACT_SERVICE")
	if base == "" {
		t.Skip("requires explicit idle local service; no tools dispatched")
	}
	for _, wanted := range []taskArtifactRequest{
		{"fixture.txt", "text", "OffGrid test — café 日本語 12345."},
		{"fixture.md", "markdown", "# Synthetic draft"},
		{"fixture.json", "json", `{"fixture":true}`},
		{"fixture.csv", "csv", "a,b\n1,2"},
	} {
		t.Run(wanted.Format, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			args, _ := json.Marshal(wanted)
			parallel, zero, max := false, float32(0), 256
			request := &api.ChatCompletionRequest{Model: os.Getenv("OFFGRID_TEST_RUNTIME_MODEL"), ParallelToolCalls: &parallel, Temperature: &zero, MaxTokens: &max, Tools: []api.Tool{taskArtifactTool()}, Messages: []api.ChatMessage{{Role: "user", Content: "Synthetic adapter test. Return exactly one task_save_artifact call with these exact arguments: " + string(args)}}}
			body, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/v1/chat/completions", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("service HTTP %d", response.StatusCode)
			}
			var result api.ChatCompletionResponse
			if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			if len(result.Choices) != 1 || len(result.Choices[0].Message.ToolCalls) != 1 {
				t.Fatal("expected one complete tool decision")
			}
			call := result.Choices[0].Message.ToolCalls[0]
			got, _, err := parseTaskArtifact(json.RawMessage(call.Function.Arguments))
			if err != nil || call.Function.Name != taskArtifactToolName || got != wanted {
				t.Fatalf("wrong synthetic decision: %+v, %v", got, err)
			}
		})
	}
}

func TestArtifactSchemaPatternsContainNoLiteralControls(t *testing.T) {
	for _, branch := range taskArtifactTool().Function.Parameters["oneOf"].([]any) {
		name := branch.(map[string]any)["properties"].(map[string]any)["name"].(map[string]any)
		if strings.IndexFunc(name["pattern"].(string), unicode.IsControl) >= 0 {
			t.Fatal("literal control characters break the runtime's schema-to-grammar conversion")
		}
	}
}

func TestArtifactParserStillRejectsEveryControlCharacter(t *testing.T) {
	for r := rune(0); r <= 0x9f; r++ {
		if !unicode.IsControl(r) {
			continue
		}
		raw, _ := json.Marshal(taskArtifactRequest{"fixture" + string(r) + ".txt", "text", "draft"})
		if _, _, err := parseTaskArtifact(raw); err == nil {
			t.Fatalf("accepted control U+%04X", r)
		}
	}
}

// Explicit opt-in against an idle local runtime. This uses the production tool
// schema and response adapter but never dispatches tools or creates artifacts.
func TestArtifactInstalledRuntimeSchema(t *testing.T) {
	base := os.Getenv("OFFGRID_TEST_SINGLE_STEP_RUNTIME")
	if base == "" {
		t.Skip("requires explicit idle local runtime; no tools dispatched")
	}
	for _, wanted := range []taskArtifactRequest{
		{"fixture.txt", "text", "OffGrid test — café 日本語 12345."},
		{"fixture.md", "markdown", "# Synthetic draft"},
		{"fixture.json", "json", `{"fixture":true}`},
		{"fixture.csv", "csv", "a,b\n1,2"},
	} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%v", wanted.Format, stream), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				args, _ := json.Marshal(wanted)
				parallel, zero, max := false, float32(0), 256
				req := &api.ChatCompletionRequest{
					Model: os.Getenv("OFFGRID_TEST_RUNTIME_MODEL"), ParallelToolCalls: &parallel,
					Temperature: &zero, MaxTokens: &max, Tools: []api.Tool{taskArtifactTool()},
					Messages: []api.ChatMessage{{Role: "user", Content: "Synthetic adapter test. Return exactly one task_save_artifact call with these exact arguments: " + string(args)}},
				}
				engine := inference.NewLlamaHTTPEngine(base)
				var result *api.ChatCompletionResponse
				var err error
				if stream {
					result = &api.ChatCompletionResponse{}
					err = engine.ChatCompletionStreamRaw(ctx, req, func(data json.RawMessage) error {
						var chunk api.ChatCompletionChunk
						if err := json.Unmarshal(data, &chunk); err != nil {
							return err
						}
						for _, choice := range chunk.Choices {
							if choice.FinishReason == nil {
								return fmt.Errorf("partial decision emitted")
							}
							result.Choices = append(result.Choices, api.ChatCompletionChoice{Message: choice.Delta, FinishReason: *choice.FinishReason})
						}
						return nil
					})
				} else {
					result, err = engine.ChatCompletion(ctx, req)
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Choices) != 1 || len(result.Choices[0].Message.ToolCalls) != 1 {
					t.Fatal("expected one complete tool decision")
				}
				call := result.Choices[0].Message.ToolCalls[0]
				got, _, err := parseTaskArtifact(json.RawMessage(call.Function.Arguments))
				if err != nil || call.Function.Name != taskArtifactToolName || got != wanted {
					t.Fatalf("wrong synthetic decision: %+v, %v", got, err)
				}
			})
		}
	}
	t.Run("conversation_with_artifact_available", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		parallel, zero, max := false, float32(0), 64
		req := &api.ChatCompletionRequest{Model: os.Getenv("OFFGRID_TEST_RUNTIME_MODEL"), ParallelToolCalls: &parallel, Temperature: &zero, MaxTokens: &max, Tools: []api.Tool{taskArtifactTool()}, Messages: []api.ChatMessage{{Role: "user", Content: "Say hello. Do not create an artifact or call any tools."}}}
		result, err := inference.NewLlamaHTTPEngine(base).ChatCompletion(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Choices) != 1 || len(result.Choices[0].Message.ToolCalls) != 0 || strings.TrimSpace(result.Choices[0].Message.StringContent()) == "" {
			t.Fatal("conversation did not abstain from tools")
		}
	})
}
