package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Session streaming has its own contract: done means the exchange was saved,
// not merely that the inference connection closed. Only the handler writes SSE.
type ChatTimings struct {
	TotalMS          int64   `json:"total_ms"`
	ReadyMS          int64   `json:"ready_ms"`
	QueueMS          int64   `json:"queue_ms"`
	LoadMS           int64   `json:"load_ms"`
	PromptMS         int64   `json:"prompt_ms"`
	FirstTextMS      int64   `json:"first_text_ms"`
	GenerationMS     int64   `json:"generation_ms"`
	CompletionTokens int     `json:"completion_tokens,omitempty"`
	TokensPerSecond  float64 `json:"tokens_per_second,omitempty"`
	ContextWindow    int     `json:"context_window"`
}

type ChatStreamEvent struct {
	Type         string            `json:"type"`
	Phase        string            `json:"phase,omitempty"`
	Delta        string            `json:"delta,omitempty"`
	Error        string            `json:"error,omitempty"`
	Session      *sessions.Session `json:"session,omitempty"`
	Message      *sessions.Message `json:"message,omitempty"`
	Metrics      *ChatTimings      `json:"metrics,omitempty"`
	FinishReason string            `json:"finish_reason,omitempty"`
}

type ChatStreamResult struct {
	Answer       string
	FinishReason string
	Metrics      ChatTimings
}

type SessionStreamer func(context.Context, *api.ChatCompletionRequest, string, func(ChatStreamEvent) error) (ChatStreamResult, error)

func (h *SessionHandlers) generateStream(w http.ResponseWriter, r *http.Request, name, content, model string, knowledge bool, profile string, maxTokens int) {
	started := time.Now()
	if h.streamer == nil {
		writeError(w, "Session streaming is unavailable", http.StatusServiceUnavailable)
		return
	}
	// Never establish a stream for an inaccessible conversation.
	session, err := h.scoped(r).Load(name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if model == "" {
		model = session.ModelID
	}
	if model == "" {
		writeError(w, "Model is required", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	write := func(event ChatStreamEvent) error {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	if err := write(ChatStreamEvent{Type: "phase", Phase: "queued"}); err != nil {
		return
	}
	events := make(chan ChatStreamEvent)
	emit := func(event ChatStreamEvent) error {
		select {
		case events <- event:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	go func() {
		defer close(events)
		fail := func(err error) {
			log.Printf("Session generation failed: %v", err)
			message := "Response interrupted or could not be saved. Your draft is retained; reload the conversation before retrying."
			if typed, ok := err.(*serviceError); ok {
				message = typed.message
			}
			_ = emit(ChatStreamEvent{Type: "error", Error: message})
		}
		unlock, err := h.lockSessionContext(ctx, name)
		if err != nil {
			return
		}
		defer unlock()
		if ctx.Err() != nil {
			return
		}
		// Read history after acquiring the lock so concurrent tabs cannot lose turns.
		current, err := h.scoped(r).Load(name)
		if err != nil {
			fail(err)
			return
		}
		messages := make([]api.ChatMessage, 0, len(current.Messages)+1)
		for _, message := range current.Messages {
			messages = append(messages, api.ChatMessage{Role: message.Role, Content: message.Content})
		}
		messages = append(messages, api.ChatMessage{Role: "user", Content: content})
		waitMS := time.Since(started).Milliseconds()
		result, err := h.streamer(ctx, &api.ChatCompletionRequest{Model: model, Messages: messages, UseKnowledgeBase: &knowledge, MaxTokens: &maxTokens, Stream: true}, profile, emit)
		if err != nil {
			fail(err)
			return
		}
		if ctx.Err() != nil {
			return
		}
		if err := emit(ChatStreamEvent{Type: "phase", Phase: "saving"}); err != nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
		updated, err := h.scoped(r).AppendExchange(name, model, content, result.Answer)
		if err != nil {
			fail(err)
			return
		}
		result.Metrics.TotalMS = time.Since(started).Milliseconds()
		result.Metrics.ReadyMS += waitMS
		result.Metrics.QueueMS += waitMS
		result.Metrics.FirstTextMS += waitMS
		_ = emit(ChatStreamEvent{Type: "done", Session: updated, Message: &sessions.Message{Role: "assistant", Content: result.Answer, Timestamp: updated.UpdatedAt}, Metrics: &result.Metrics, FinishReason: result.FinishReason})
	}()
	heartbeat := time.NewTicker(10 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := write(event); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
