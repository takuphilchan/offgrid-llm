package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationInputHTTPAndGoClientOnlyObserve(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	first := acceptContextInput(t, s, scope, c.ID, "first", "Keep this input")
	second := acceptContextInput(t, s, scope, c.ID, "second", "And this follow-up")
	issue := api.ConversationDeliveryIssue{Code: "model_unavailable", Message: "Select an installed model", RetryAt: time.Now().Add(time.Minute).UTC()}
	if err := s.agentManager.DeferConversationDelivery(scope, c.ID, first.ID, issue); err != nil {
		t.Fatal(err)
	}
	before, _ := s.agentManager.Conversation(scope, c.ID)
	s.config.EnableConversationPreview = false
	server := httptest.NewServer(http.HandlerFunc(s.handleConversations))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err := client.ConversationInputs(context.Background(), c.ID, 0, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != first.ID || !page.HasMore {
		t.Fatal(page, err)
	}
	page, err = client.ConversationInputs(context.Background(), c.ID, page.NextCursor, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != second.ID || page.HasMore {
		t.Fatal(page, err)
	}
	entry, err := client.ConversationInput(context.Background(), c.ID, first.ID)
	if err != nil || entry.RunID != "" || entry.DeliveryIssue == nil || entry.DeliveryIssue.Code != issue.Code {
		t.Fatal(entry, err)
	}
	for _, suffix := range []string{"/inputs?after=-1", "/inputs?limit=201", "/inputs?limit=invalid"} {
		if response := controlHTTP(s, "GET", "/api/v2/conversations/"+c.ID+suffix, nil); response.Code != 400 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	foreign, err := s.agentManager.ApplyConversationCommand(agents.ControlScope{Workspace: scope.Workspace, Actor: "other"}, agents.ConversationCommand{RequestID: "other", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	if response := controlHTTP(s, "GET", "/api/v2/conversations/"+foreign.TargetID+"/inputs/"+first.ID, nil); response.Code != 404 {
		t.Fatal("foreign inbox exposed", response.Code)
	}
	after, _ := s.agentManager.Conversation(scope, c.ID)
	if before.Revision != after.Revision {
		t.Fatal("GET changed control state", before.Revision, after.Revision)
	}
	entry, _ = s.agentManager.ConversationInput(scope, c.ID, first.ID)
	if entry.RunID != "" || entry.State != "accepted" || !entry.DeliveryIssue.RetryAt.Equal(issue.RetryAt) {
		t.Fatal("GET dispatched or reset delivery", entry)
	}
}
