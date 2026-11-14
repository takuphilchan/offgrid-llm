package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestCanonicalFolderRetryPreservesEnvelopeAfterLostResponse(t *testing.T) {
	for _, failure := range []string{"server-error", "incomplete-receipt", "expired-auth"} {
		t.Run(failure, func(t *testing.T) {
			var commands []string
			reads, lookups := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/conversations/conversation":
					reads++
					if reads > 1 {
						t.Error("retry refreshed conversation revision")
					}
					json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: 8})
				case "/api/v2/execution/workspaces":
					lookups++
					if lookups > 1 {
						t.Error("retry required host to stay connected")
					}
					json.NewEncoder(w).Encode(map[string]any{"protocol": 1, "workspaces": []execution.Resource{{ProjectContext: true, Reference: execution.Reference{Host: "host", Session: "session", Grant: "private-grant", Revision: 3}}}})
				case "/api/v2/conversations/conversation/commands":
					body, _ := io.ReadAll(r.Body)
					commands = append(commands, string(body))
					if len(commands) == 1 {
						switch failure {
						case "server-error":
							w.WriteHeader(500)
						case "expired-auth":
							w.WriteHeader(401)
						case "incomplete-receipt":
							io.WriteString(w, `{}`)
						}
						return
					}
					json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Conversation: &api.Conversation{ID: "conversation", Revision: 9}})
				default:
					t.Error("unexpected request", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "fixture", nil)
			control := &conversationFolderControl{}
			var out bytes.Buffer
			if handled, err := control.handle(context.Background(), client, "conversation", "/folder", "session", &out); !handled || err == nil || control.pending == nil {
				t.Fatal("unknown outcome lost", handled, err)
			}
			if _, err := control.handle(context.Background(), client, "conversation", "/folder", "none", &out); err == nil {
				t.Fatal("new choice discarded unknown command")
			}
			if _, err := control.handle(context.Background(), client, "conversation", "/retry-folder", "", &out); err != nil {
				t.Fatal(err)
			}
			if len(commands) != 2 || commands[0] != commands[1] || reads != 1 || lookups != 1 || control.pending != nil {
				t.Fatal("retry changed command", commands, reads, lookups)
			}
			if strings.Contains(out.String(), "private-grant") {
				t.Fatal("printed private reference")
			}
		})
	}
}

func TestCanonicalChatFolderConflictAllowsFreshChoice(t *testing.T) {
	var commands []api.ConversationCommand
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/conversations/conversation":
			json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: int64(8 + len(commands))})
		case "/v1/sessions/Notes":
			json.NewEncoder(w).Encode(sessions.Session{Name: "Notes", CanonicalID: "conversation", CreatedAt: time.Now(), UpdatedAt: time.Now()})
		case "/api/v2/conversations/conversation/commands":
			var command api.ConversationCommand
			if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
				t.Error(err)
			}
			commands = append(commands, command)
			if len(commands) == 1 {
				w.WriteHeader(409)
				return
			}
			json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Conversation: &api.Conversation{ID: "conversation", Revision: 10}})
		default:
			t.Error("unexpected request", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "fixture", nil)
	var out bytes.Buffer
	err := RunCanonicalConversationChat(context.Background(), client, &sessions.Session{Name: "Notes", CanonicalID: "conversation"}, strings.NewReader("/folder none\n/retry-folder\n/folder none\n/exit\n"), &out)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || commands[0].RequestID == commands[1].RequestID || commands[0].ExpectedRevision != 8 || commands[1].ExpectedRevision != 9 || !strings.Contains(out.String(), "Folder command acknowledged") {
		t.Fatal(commands, out.String())
	}
}
