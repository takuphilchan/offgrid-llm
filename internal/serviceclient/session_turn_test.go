package serviceclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestConversationCancellationReachesOwnedTurnBeforeOrAfterHeaders(t *testing.T) {
	for _, headers := range []bool{false, true} {
		t.Run(fmt.Sprint(headers), func(t *testing.T) {
			started, stopped := make(chan struct{}), make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer private" {
					t.Error("missing authentication")
				}
				switch r.URL.Path {
				case "/v1/sessions/chat/generate":
					_, _ = io.Copy(io.Discard, r.Body)
					if headers {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprint(w, ": waiting\n\n")
						w.(http.Flusher).Flush()
					}
					close(started)
					select {
					case <-r.Context().Done():
					case <-time.After(10 * time.Second):
						t.Error("generation request was not disconnected")
					}
				case "/v1/sessions/chat/turn/cancel":
					var body struct {
						ID string `json:"id"`
					}
					_ = json.NewDecoder(r.Body).Decode(&body)
					stopped <- body.ID
					fmt.Fprint(w, `{}`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := New(server.URL, "private", nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				_, err := client.Conversation(ctx, "chat", ConversationRequest{RequestID: "owned-turn", Content: "hello"}, nil)
				result <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not start")
			}
			cancel()
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("cancelled call succeeded")
				}
			case <-time.After(8 * time.Second):
				t.Fatal("cancel stuck")
			}
			select {
			case id := <-stopped:
				if id != "owned-turn" {
					t.Fatal("cancelled a different turn")
				}
			default:
				t.Fatal("saved generation left running")
			}
		})
	}
}

func TestPreparationPreservesRetainedSourceIdentity(t *testing.T) {
	source := sessions.Attachment{ID: "file-" + strings.Repeat("a", 32), Name: "scan.pdf", Bytes: 4, SHA256: strings.Repeat("b", 64), Status: "failed"}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/v1/sessions/chat/attachments/"+source.ID+"/prepare" {
			t.Error("retried upload instead of extraction")
		}
		prepared := source
		prepared.Status = "ready"
		prepared.Revision = strings.Repeat("c", 64)
		if calls == 2 {
			prepared.SHA256 = strings.Repeat("d", 64)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"attachment": prepared})
	}))
	defer server.Close()
	client, _ := New(server.URL, "", nil)
	ready, err := client.PrepareSessionFile(context.Background(), "chat", source)
	if err != nil || ready.Status != "ready" {
		t.Fatalf("retry: %v", err)
	}
	if _, err = client.PrepareSessionFile(context.Background(), "chat", source); err == nil {
		t.Fatal("changed original accepted")
	}
	if calls != 2 {
		t.Fatal("unexpected replay")
	}
}
