package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationCLIResponseOptionsCapabilityAndForwarding(t *testing.T) {
	for _, supported := range []bool{true, false} {
		t.Run(map[bool]string{true: "supported", false: "old-service"}[supported], func(t *testing.T) {
			var received []api.ConversationInputCommand
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/system":
					caps := []string{"conversation-inputs-v1"}
					if supported {
						caps = append(caps, "conversation-input-options-v1")
					}
					json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": caps})
				case "/v1/sessions/Notes":
					json.NewEncoder(w).Encode(sessions.Session{Name: "Notes", CanonicalID: "conversation", CreatedAt: time.Now(), UpdatedAt: time.Now()})
				case "/api/v2/conversations/conversation":
					json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: 3})
				case "/api/v2/conversations/conversation/inputs":
					if r.Method == "GET" {
						json.NewEncoder(w).Encode(api.ConversationInboxPage{})
						return
					}
					var input api.ConversationInputCommand
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Error(err)
					}
					received = append(received, input)
					json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Input: &api.ConversationInboxEntry{ID: "input", Input: input.Payload}})
				default:
					t.Error("unexpected route", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "fixture", nil)
			err := RunPlainConversationTurn(context.Background(), client, []string{"send", "Notes", "--response-profile", "extended", "--max-tokens", "256", "--request-id", "options-request", "Explain this"}, &bytes.Buffer{}, true)
			if !supported {
				if err == nil || !strings.Contains(err.Error(), "no work was submitted") || len(received) != 0 {
					t.Fatal(err, received)
				}
				return
			}
			if err != nil || len(received) != 1 {
				t.Fatal(err, received)
			}
			if received[0].Payload.ResponseProfile != "extended" || received[0].Payload.MaxTokens != 256 || received[0].RequestID != "options-request" {
				t.Fatal(received)
			}
		})
	}
	for _, args := range [][]string{{"--response-profile", "invalid"}, {"--max-tokens", "0"}, {"--max-tokens", "4097"}} {
		if err := RunPlainConversationTurn(context.Background(), nil, append([]string{"send", "Notes"}, args...), &bytes.Buffer{}, true); err == nil {
			t.Fatal("invalid option accepted", args)
		}
	}
}

func TestConversationCLIDisabledDefaultAdmissionCannotFallBack(t *testing.T) {
	for _, interactive := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v2/system":
				json.NewEncoder(w).Encode(map[string]any{"product": "offgrid", "capabilities": []string{"conversation-inputs-v1", "conversation-input-options-v1"}})
			case "/v1/sessions/Notes":
				json.NewEncoder(w).Encode(sessions.Session{Name: "Notes", CreatedAt: time.Now(), UpdatedAt: time.Now()})
			default:
				t.Error("disabled admission attempted mutation", r.Method, r.URL.Path)
				w.WriteHeader(404)
			}
		}))
		client, _ := serviceclient.New(server.URL, "fixture", nil)
		var err error
		if interactive {
			err = RunPlainConversationChat(context.Background(), client, []string{"chat", "Notes"}, strings.NewReader("Hello\n/exit\n"), &bytes.Buffer{}, false)
		} else {
			err = RunPlainConversationTurn(context.Background(), client, []string{"send", "Notes", "Hello"}, &bytes.Buffer{}, true)
		}
		server.Close()
		if err == nil || !strings.Contains(err.Error(), "admission is disabled") {
			t.Fatal(interactive, err)
		}
	}
}
