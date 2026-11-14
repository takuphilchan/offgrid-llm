package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestUserPreferencesHTTPClientIsolationAndStrictAuthorityBoundary(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	_, key, err := s.userStore.CreateUser("alice", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, foreign, err := s.userStore.CreateUser("bob", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleUserPreferences))
	host := httptest.NewServer(handler)
	defer host.Close()
	alice, err := serviceclient.New(host.URL, key, nil)
	if err != nil {
		t.Fatal(err)
	}
	bob, err := serviceclient.New(host.URL, foreign, nil)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := alice.UserPreferences(context.Background())
	if err != nil || initial.Revision != 0 {
		t.Fatal(initial, err)
	}
	no := false
	command := api.UserPreferencesCommand{RequestID: "set-defaults", TargetID: initial.ID, Kind: "configure_user_preferences", Payload: api.UserDefaultValues{Model: "model", Delegation: &no}}
	receipt, err := alice.ConfigureUserPreferences(context.Background(), command)
	if err != nil || receipt.UserPreferences == nil || receipt.Revision != 1 {
		t.Fatal(receipt, err)
	}
	// The feature flag controls new work. Preferences and existing state remain
	// accessible independently of model availability and new admissions.
	s.registry = nil
	s.config.EnableConversationPreview = false
	if _, err = alice.ConfigureUserPreferences(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	other, err := bob.UserPreferences(context.Background())
	if err != nil || other.Revision != 0 || other.Preferences.Model != "" {
		t.Fatal("foreign defaults", other, err)
	}
	for _, invalid := range []string{
		`{"request_id":"authority","target_id":"user-defaults","expected_revision":1,"kind":"configure_user_preferences","payload":{"restrictions":{"computer":true}}}`,
		`{"request_id":"authority","target_id":"user-defaults","expected_revision":1,"kind":"configure_user_preferences","payload":{"execution_grant":"grant"}}`,
	} {
		r := httptest.NewRequest("POST", "/api/v2/user-preferences/commands", strings.NewReader(invalid))
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest("GET", "/api/v2/user-preferences", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatal("anonymous preferences admitted", w.Code)
	}
}

func TestUserDefaultsProvenanceAndAcceptedRunArePinned(t *testing.T) {
	s := profileFixture(t)
	s.config.EnableConversationPreview = true
	s.config.DefaultModel = "service-model"
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: "local-admin"}
	no := false
	defaults := api.UserPreferencesCommand{RequestID: "defaults", TargetID: agents.UserPreferencesID, Kind: "configure_user_preferences", Payload: api.UserDefaultValues{Model: "model", Delegation: &no}}
	if _, err := s.agentManager.ConfigureUserPreferences(scope, defaults); err != nil {
		t.Fatal(err)
	}
	conversation, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "conversation", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/v2/conversations/"+conversation.TargetID+"/configuration", nil)
	effective, err := s.effectiveConversationPreferences(request, conversation.Conversation, "")
	if err != nil || effective.Model != "model" || effective.Delegation || effective.Provenance["model"] != "user_default" || effective.UserPreferencesRevision != 1 {
		t.Fatal(effective, err)
	}
	work := agents.ConversationCommand{RequestID: "input", TargetID: conversation.TargetID, ExpectedRevision: 1, Kind: "create_work", Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "A simple question"}}}
	w := controlHTTP(s, "POST", "/api/v2/conversations/"+conversation.TargetID+"/commands", work)
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var accepted api.ConversationCommandReceipt
	if err = json.Unmarshal(w.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	defaults.RequestID, defaults.ExpectedRevision = "new-defaults", 1
	defaults.Payload = api.UserDefaultValues{Model: "later-model"}
	if _, err = s.agentManager.ConfigureUserPreferences(scope, defaults); err != nil {
		t.Fatal(err)
	}
	task, found := s.agentManager.GetTask(accepted.Work.RunID)
	if !found || task.Model != "model" || !task.Config.DisableDelegation || task.Config.EffectivePreferences == nil || task.Config.EffectivePreferences.UserPreferencesRevision != 1 || task.Config.EffectivePreferences.Provenance["model"] != "user_default" {
		t.Fatal("accepted run changed defaults", task)
	}
	effective, err = s.effectiveConversationPreferences(request, conversation.Conversation, "")
	if err != nil || effective.Model != "later-model" || effective.UserPreferencesRevision != 2 {
		t.Fatal(effective, err)
	}
	// An explicit request still wins without changing the saved defaults.
	effective, err = s.effectiveConversationPreferences(request, conversation.Conversation, "override")
	if err != nil || effective.Model != "override" || effective.Provenance["model"] != "request" {
		t.Fatal(effective, err)
	}
}
