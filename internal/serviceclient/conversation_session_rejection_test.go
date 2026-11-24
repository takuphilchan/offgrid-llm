package serviceclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationInputRejectionRequiresInitialSubmitResponse(t *testing.T) {
	for _, phase := range []string{"inspect", "submit"} {
		for _, status := range []int{400, 403, 404, 409, 413, 422, 429, 503} {
			t.Run(fmt.Sprintf("%s-%d", phase, status), func(t *testing.T) {
				posts := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch {
					case r.Method == "POST":
						posts++
						w.WriteHeader(status)
					case r.URL.Path == "/api/v2/conversations/conversation/inputs":
						if phase == "inspect" {
							w.WriteHeader(status)
							return
						}
						json.NewEncoder(w).Encode(api.ConversationInboxPage{})
					case r.URL.Path == "/api/v2/conversations/conversation":
						json.NewEncoder(w).Encode(api.Conversation{ID: "conversation", Revision: 1})
					default:
						t.Error("unexpected request", r.Method, r.URL.Path)
						w.WriteHeader(404)
					}
				}))
				defer server.Close()
				client, _ := New(server.URL, "", nil)
				_, err := client.SubmitSessionInput(context.Background(), "conversation", "request", api.ConversationInboxInput{Content: "hello"})
				want := phase == "submit" && (status == 400 || status == 413 || status == 422)
				var remote *Error
				if ConversationInputRejected(err) != want || !errors.As(err, &remote) || remote.Status != status {
					t.Fatal("rejection provenance or wrapped error lost", err, want)
				}
				if phase == "inspect" && posts != 0 || phase == "submit" && posts != 1 {
					t.Fatal(posts)
				}
			})
		}
	}
}

func TestConversationInputOptionsRequireCapabilityOnlyBeforeFreshSubmission(t *testing.T) {
	for _, retained := range []bool{false, true} {
		t.Run(fmt.Sprint(retained), func(t *testing.T) {
			input := api.ConversationInboxInput{Content: "hello", ResponseProfile: "interactive", MaxTokens: 256}
			identities, posts := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v2/conversations/conversation/inputs":
					if r.Method == "POST" {
						posts++
						t.Error("unsupported options were submitted")
						w.WriteHeader(500)
						return
					}
					items := []api.ConversationInboxEntry{}
					if retained {
						items = append(items, api.ConversationInboxEntry{ID: "input", RequestID: "request", Input: input, State: "accepted"})
					}
					json.NewEncoder(w).Encode(api.ConversationInboxPage{Items: items})
				case "/api/v2/system":
					identities++
					json.NewEncoder(w).Encode(Identity{Product: "offgrid"})
				default:
					t.Error("unexpected operation", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, _ := New(server.URL, "", nil)
			entry, err := client.SubmitSessionInput(context.Background(), "conversation", "request", input)
			if retained {
				if err != nil || entry.ID != "input" || identities != 0 {
					t.Fatal(entry, err, identities)
				}
			} else {
				var remote *Error
				if !errors.As(err, &remote) || remote.Code != "upgrade_required" || identities != 1 || !ConversationInputRejected(err) {
					t.Fatal(entry, err, identities)
				}
			}
			if posts != 0 {
				t.Fatal(posts)
			}
		})
	}
}
