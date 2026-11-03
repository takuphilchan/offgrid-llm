package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type documentCounterFunc func(context.Context, *api.ChatCompletionRequest) (int, error)

func (f documentCounterFunc) CountPromptTokens(ctx context.Context, r *api.ChatCompletionRequest) (int, error) {
	return f(ctx, r)
}

// Deterministic tokenizer double, not a claim about any model's token density.
func fixtureDocumentTokens(_ context.Context, request *api.ChatCompletionRequest) (int, error) {
	total := 24
	for _, message := range request.Messages {
		total += (len(message.StringContent())+3)/4 + 8
	}
	return total, nil
}

func TestDocumentFollowupsUseMeasuredTokensAndRetainHistory(t *testing.T) {
	manager := sessions.NewSessionManager(t.TempDir())
	scope := manager.WithAccess(sessions.Access{UserID: "alice"})
	saved, _ := scope.Create("paper", "model")
	text := strings.Repeat("The document describes local energy research.\n", 4500) + "The calibration constant is 718.25."
	a, err := scope.CreateAttachment("paper", "paper.txt", "text/plain", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	a, err = scope.PrepareAttachment("paper", a.ID, text, "fixture", "")
	if err != nil {
		t.Fatal(err)
	}
	refs := []sessions.AttachmentReference{{ID: a.ID, Revision: a.Revision}}
	for i := 0; i < 8; i++ {
		turn := &sessions.Turn{Prompt: fmt.Sprintf("Question %d: what is the calibration constant?", i), MaxTokens: 4096}
		if i == 0 {
			turn.Attachments = refs
		}
		base, sources, err := collectAttachmentMessages(scope, "paper", saved, turn)
		if err != nil {
			t.Fatal(err)
		}
		original := append([]api.ChatMessage(nil), base...)
		request := &api.ChatCompletionRequest{Model: "model", Messages: base, MaxTokens: &turn.MaxTokens}
		if err := prepareChatSources(withChatSources(context.Background(), sources, turn), request, 8192, documentCounterFunc(fixtureDocumentTokens)); err != nil {
			t.Fatalf("follow-up %d rejected: %v", i, err)
		}
		count, _ := fixtureDocumentTokens(context.Background(), request)
		if count+turn.MaxTokens+64 > 8192 || turn.MaxTokens != 4096 || !reflect.DeepEqual(base, original) {
			t.Fatal("budget, explicit setting or history changed")
		}
		last := request.Messages[len(request.Messages)-1].StringContent()
		if !strings.Contains(last, "718.25") || strings.Count(last, `"id":"`+a.ID+`"`) != 1 || len(turn.AttachmentSelections) != 1 {
			t.Fatal("follow-up lost, duplicated or failed to record its source")
		}
		if i == 7 {
			if _, err := attachmentMessages(scope, "paper", saved, turn, 8192); err == nil {
				t.Fatal("fixture no longer reproduces the old byte-as-token rejection")
			}
		}
		saved.Messages = append(saved.Messages, sessions.Message{Role: "user", Content: turn.Prompt, Attachments: turn.Attachments}, sessions.Message{Role: "assistant", Content: strings.Repeat("A supported explanation with evidence. ", 28)})
	}
	_, retained, err := scope.ReadAttachment("paper", a.ID)
	if err != nil || retained != text {
		t.Fatal("source was modified")
	}
}

func TestDocumentTokenBudgetBoundaries(t *testing.T) {
	for _, test := range []struct {
		name             string
		counter          documentCounterFunc
		window, response int
		code             string
	}{
		{"response consumes context", documentCounterFunc(fixtureDocumentTokens), 4096, 4096, "attachment_context_limit"},
		{"history truly full", func(context.Context, *api.ChatCompletionRequest) (int, error) { return 9000, nil }, 8192, 1024, "attachment_context_limit"},
		{"counter failure", func(context.Context, *api.ChatCompletionRequest) (int, error) {
			return 0, errors.New("private runtime error")
		}, 8192, 1024, "context_count_unavailable"},
		{"old runtime fallback", func(context.Context, *api.ChatCompletionRequest) (int, error) {
			return 0, inference.ErrPromptCountUnavailable
		}, 8192, 1024, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			turn := &sessions.Turn{Prompt: "Explain this", MaxTokens: test.response}
			request := &api.ChatCompletionRequest{Messages: []api.ChatMessage{{Role: "user", Content: turn.Prompt}}, MaxTokens: &turn.MaxTokens}
			original := append([]api.ChatMessage(nil), request.Messages...)
			sources := []sessions.AttachmentText{{Attachment: sessions.Attachment{ID: "source", Name: "source.txt"}, Text: strings.Repeat("evidence ", 2000)}}
			err := prepareChatSources(withChatSources(context.Background(), sources, turn), request, test.window, test.counter)
			if test.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			typed, ok := err.(*serviceError)
			if !ok || typed.code != test.code || strings.Contains(typed.message, "private runtime") {
				t.Fatalf("wrong failure: %v", err)
			}
			if !reflect.DeepEqual(original, request.Messages) || len(turn.AttachmentSelections) > 0 || turn.MaxTokens != test.response {
				t.Fatal("failed planning changed request/evidence")
			}
		})
	}
}

func TestDocumentBudgetMeasuresDenseUnicodeAndCancellation(t *testing.T) {
	turn := &sessions.Turn{Prompt: "Explain 日本語", MaxTokens: 1024}
	sources := []sessions.AttachmentText{{Attachment: sessions.Attachment{ID: "source", Name: "unicode.txt"}, Text: strings.Repeat("日本語研究 données café.\n", 2000)}}
	request := &api.ChatCompletionRequest{Messages: []api.ChatMessage{{Role: "user", Content: turn.Prompt}}, MaxTokens: &turn.MaxTokens}
	calls := 0
	counter := documentCounterFunc(func(ctx context.Context, r *api.ChatCompletionRequest) (int, error) {
		calls++
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return len(r.Messages[0].StringContent()) + 24, nil
	})
	ctx := withChatSources(context.Background(), sources, turn)
	if err := prepareChatSources(ctx, request, 4096, counter); err != nil {
		t.Fatal(err)
	}
	if calls < 3 || calls > 7 || len(request.Messages[0].StringContent())+24+1024+64 > 4096 {
		t.Fatal("density was guessed instead of measured", calls)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := prepareChatSources(cancelled, request, 4096, counter); err == nil {
		t.Fatal("cancelled selection accepted")
	}
}

// The tokenizer and generation must share one pinned lease. Otherwise a model
// switch between planning and generation could invalidate the measured budget.
type leasedDocumentEngine struct {
	chatServiceStreamEngine
	gate   *inference.LifecycleGate
	t      *testing.T
	counts int
}

func (e *leasedDocumentEngine) CountPromptTokens(ctx context.Context, r *api.ChatCompletionRequest) (int, error) {
	e.counts++
	if e.gate.Status().Active != 1 {
		e.t.Error("tokenizer ran outside generation lease")
	}
	return fixtureDocumentTokens(ctx, r)
}
func TestDocumentStreamUsesMeasuredBudgetInsideGenerationLease(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	engine := &leasedDocumentEngine{chatServiceStreamEngine: chatServiceStreamEngine{finish: "stop"}, gate: gate, t: t}
	server := &Server{config: &config.Config{MaxContextSize: 65536, ChatContextSize: 8192}, registry: registry, engine: engine, inferenceLifecycle: gate}
	for _, response := range []int{4096, 8192} {
		turn := &sessions.Turn{Prompt: "What is the evidence?", MaxTokens: response}
		sources := []sessions.AttachmentText{{Attachment: sessions.Attachment{ID: "source", Name: "paper.txt"}, Text: strings.Repeat("Supported evidence from a long paper. ", 3000)}}
		messages := []api.ChatMessage{{Role: "user", Content: "Explain the document"}, {Role: "assistant", Content: strings.Repeat("Previous answer with evidence. ", 200)}, {Role: "user", Content: turn.Prompt}}
		ctx := withChatSources(context.Background(), sources, turn)
		before := engine.calls
		result, err := server.streamSessionChat(ctx, &api.ChatCompletionRequest{Model: "model", Messages: messages, MaxTokens: &turn.MaxTokens, Stream: true}, "interactive", func(ChatStreamEvent) error { return nil })
		if response == 4096 {
			if err != nil || result.Answer != "Hello" || engine.counts < 2 || len(turn.AttachmentSelections) != 1 {
				t.Fatalf("measured stream failed: %+v %v", result, err)
			}
		} else {
			failure, ok := err.(*serviceError)
			if !ok || failure.code != "attachment_context_limit" || engine.calls != before {
				t.Fatalf("invalid request dispatched or lost recovery: %v", err)
			}
		}
		if gate.Status().Active != 0 {
			t.Fatal("lease leaked")
		}
	}
}
