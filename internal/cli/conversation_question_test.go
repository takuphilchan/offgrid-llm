package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationQuestionCLIAnswersWithoutWorkerSelection(t *testing.T) {
	for _, interactive := range []bool{true, false} {
		t.Run(map[bool]string{true: "interactive-retry", false: "structured"}[interactive], func(t *testing.T) {
			var mu sync.Mutex
			var commands []api.ConversationInputCommand
			var entry *api.ConversationInboxEntry
			question := &api.ConversationQuestion{ID: "question-one", WorkID: "work-one", RunID: "run-one", Content: "Include offline runtimes?"}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				switch r.URL.Path {
				case "/v1/sessions/Notes":
					json.NewEncoder(w).Encode(sessions.Session{Name: "Notes", CanonicalID: "conversation", CreatedAt: time.Now(), UpdatedAt: time.Now()})
				case "/api/v2/conversations/conversation":
					json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: 5, FocusedWorkID: "work-one", PendingQuestion: question})
				case "/api/v2/conversations/conversation/inputs":
					if r.Method == "GET" {
						items := []api.ConversationInboxEntry{}
						if entry != nil {
							items = append(items, *entry)
						}
						json.NewEncoder(w).Encode(api.ConversationInboxPage{Items: items})
						return
					}
					var command api.ConversationInputCommand
					if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
						t.Error(err)
					}
					commands = append(commands, command)
					entry = &api.ConversationInboxEntry{ID: "accepted-answer", RequestID: command.RequestID, ConversationID: command.TargetID, Input: command.Payload, State: "responded", Incorporation: "worker_answer"}
					question = nil
					if interactive {
						w.WriteHeader(503)
						return
					}
					json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Input: entry})
				default:
					t.Error("answer used unrelated operation", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "fixture", nil)
			var out bytes.Buffer
			var err error
			if interactive {
				err = RunCanonicalConversationChat(context.Background(), client, &sessions.Session{Name: "Notes", CanonicalID: "conversation"}, strings.NewReader("Yes, include them.\n/retry\n/exit\n"), &out)
			} else {
				err = RunPlainConversationTurn(context.Background(), client, []string{"send", "Notes", "--answer", "question-one", "--request-id", "answer-once", "Yes, include them."}, &out, true)
			}
			if err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(commands) != 1 || commands[0].ExpectedRevision != 5 || commands[0].Payload.Intent != "answer" || commands[0].Payload.QuestionID != "question-one" || commands[0].Payload.Content != "Yes, include them." {
				t.Fatal(commands)
			}
			if interactive && (!strings.Contains(out.String(), "Include offline runtimes?") || !strings.Contains(out.String(), "Accepted input accepted-answer")) {
				t.Fatal(out.String())
			}
			if !interactive && !strings.Contains(out.String(), `"incorporation":"worker_answer"`) {
				t.Fatal("structured answer result missing", out.String())
			}
		})
	}
}
