package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type blockedStreamProcess struct {
	retryDetectLlamaProcess
	cancelled chan struct{}
	started   chan struct{}
}

func (p *blockedStreamProcess) Activate(ctx context.Context, _, _, _ string, _ inference.LoadOptions) (int, error) {
	if p.started != nil {
		close(p.started)
	}
	<-ctx.Done()
	close(p.cancelled)
	return 0, ctx.Err()
}

func TestChatStreamStartsBeforeLoadAndCancelsOwnedRequest(t *testing.T) {
	s := newTestServer(t)
	if err := os.WriteFile(filepath.Join(s.config.ModelsDir, "slow.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	model := s.registry.ListModels()[0].ID
	p := &blockedStreamProcess{cancelled: make(chan struct{}), started: make(chan struct{})}
	s.llamaProcess = p
	s.engine = &beforeOutputFailureEngine{}
	server := httptest.NewServer(http.HandlerFunc(s.handleChatCompletions))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, _ := http.NewRequestWithContext(ctx, "POST", server.URL, strings.NewReader(fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hello"}],"stream":true}`, model)))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal("stream did not start before the blocked model load:", err)
	}
	reader := bufio.NewReader(response.Body)
	first, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(first, `"delta":{"role":"assistant"}`) || strings.Contains(first, `"tool_calls"`) || strings.Contains(first, `"content"`) {
		t.Fatalf("invalid start event: %s (%v)", first, err)
	}
	// Preparation includes queue time; independently wait for the owned load.
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(line, `"phase":"preparing_model"`) {
			break
		}
	}
	select {
	case <-p.started:
	case <-ctx.Done():
		t.Fatal("model preparation never started")
	}
	cancel()
	response.Body.Close()
	select {
	case <-p.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("client cancellation did not reach owned model load")
	}
	if s.engine.(*beforeOutputFailureEngine).calls != 0 {
		t.Fatal("cancelled load reached generation")
	}
}

func TestChatStreamValidationBeforeStartup(t *testing.T) {
	s := newTestServer(t)
	for _, body := range []string{
		`{"stream":true,"messages":[{"role":"user","content":"hi"}]}`,
		`{"stream":true,"model":"missing","messages":[{"role":"user","content":"hi"}]}`,
		`{"stream":true,"model":"missing","response_format":{"type":"invalid"}}`,
	} {
		w := httptest.NewRecorder()
		s.handleChatCompletions(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
		if w.Code < 400 || strings.Contains(w.Body.String(), "data:") {
			t.Fatalf("invalid request accepted: %d %s", w.Code, w.Body)
		}
	}
}

type waitingStreamEngine struct{ inference.Engine }

func (*waitingStreamEngine) ChatCompletionStreamRaw(ctx context.Context, _ *api.ChatCompletionRequest, _ inference.ChatCompletionStreamCallback) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestChatStreamHeartbeatDoesNotExtendBudgetOrInventOutput(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", nil)
	stream, err := newChatEventStream(w, r, "fixture", 60*time.Millisecond, 5*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{engine: &waitingStreamEngine{}}
	s.handleChatCompletionsStream(stream, r.WithContext(stream.ctx), &api.ChatCompletionRequest{Model: "fixture"}, testStreamLease(t))
	stream.Close()
	body := w.Body.String()
	if !strings.Contains(body, ": offgrid ") || !strings.Contains(body, `"code":"inference_timeout"`) || strings.Contains(body, "[DONE]") || strings.Contains(body, `"content"`) || strings.Contains(body, `"tool_calls"`) {
		t.Fatalf("deadline/heartbeat contract broken: %s", body)
	}
	before := w.Body.Len()
	if _, err := stream.Write([]byte("late")); err == nil || w.Body.Len() != before {
		t.Fatal("stream wrote after close")
	}
}

func TestChatStreamSerializesConcurrentFrames(t *testing.T) {
	w := httptest.NewRecorder()
	s, err := newChatEventStream(w, httptest.NewRequest("POST", "/", nil), "fixture", time.Second, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 20; j++ {
				if err := s.writeJSON(map[string]int{"frame": j}); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	group.Wait()
	s.Close()
	if strings.Count(w.Body.String(), "data:") != 201 {
		t.Fatal("lost/interleaved frames")
	}
}

type failingStreamWriter struct{ header http.Header }

func TestChatStreamPreservesNativeToolFailureClassification(t *testing.T) {
	w := httptest.NewRecorder()
	s, err := newChatEventStream(w, httptest.NewRequest("POST", "/", nil), "fixture", time.Second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	s.writeError(&inference.EngineError{Code: "invalid_model_tool_response", Message: "No tool call was released", Details: "PRIVATE"})
	s.Close()
	body := w.Body.String()
	if !strings.Contains(body, "invalid_model_tool_response") || strings.Contains(body, "PRIVATE") || strings.Contains(body, "[DONE]") {
		t.Fatal(body)
	}
}

func (w failingStreamWriter) Header() http.Header     { return w.header }
func (failingStreamWriter) WriteHeader(int)           {}
func (failingStreamWriter) Flush()                    {}
func (failingStreamWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestChatStreamWriteFailureCancelsWithoutStartingHeartbeat(t *testing.T) {
	_, err := newChatEventStream(failingStreamWriter{header: http.Header{}}, httptest.NewRequest("POST", "/", nil), "fixture", time.Second, time.Millisecond)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write failure lost: %v", err)
	}
}

func TestInferenceOperationalErrorsPreserveCancellationAndRedactCause(t *testing.T) {
	for _, tc := range []struct {
		cause  error
		status int
		code   string
	}{
		{context.Canceled, 499, "request_cancelled"},
		{context.DeadlineExceeded, 504, "inference_timeout"},
	} {
		err := inferenceContextError(fmt.Errorf("private runtime detail: %w", tc.cause))
		if err == nil || !errors.Is(err, tc.cause) {
			t.Fatal("operational cause lost")
		}
		w := httptest.NewRecorder()
		writeServiceError(w, err)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "switch model") {
			t.Fatalf("incorrect operational error: %d %s", w.Code, w.Body)
		}
	}
}

type deadlineStreamWriter struct {
	*httptest.ResponseRecorder
	deadline time.Time
	flushErr error
}

func (w *deadlineStreamWriter) SetWriteDeadline(deadline time.Time) error {
	w.deadline = deadline
	return nil
}
func (w *deadlineStreamWriter) FlushError() error { return w.flushErr }

func TestChatStreamDeadlinesAndFlushErrorsSurviveLoggingMiddleware(t *testing.T) {
	base := &deadlineStreamWriter{ResponseRecorder: httptest.NewRecorder(), flushErr: io.ErrClosedPipe}
	writer := &statusResponseWriter{ResponseWriter: base, statusCode: http.StatusOK}
	_, err := newChatEventStream(writer, httptest.NewRequest("POST", "/", nil), "fixture", time.Second, time.Millisecond)
	if !errors.Is(err, io.ErrClosedPipe) || base.deadline.IsZero() || time.Until(base.deadline) > 11*time.Second {
		t.Fatalf("logging middleware lost deadline or flush error: %v, %v", base.deadline, err)
	}
}
