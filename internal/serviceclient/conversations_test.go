package serviceclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationTransportPreservesEnvelopeAndNeverReplaysConflict(t *testing.T) {
	calls := 0
	command := api.ConversationCommand{RequestID: "same-request", TargetID: "conversation-1", ExpectedRevision: 7, Kind: "create_work", Payload: api.ConversationCommandPayload{Input: &api.ConversationInput{Content: "Keep this exact request"}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture-key" || r.URL.Path != "/api/v2/conversations/conversation-1/commands" || r.Method != "POST" {
			t.Error("wrong authenticated command", r.Method, r.URL.Path)
		}
		var got api.ConversationCommand
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		if got.RequestID != command.RequestID || got.ExpectedRevision != 7 || got.Payload.Input.Content != command.Payload.Input.Content {
			t.Error("changed command", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":{"code":"conversation_conflict","message":"Refresh the snapshot"}}`))
	}))
	defer server.Close()
	client, err := New(server.URL, "fixture-key", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.ConversationCommand(context.Background(), command); err == nil {
		t.Fatal("conflict hidden")
	}
	if calls != 1 {
		t.Fatal("mutation automatically replayed", calls)
	}
}

func TestWorkItemReadsSharedEnvelopeAndRejectsMismatches(t *testing.T) {
	for _, fixture := range []struct {
		name, body string
		valid      bool
	}{
		{"canonical", `{"work":{"id":"work-one","revision":7,"run_id":"run-one"},"run":{"run_id":"run-one"}}`, true},
		{"bare", `{"id":"work-one","revision":7,"run_id":"run-one"}`, false},
		{"wrong-run", `{"work":{"id":"work-one","revision":7,"run_id":"run-one"},"run":{"run_id":"another"}}`, false},
		{"wrong-work", `{"work":{"id":"another","revision":7,"run_id":"run-one"},"run":{"run_id":"run-one"}}`, false},
		{"missing-revision", `{"work":{"id":"work-one","run_id":"run-one"},"run":{"run_id":"run-one"}}`, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/api/v2/work-items/work-one" {
					t.Error(r.Method, r.URL.Path)
				}
				w.Write([]byte(fixture.body))
			}))
			defer server.Close()
			client, _ := New(server.URL, "fixture", nil)
			work, err := client.WorkItem(context.Background(), "work-one")
			if fixture.valid && (err != nil || work.Revision != 7) || !fixture.valid && err == nil || calls != 1 {
				t.Fatal(work, err, calls)
			}
		})
	}
}
