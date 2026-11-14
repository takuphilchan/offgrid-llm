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

func TestConversationCLIDiscussesPendingQuestionAndStartsSeparateWork(t *testing.T) {
	var received []api.ConversationInputCommand
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/conversations/conversation":
			json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: 5, FocusedWorkID: "work-one", PendingQuestion: &api.ConversationQuestion{ID: "question-one", WorkID: "work-one", RunID: "run-one", Content: "Include offline runtimes?"}})
		case "/api/v2/conversations/conversation/inputs":
			if r.Method == "GET" {
				json.NewEncoder(w).Encode(api.ConversationInboxPage{Items: []api.ConversationInboxEntry{}})
				return
			}
			var command api.ConversationInputCommand
			if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
				t.Error(err)
			}
			received = append(received, command)
			json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Input: &api.ConversationInboxEntry{ID: "accepted", RequestID: command.RequestID, Input: command.Payload, State: "accepted"}})
		default:
			t.Error("unexpected operation", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "fixture", nil)
	var out bytes.Buffer
	if err := RunCanonicalConversationChat(context.Background(), client, &sessions.Session{Name: "Notes", CanonicalID: "conversation"}, strings.NewReader("/message What does offline mean?\n/new Compare another topic.\n/exit\n"), &out); err != nil {
		t.Fatal(err)
	}
	if len(received) != 2 || received[0].Payload.Intent != "message" || received[0].Payload.QuestionID != "" || received[0].Payload.FocusedWorkID == nil || *received[0].Payload.FocusedWorkID != "work-one" || received[1].Payload.Intent != "new_work" || received[1].Payload.FocusedWorkID != nil || received[1].Payload.QuestionID != "" {
		t.Fatal(received)
	}
}

func TestConversationCLIScriptedIntent(t *testing.T) {
	for _, intent := range []string{"message", "new_work", "correction"} {
		t.Run(intent, func(t *testing.T) {
			var received api.ConversationInputCommand
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/sessions/Notes":
					json.NewEncoder(w).Encode(sessions.Session{Name: "Notes", CanonicalID: "conversation", CreatedAt: time.Now(), UpdatedAt: time.Now()})
				case "/api/v2/conversations/conversation":
					json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: 3, FocusedWorkID: "work-one"})
				case "/api/v2/conversations/conversation/inputs":
					if r.Method == http.MethodGet {
						json.NewEncoder(w).Encode(api.ConversationInboxPage{Items: []api.ConversationInboxEntry{}})
						return
					}
					if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
						t.Error(err)
					}
					json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Input: &api.ConversationInboxEntry{ID: "input-one", Input: received.Payload, State: "accepted"}})
				default:
					t.Error("unexpected request", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "fixture", nil)
			var out bytes.Buffer
			if err := RunPlainConversationTurn(context.Background(), client, []string{"send", "Notes", "--intent", intent, "--request-id", "script-input", "Keep these constraints"}, &out, true); err != nil {
				t.Fatal(err)
			}
			if received.Payload.Intent != intent || received.Payload.QuestionID != "" || received.RequestID != "script-input" {
				t.Fatal(received)
			}
		})
	}
	for _, args := range [][]string{{"--intent", "unknown"}, {"--intent", "message", "--answer", "question"}} {
		if err := RunPlainConversationTurn(context.Background(), nil, append([]string{"send", "Notes"}, args...), &bytes.Buffer{}, true); err == nil {
			t.Fatal("invalid intent accepted", args)
		}
	}
}
