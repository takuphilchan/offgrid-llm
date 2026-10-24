package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
)

// The HTTP server and llama client already allow fifteen minutes. Keep-alives
// must not extend this budget or hide a stalled runtime indefinitely.
const chatStreamBudget = 15 * time.Minute
const chatStreamHeartbeat = 15 * time.Second

// chatEventStream serializes response writes and owns the keep-alive lifetime.
// Comments describe transport progress, never model tokens or tool decisions.
type chatEventStream struct {
	http.ResponseWriter
	ctx      context.Context
	cancel   context.CancelFunc
	parent   context.Context
	id       string
	started  time.Time
	mu       sync.Mutex
	phase    string
	closed   bool
	done     chan struct{}
	stopOnce sync.Once
}

func startChatEventStream(w http.ResponseWriter, r *http.Request, model string) (*chatEventStream, error) {
	return newChatEventStream(w, r, model, chatStreamBudget, chatStreamHeartbeat)
}

func newChatEventStream(w http.ResponseWriter, r *http.Request, model string, budget, heartbeat time.Duration) (*chatEventStream, error) {
	if _, ok := w.(http.Flusher); !ok {
		writeError(w, "Streaming not supported", http.StatusInternalServerError)
		return nil, errors.New("streaming not supported")
	}
	ctx, cancel := context.WithTimeout(r.Context(), budget)
	s := &chatEventStream{ResponseWriter: w, ctx: ctx, cancel: cancel, parent: r.Context(),
		id: fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()), started: time.Now(), phase: "waiting_for_runtime", done: make(chan struct{})}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	// A standard role-only chunk starts the stream without claiming that the
	// model has generated text. Do not include usage, a tool call or a finish.
	if err := s.writeJSON(map[string]any{"id": s.id, "object": "chat.completion.chunk", "created": s.started.Unix(), "model": model,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"role": "assistant"}, "finish_reason": nil}}}); err != nil {
		cancel()
		return nil, err
	}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				if !s.closed && ctx.Err() == nil {
					_ = s.writeProgressLocked()
				}
				s.mu.Unlock()
			}
		}
	}()
	return s, nil
}

func (s *chatEventStream) Close() {
	s.stopOnce.Do(func() {
		s.cancel()
		<-s.done // no writer may outlive the HTTP handler
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
	})
}

func (s *chatEventStream) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(data)
}

func (s *chatEventStream) writeLocked(data []byte) (int, error) {
	if s.closed || s.parent.Err() != nil {
		return 0, context.Canceled
	}
	controller := http.NewResponseController(s.ResponseWriter)
	// Bound a disconnected/non-reading client's writes as well as inference.
	_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
	n, err := s.ResponseWriter.Write(data)
	if err == nil {
		err = controller.Flush()
	}
	if err != nil {
		s.cancel()
	}
	return n, err
}

// Every Write flushes atomically. Existing stream serializers may still Flush.
func (s *chatEventStream) Flush() {}

func (s *chatEventStream) writeJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s, "data: %s\n\n", data)
	return err
}

func (s *chatEventStream) setPhase(phase string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if s.phase == phase {
		return nil
	}
	s.phase = phase
	return s.writeProgressLocked()
}

func (s *chatEventStream) writeProgressLocked() error {
	data, _ := json.Marshal(map[string]any{"phase": s.phase, "elapsed_ms": time.Since(s.started).Milliseconds()})
	_, err := s.writeLocked([]byte(": offgrid " + string(data) + "\n\n"))
	return err
}

func (s *chatEventStream) writeError(err error) {
	if s.parent.Err() != nil {
		return // client disconnected; never convert cancellation into success
	}
	code, message := "stream_error", "Inference failed before a complete answer was produced"
	var service *serviceError
	var decision *inference.DecisionValidationError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code, message = "inference_timeout", "Inference deadline expired; no complete answer was produced. The request was not replayed"
	case errors.Is(err, context.Canceled):
		code, message = "request_cancelled", "Inference was cancelled; no complete answer was produced"
	case errors.As(err, &decision):
		code, message = "invalid_model_tool_response", "The model returned a tool call that failed validation. No tool call from this response was released. Try a model qualified for this agent's tool workflow"
	case errors.As(err, &service):
		message = service.message
		if service.code != "" {
			code = service.code
		}
	default:
		if engineErr := inference.AsEngineError(err); engineErr != nil {
			if engineErr.Code != "" {
				code = engineErr.Code
			}
			if engineErr.Message != "" {
				message = engineErr.Message
			}
		}
	}
	_ = s.writeJSON(map[string]any{"error": map[string]string{"message": message, "type": code, "code": code}})
}
