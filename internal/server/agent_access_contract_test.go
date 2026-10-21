package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestAccessSchemaRejectsInvalidModeCombinations(t *testing.T) {
	for _, tc := range []struct {
		args  string
		valid bool
	}{
		{`{"mode":"browser","target":"Microsoft Edge","url":""}`, false},
		{`{"mode":"browser","target":"","url":"https://www.google.com"}`, false},
		{`{"mode":"app","target":"Microsoft Edge","url":"https://example.com"}`, false},
		{`{"mode":"app","target":"Microsoft Edge","url":""}`, true},
		{`{"mode":"browser","target":"Public documentation","url":"https://example.com/docs"}`, true},
		{`{"mode":"browser","target":"Public documentation","url":"http://example.com"}`, false},
	} {
		t.Run(tc.args, func(t *testing.T) {
			response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "test", Type: "function", Function: api.FunctionCall{Name: computerAccessToolName, Arguments: tc.args}}}}}}}
			err := inference.ValidateToolDecision(&api.ChatCompletionRequest{Tools: []api.Tool{computerAccessTool()}}, response)
			if (err == nil) != tc.valid {
				t.Fatalf("schema validity=%v: %v", tc.valid, err)
			}
			var input *agents.InputRequired
			if errors.As(computerAccessRequest(json.RawMessage(tc.args)), &input) != tc.valid {
				t.Fatal("schema/validator disagreement")
			}
		})
	}
}

func TestAccessFeedbackIdentifiesFieldWithoutEchoingPrivateData(t *testing.T) {
	for _, tc := range []struct{ args, want string }{
		{`{"mode":"browser","target":"","url":"https://example.com/PRIVATE"}`, "target is empty"},
		{`{"mode":"app","target":"Editor","url":"https://example.com/PRIVATE"}`, "url must be empty"},
		{`{"mode":"app","target":"Editor"}`, "three string fields"},
		{`{"mode":"app","target":"PRIVATE","target":"Editor","url":""}`, "three string fields"},
		{`{"mode":"browser","target":"Site","url":"https://user:PRIVATE@example.com"}`, "without embedded credentials"},
	} {
		err := computerAccessRequest(json.RawMessage(tc.args))
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatalf("incorrect feedback: %v", err)
		}
	}
}

// Uses an explicitly selected idle local model, real runner/adapter/schema and
// disposable task persistence. Stops at consent; never dispatches host actions.
func TestAccessInstalledRuntimePreservesRequestedEnvironment(t *testing.T) {
	base := os.Getenv("OFFGRID_TEST_SINGLE_STEP_RUNTIME")
	if base == "" {
		t.Skip("requires explicit idle local runtime; no host dispatch")
	}
	for _, tc := range []struct{ prompt, mode, target string }{
		{"Open Microsoft Edge and search for tourism in Zimbabwe. Find three useful public sources and summarize their findings with links. Do not sign in, submit forms, or change profiles. If profile selection is required, ask me.", "app", "Microsoft Edge"},
		{"Open Google Chrome and search for tourism in Zimbabwe. Use Chrome, not a separate browser.", "app", "Google Chrome"},
		{"Open Notepad. In a new blank document type café 日本語. Do not save or change existing documents.", "app", "Notepad"},
		{"Use a separate managed browser to read https://example.com and summarize it.", "browser", ""},
	} {
		t.Run(tc.mode+tc.target, func(t *testing.T) {
			s := taskFirstServer(t)
			tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
			calls := 0
			r := agents.NewRunner(s.agentManager, tools, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, offered []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				parallel, zero, max := false, float32(0), 512
				return inference.NewLlamaHTTPEngine(base).ChatCompletion(ctx, &api.ChatCompletionRequest{Model: os.Getenv("OFFGRID_TEST_RUNTIME_MODEL"), Messages: messages, Tools: offered, ParallelToolCalls: &parallel, Temperature: &zero, MaxTokens: &max})
			})
			t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst = true
			cfg.SystemPrompt = taskFirstPrompt
			cfg.ContextWindow = 8192
			cfg.MaxIterations = 10
			task, err := r.Create(tc.prompt, os.Getenv("OFFGRID_TEST_RUNTIME_MODEL"), "local-admin", cfg)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			got, err := r.Continue(ctx, task.ID, task.Actor, "start", "", false)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != agents.TaskInput || got.PendingInput == nil {
				t.Fatalf("did not reach consent: status=%s error=%s calls=%d", got.Status, got.Error, calls)
			}
			input := got.PendingInput
			if input.Mode != tc.mode || (tc.target != "" && input.Target != tc.target) || (tc.mode == "app" && input.URL != "") {
				t.Fatalf("wrong environment: %+v", input)
			}
			if got.Config.ComputerSession != "" || got.Checkpoint.ExecutingCall != "" {
				t.Fatal("test acquired authority or dispatched")
			}
			t.Logf("consent reached in %d model calls; mode=%s target=%s", calls, input.Mode, input.Target)
		})
	}
}

// Replay only rejected pre-effect proposals into disposable persistence. This
// exercises explicit resume with the real model, not the user's saved job.
func TestAccessInstalledRuntimeRepairsPersistedInvalidRequest(t *testing.T) {
	base := os.Getenv("OFFGRID_TEST_SINGLE_STEP_RUNTIME")
	if base == "" {
		t.Skip("requires explicit idle local runtime; no host dispatch")
	}
	for _, invalid := range []string{
		`{"mode":"browser","target":"Microsoft Edge","url":""}`,
		`{"mode":"browser","target":"","url":"https://www.google.com"}`,
	} {
		t.Run(invalid, func(t *testing.T) {
			s := taskFirstServer(t)
			dir := t.TempDir()
			s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, dir)
			tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
			r := agents.NewRunner(s.agentManager, tools, func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: computerAccessToolName, Arguments: invalid}}}}}}}, nil
			})
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst, cfg.SystemPrompt, cfg.ContextWindow, cfg.MaxIterations = true, taskFirstPrompt, 8192, 12
			task, err := r.Create("Open Microsoft Edge and search for tourism in Zimbabwe. Find three useful public sources and summarize their findings with links. Do not sign in, submit forms, or change profiles. If profile selection is required, ask me.", os.Getenv("OFFGRID_TEST_RUNTIME_MODEL"), "local-admin", cfg)
			if err != nil {
				t.Fatal(err)
			}
			task, err = r.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
			if err != nil || task.Status != agents.TaskInterrupted || task.ErrorCode != "tool_proposal_repair_limit" {
				t.Fatalf("missing bounded rejection: %v %+v", err, task)
			}
			if err = r.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, dir)
			calls := 0
			r = agents.NewRunner(s.agentManager, tools, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, offered []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				parallel, zero, max := false, float32(0), 512
				return inference.NewLlamaHTTPEngine(base).ChatCompletion(ctx, &api.ChatCompletionRequest{Model: task.Model, Messages: messages, Tools: offered, ParallelToolCalls: &parallel, Temperature: &zero, MaxTokens: &max})
			})
			t.Cleanup(func() { _ = r.Shutdown(context.Background()) })
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			got, err := r.Continue(ctx, task.ID, task.Actor, "resume", "", false)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != task.ID || got.Status != agents.TaskInput || got.PendingInput == nil || got.PendingInput.Mode != "app" || got.PendingInput.Target != "Microsoft Edge" || got.PendingInput.URL != "" {
				t.Fatalf("explicit resume did not repair access: status=%s error=%s input=%+v calls=%d", got.Status, got.Error, got.PendingInput, calls)
			}
			if got.Config.ComputerSession != "" || got.Checkpoint.ExecutingCall != "" {
				t.Fatal("resume dispatched or acquired authority")
			}
			for _, step := range got.Steps {
				if step.Type != "rejected" && step.Type != "completion_rejected" {
					t.Fatalf("unexpected effect during access repair: %s", step.Type)
				}
			}
			t.Logf("same persisted job repaired in %d model calls; waiting for local consent", calls)
		})
	}
}
