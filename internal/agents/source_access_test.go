package agents

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestKnowledgeRevocationStopsInferenceDispatchAndCompletion(t *testing.T) {
	for _, boundary := range []string{"before-inference", "after-answer", "before-tool", "authorization", "stream"} {
		t.Run(boundary, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			tools := &runTestTools{}
			allowed, calls := boundary != "before-inference", 0
			if boundary == "authorization" {
				tools.authorize = func(string, json.RawMessage, ToolExecution) error { allowed = false; return nil }
			}
			r := NewRunner(m, tools, func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				calls++
				if boundary != "authorization" {
					allowed = false
				}
				message := api.ChatMessage{Role: "assistant", Content: "private answer"}
				if boundary == "before-tool" || boundary == "authorization" {
					message.Content = ""
					message.ToolCalls = []api.ToolCall{{ID: "call", Type: "function", Function: api.FunctionCall{Name: "write_file", Arguments: `{"path":"test","content":"private"}`}}}
				}
				return runAnswer(message), nil
			})
			r.SourceAccess = func(*Task) bool { return allowed }
			if boundary == "stream" {
				r.StreamCaller = func(_ context.Context, _ *Task, _ []api.ChatMessage, _ []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
					calls++
					allowed = false
					return nil, emit("generating", "private answer")
				}
			}
			cfg := DefaultAgentConfig()
			cfg.KnowledgeDependencies = []string{"private"}
			task, err := r.Create("question", "model", "owner", cfg)
			if err != nil {
				t.Fatal(err)
			}
			result, err := r.Continue(context.Background(), task.ID, "owner", "start", "", false)
			if err != nil || result.Status != TaskFailed || result.ErrorCode != "knowledge_unavailable" || result.Result != "" {
				t.Fatal("revocation not terminal", result, err)
			}
			if tools.calls.Load() != 0 || boundary == "before-inference" && calls != 0 {
				t.Fatal("work dispatched after revocation")
			}
			if result.Progress != nil && result.Progress.Preview != "" {
				t.Fatal("revoked preview persisted")
			}
			if saved, ok := m.GetTask(task.ID); !ok || len(saved.Config.KnowledgeDependencies) != 1 {
				t.Fatal("dependency lost")
			}
		})
	}
}
