package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestContextCounterOverridesEstimateAndPreservesInstructions(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, NewToolRegistry(), nil)
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.ContextWindow, cfg.MaxTokens = 8192, 1024
	task, _ := r.Create(strings.Repeat("Quoted \\\"text\\\" ", 2000), "any-model", "alice", cfg)
	original := task.Checkpoint.Messages[1].StringContent()
	r.CountContext = func(ctx context.Context, got *Task, messages []api.ChatMessage, tools []api.Tool) (int, error) {
		if got.Model != "any-model" || messages[1].StringContent() != original {
			t.Fatal("count changed authority or instructions")
		}
		return 5000, nil // deterministic tokenizer double, not an accuracy claim
	}
	messages, err := r.prepareContext(task, nil)
	if err != nil || messages[1].StringContent() != original || task.Context.EstimatedTokens != 5000 || len(task.ContextArchive) != 0 {
		t.Fatalf("%+v %v", task.Context, err)
	}
}

func TestLongConversationAnswerArchivesWithoutLosingConstraints(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, NewToolRegistry(), nil)
	defer r.Shutdown(context.Background())
	cfg := taskConfig()
	cfg.ContextWindow, cfg.MaxTokens = 8192, 1024
	answer := strings.Repeat("previous answer ", 2500) + "final citation"
	cfg.ConversationContext = []api.ChatMessage{{Role: "user", Content: "Do not change files."}, {Role: "assistant", Content: answer}}
	task, err := r.Create("Explain the result", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = m.updateTask(task.ID, func(saved *Task) error { saved.Status = TaskRunning; return nil })
	if err != nil {
		t.Fatal(err)
	}
	// This test double proves measured accounting decides compaction, without
	// asserting tokenizer accuracy for any installed model.
	r.CountContext = func(_ context.Context, _ *Task, messages []api.ChatMessage, _ []api.Tool) (int, error) {
		for _, message := range messages {
			if message.StringContent() == answer {
				return 12000, nil
			}
		}
		return 2000, nil
	}
	messages, err := r.prepareContext(task, nil)
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	for _, message := range messages {
		content.WriteString(message.StringContent())
	}
	if !strings.Contains(content.String(), "Do not change files.") || !strings.Contains(content.String(), "Explain the result") || len(task.ContextArchive) != 1 || task.ContextArchive[0].Messages[0].StringContent() != answer {
		t.Fatal("lost instructions, current request, or exact archived answer")
	}
}

func TestContextCounterFailuresNeverPermitDispatch(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		err   error
	}{
		{"real_overflow", 9000, nil}, {"invalid", 0, nil}, {"cancelled", 0, context.Canceled}, {"runtime", 0, errors.New("PRIVATE runtime error")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			r := NewRunner(m, NewToolRegistry(), func(context.Context, *Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
				t.Fatal("generation after failed budget check")
				return nil, nil
			})
			defer r.Shutdown(context.Background())
			r.CountContext = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (int, error) { return tc.count, tc.err }
			cfg := taskConfig()
			cfg.ContextWindow = 8192
			task, _ := r.Create("short request", "model", "alice", cfg)
			result, err := r.Continue(context.Background(), task.ID, "alice", "start", "", false)
			if err != nil || result.Status != TaskFailed || !strings.Contains(result.Error, "No further") && !strings.Contains(result.Error, "no further") || strings.Contains(result.Error, "PRIVATE") {
				t.Fatalf("status=%s error=%s: %v", result.Status, result.Error, err)
			}
		})
	}
}

func TestContextCounterUnavailableRetainsConservativeGuard(t *testing.T) {
	m := NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	r := NewRunner(m, NewToolRegistry(), nil)
	defer r.Shutdown(context.Background())
	r.CountContext = func(context.Context, *Task, []api.ChatMessage, []api.Tool) (int, error) {
		return 0, ErrContextCountUnavailable
	}
	cfg := taskConfig()
	cfg.ContextWindow = 8192
	task, _ := r.Create("short request", "model", "alice", cfg)
	if _, err := r.prepareContext(task, nil); err != nil {
		t.Fatal(err)
	}
	task.Checkpoint.Messages[1].Content = strings.Repeat("genuinely oversized", 3000)
	if _, err := r.prepareContext(task, nil); err == nil {
		t.Fatal("fallback bypassed budget")
	}
}

func TestModelContextBytesCountsContentNotTransportEscaping(t *testing.T) {
	plain := []api.ChatMessage{{Role: "tool", Name: "fixture", ToolCallID: "call", Content: strings.Repeat("a", 500)}}
	escaped := append([]api.ChatMessage(nil), plain...)
	escaped[0].Content = strings.Repeat("\\\"", 250)
	if modelContextBytes(plain, nil) != modelContextBytes(escaped, nil) {
		t.Fatal("counted outer JSON escaping")
	}
	escaped[0].Content = strings.Repeat("旅行", 100)
	if modelContextBytes(escaped, nil)-modelContextBytes(plain, nil) != 100 {
		t.Fatal("lost UTF-8 byte accounting")
	}
}

func TestContextArchiveAliasesRetainDigestCompatibility(t *testing.T) {
	task := &Task{ContextArchive: []ArchivedContext{{ID: strings.Repeat("a", 64), Messages: []api.ChatMessage{{Role: "tool", Content: "original evidence"}}}}}
	for _, ref := range []string{"archive:1", strings.Repeat("a", 64)} {
		page, err := contextPage(task, ref, 0)
		if err != nil || !strings.Contains(page, "original evidence") {
			t.Fatal(page, err)
		}
	}
	for _, ref := range []string{"archive:0", "archive:-1", "archive:2", "archive:999999999999999999999", "archive:other"} {
		if _, err := contextPage(task, ref, 0); err == nil {
			t.Fatal("invalid alias accepted", ref)
		}
	}
}
