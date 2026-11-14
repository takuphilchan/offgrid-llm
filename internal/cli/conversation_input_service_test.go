package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestCanonicalConversationProfileSelectionRetainsModelAndRestrictions(t *testing.T) {
	for _, arg := range []string{"writer 5", "none"} {
		t.Run(arg, func(t *testing.T) {
			disabled := false
			knowledge := []string{}
			current := api.Conversation{ID: "conversation", Revision: 8, Preferences: api.ConversationPreferences{Model: "explicit-model", Restrictions: api.ResourceRestrictions{Computer: &disabled, Knowledge: &knowledge}}}
			var command api.ConversationCommand
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer fixture" {
					t.Error("missing authentication")
				}
				switch r.URL.Path {
				case "/api/v2/conversations/conversation":
					json.NewEncoder(w).Encode(current)
				case "/api/v2/agent-profiles/writer":
					if r.URL.Query().Get("revision") != "5" {
						t.Error("revision not pinned")
					}
					json.NewEncoder(w).Encode(map[string]any{"id": "writer", "revision": 5})
				case "/api/v2/conversations/conversation/commands":
					if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
						t.Error(err)
					}
					json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Conversation: &current})
				default:
					t.Error("unexpected operation", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "fixture", nil)
			if err := canonicalChatControl(context.Background(), client, current.ID, "/profile", arg); err != nil {
				t.Fatal(err)
			}
			preferences := command.Payload.Preferences
			if command.ExpectedRevision != 8 || preferences == nil || preferences.Model != "explicit-model" || preferences.Restrictions.Computer == nil || *preferences.Restrictions.Computer || preferences.Restrictions.Knowledge == nil {
				t.Fatal("specialization lost persistent restrictions", command)
			}
			if arg == "none" && preferences.ProfileID != "" || arg != "none" && (preferences.ProfileID != "writer" || preferences.ProfileRevision != 5) {
				t.Fatal("profile selection not pinned", preferences)
			}
		})
	}
}

func TestCanonicalFolderSelectionAndClearUseSharedCommands(t *testing.T) {
	for _, mode := range []string{"supported", "unsupported", "none"} {
		t.Run(mode, func(t *testing.T) {
			current := api.Conversation{ID: "conversation", Revision: 8, ProjectID: "project"}
			var commands []api.ConversationCommand
			resource := execution.Resource{ProjectContext: mode != "unsupported", Reference: execution.Reference{Host: "host", Session: "session", Grant: "grant", Revision: 3}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/conversations/conversation":
					json.NewEncoder(w).Encode(current)
				case "/api/v2/execution/workspaces":
					if mode == "none" {
						t.Error("clearing required a live host")
					}
					json.NewEncoder(w).Encode(map[string]any{"protocol": 1, "workspaces": []execution.Resource{resource}})
				case "/api/v2/conversations/conversation/commands":
					var command api.ConversationCommand
					if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
						t.Error(err)
					}
					commands = append(commands, command)
					json.NewEncoder(w).Encode(api.ConversationCommandReceipt{Conversation: &current})
				default:
					t.Error("unexpected operation", r.Method, r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := serviceclient.New(server.URL, "fixture", nil)
			session := "session"
			if mode == "none" {
				session = "none"
			}
			err := canonicalChatControl(context.Background(), client, current.ID, "/folder", session)
			if mode == "unsupported" {
				if err == nil || len(commands) != 0 {
					t.Fatal("unsupported host silently selected", err)
				}
				return
			}
			if err != nil || len(commands) != 1 {
				t.Fatal(commands, err)
			}
			command := commands[0]
			if command.ExpectedRevision != 8 || command.TargetID != current.ID || command.RequestID == "" {
				t.Fatal(command)
			}
			if mode == "none" {
				if command.Kind != "clear_project_folder" || command.Payload.Folder != nil {
					t.Fatal(command)
				}
			} else if command.Kind != "use_project_folder" || command.Payload.Folder == nil || command.Payload.Folder.Revision != 3 {
				t.Fatal(command)
			}
		})
	}
}

func TestSessionFolderStructuredOutputNeedsNoProfileOrExecutor(t *testing.T) {
	current := api.Conversation{ID: "conversation", Revision: 4, ProjectID: "project"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sessions/Notes":
			json.NewEncoder(w).Encode(sessions.Session{Name: "Notes", CanonicalID: current.ID, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		case "/api/v2/conversations/conversation":
			json.NewEncoder(w).Encode(current)
		case "/api/v2/conversations/conversation/commands":
			var command api.ConversationCommand
			if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
				t.Error(err)
			}
			if command.Kind != "clear_project_folder" || command.ExpectedRevision != 4 {
				t.Error(command)
			}
			current.Revision++
			json.NewEncoder(w).Encode(api.ConversationCommandReceipt{RequestID: command.RequestID, TargetID: current.ID, Revision: current.Revision, Conversation: &current})
		default:
			t.Error("unexpected setup, executor, or job request", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, _ := serviceclient.New(server.URL, "fixture", nil)
	var out bytes.Buffer
	if err := RunSessionService(context.Background(), client, []string{"folder", "Notes", "none"}, &out, true, nil); err != nil {
		t.Fatal(err)
	}
	var receipt api.ConversationCommandReceipt
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil || receipt.RequestID == "" || receipt.Conversation == nil || receipt.Conversation.ProjectID != "project" {
		t.Fatal(out.String(), err)
	}
}
