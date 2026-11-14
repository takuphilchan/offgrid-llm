package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestConversationInputPublicAdmissionAndCrossClientStopWithoutModel(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.EnableConversationPreview = true
	s.registry = nil
	server := httptest.NewServer(http.HandlerFunc(s.handleConversations))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	command := api.ConversationInputCommand{RequestID: "typed-input", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Explain the architecture", Origin: "dictation"}}
	receipt, err := client.SubmitConversationInput(context.Background(), command)
	if err != nil || receipt.Input == nil || receipt.Input.RunID != "" || receipt.Input.State != "accepted" {
		t.Fatal(receipt, err)
	}
	s.config.EnableConversationPreview = false
	retry, err := client.SubmitConversationInput(context.Background(), command)
	if err != nil || retry.Input.ID != receipt.Input.ID {
		t.Fatal("retry blocked by disabled admission/readiness", retry, err)
	}
	stop := api.ConversationInputControlCommand{RequestID: "other-client-stop", TargetID: receipt.Input.ID, ExpectedRevision: receipt.Revision, Kind: "stop"}
	path := "/api/v2/conversations/" + c.ID + "/inputs/" + receipt.Input.ID + "/commands"
	response := controlHTTP(s, "POST", path, stop)
	if response.Code != http.StatusAccepted {
		t.Fatal(response.Code, response.Body.String())
	}
	before, _ := s.agentManager.Conversation(scope, c.ID)
	entry, err := client.ConversationInput(context.Background(), c.ID, receipt.Input.ID)
	if err != nil || entry.State != "stopping" || entry.HoldReleasedAt != nil {
		t.Fatal(entry, err)
	}
	after, _ := s.agentManager.Conversation(scope, c.ID)
	if before.Revision != after.Revision {
		t.Fatal("observation settled Stop")
	}
	controller := &conversationDeliveryController{server: s}
	if err = controller.coordinate(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	entry, err = client.ConversationInput(context.Background(), c.ID, receipt.Input.ID)
	if err != nil || entry.State != "stopped" || entry.RunID != "" || entry.HoldReleasedAt == nil {
		t.Fatal("service did not settle withdrawal", entry, err)
	}
	if _, err = client.ControlConversationInput(context.Background(), c.ID, stop); err != nil {
		t.Fatal("control retry failed after settlement", err)
	}
	config := agents.AgentConfig{}
	if err = s.assembleConversationInboxContext(httptest.NewRequest("GET", "/", nil), c.ID, "", &config); err != nil {
		t.Fatal(err)
	}
	if len(config.ConversationContext) != 1 || config.ConversationContext[0].Role != "assistant" || !strings.Contains(config.ConversationContext[0].StringContent(), "withdrawn") {
		t.Fatal("withdrawn text remained an active user instruction", config.ConversationContext)
	}
}

func TestConversationInputPublicEnvelopeScopeAndCompatibility(t *testing.T) {
	s, scope, c := contextInboxFixture(t)
	s.config.EnableConversationPreview = true
	path := "/api/v2/conversations/" + c.ID + "/inputs"
	command := api.ConversationInputCommand{RequestID: "input", TargetID: c.ID, ExpectedRevision: c.Revision, Kind: "submit_input", Payload: api.ConversationInboxInput{Content: "Hello"}}
	raw, _ := json.Marshal(command)
	for _, body := range []string{strings.Replace(string(raw), `"payload":`, `"host_grant":"forged","payload":`, 1), strings.Replace(string(raw), `"request_id":"input"`, `"request_id":"input","request_id":"duplicate"`, 1), strings.Replace(string(raw), `"expected_revision":1,`, "", 1)} {
		w := httptest.NewRecorder()
		s.handleConversations(w, httptest.NewRequest("POST", path, strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatal("malformed envelope admitted", w.Code, w.Body.String())
		}
	}
	command.Payload.Intent = "answer"
	if response := controlHTTP(s, "POST", path, command); response.Code != 400 {
		t.Fatal("unidentified answer implicitly approved work", response.Code)
	}
	command.Payload.Intent = ""
	foreign, err := s.agentManager.ApplyConversationCommand(agents.ControlScope{Workspace: scope.Workspace, Actor: "other"}, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	command.TargetID = foreign.TargetID
	if response := controlHTTP(s, "POST", "/api/v2/conversations/"+foreign.TargetID+"/inputs", command); response.Code != 404 {
		t.Fatal("foreign input admitted", response.Code)
	}
	command.TargetID = c.ID
	if response := controlHTTP(s, "POST", path, command); response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	command.Payload.Content = "Different request under old identity"
	if response := controlHTTP(s, "POST", path, command); response.Code != 409 {
		t.Fatal("conflicting identity reused", response.Code)
	}
	command.RequestID = "stale-revision"
	if response := controlHTTP(s, "POST", path, command); response.Code != 409 {
		t.Fatal("stale revision accepted", response.Code)
	}
}
