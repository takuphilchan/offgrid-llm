//go:build !llama

package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSingleStepPreviewOnlyMessageAcrossEverySplit(t *testing.T) {
	for _, raw := range []string{`{"message":"café 日本語 😃\n\t\"quoted\" \\ slash\/"}`, " \n{ \"message\" : \"hello\" }", `{"message":"a\u00e9\ud83d\ude03z"}`} {
		var value struct{ Message string }
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		for size := 1; size <= len(raw); size++ {
			p := singleStepPreview{}
			var got strings.Builder
			for end := size; end < len(raw)+size; end += size {
				if end > len(raw) {
					end = len(raw)
				}
				if err := p.update(raw[:end], func(s string) error { got.WriteString(s); return nil }); err != nil {
					t.Fatal(err)
				}
				if end == len(raw) {
					break
				}
			}
			if got.String() != value.Message {
				t.Fatalf("split %d: %q != %q", size, got.String(), value.Message)
			}
		}
	}
	for _, raw := range []string{`{"tool_call":{"name":"write","arguments":{"message":"secret"}}}`, `{"reasoning":"secret","message":"answer"}`, `prose {"message":"secret"}`, `{"mess age":"secret"}`} {
		p := singleStepPreview{}
		if err := p.update(raw, func(string) error { t.Fatal("private envelope leaked"); return nil }); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSingleStepPreviewCallbackFailure(t *testing.T) {
	p := singleStepPreview{}
	want := errors.New("cancelled presentation")
	if err := p.update(`{"message":"hi`, func(string) error { return want }); !errors.Is(err, want) {
		t.Fatal(err)
	}
}

func TestSingleStepPreviewIsSeparateFromValidatedDecision(t *testing.T) {
	for _, tail := range []string{` world"}`, `","tool_call":{"name":"fixture_open","arguments":{}}}`, ` world`} {
		t.Run(tail, func(t *testing.T) {
			preview := make(chan string, 1)
			release := make(chan struct{})
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				send := func(text string) {
					data, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": text}}}})
					fmt.Fprintf(w, "data: %s\n\n", data)
					w.(http.Flusher).Flush()
				}
				send(`{"message":"hello`)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				send(tail)
				fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			}))
			defer backend.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ctx = WithDecisionPreview(ctx, func(s string) error {
				select {
				case preview <- s:
				default:
				}
				return nil
			})
			completed := make(chan error, 1)
			calls := 0
			go func() {
				completed <- NewLlamaHTTPEngine(backend.URL).ChatCompletionStreamRaw(ctx, singleStepFixture(), func(json.RawMessage) error { calls++; return nil })
			}()
			select {
			case text := <-preview:
				if text != "hello" {
					t.Errorf("bad draft: %q", text)
				}
			case <-ctx.Done():
				t.Error("draft buffered until terminal")
			}
			close(release)
			err := <-completed
			valid := tail == ` world"}`
			if valid && (err != nil || calls != 1) {
				t.Fatal("complete response lost", err, calls)
			}
			if !valid && (err == nil || calls != 0) {
				t.Fatal("draft authorized malformed decision", err, calls)
			}
		})
	}
}
