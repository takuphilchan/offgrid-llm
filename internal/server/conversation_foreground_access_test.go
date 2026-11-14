package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestConversationForegroundOwnerCanInspectAndStopWithoutProfile(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth, s.config.DefaultModel = true, "model"
	s.userStore = users.NewUserStore(t.TempDir())
	owner, key, err := s.userStore.CreateUser("owner", "fixture-password", users.RoleGuest)
	if err != nil {
		t.Fatal(err)
	}
	_, foreign, err := s.userStore.CreateUser("other", "fixture-password", users.RoleGuest)
	if err != nil {
		t.Fatal(err)
	}
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: owner.ID}
	c, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "new", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	input := acceptContextInput(t, s, scope, c.TargetID, "input", "Answer this question")
	task, err := s.prepareConversationTurn(context.Background(), scope, c.TargetID, input.ID)
	if err != nil || task.Config.Profile != nil {
		t.Fatal(task, err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/jobs/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.profileJobAccess(s.handleJobRead)))
	mux.HandleFunc("/api/v2/conversations/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleConversations))
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(mux)
	do := func(token, method, path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	for _, path := range []string{"/api/v2/jobs/" + task.ID, "/api/v2/conversations/" + c.TargetID + "/inputs/" + input.ID} {
		if response := do(key, "GET", path); response.Code != 200 {
			t.Fatal("owner denied", response.Code, response.Body.String())
		}
		if response := do(foreign, "GET", path); response.Code != 404 {
			t.Fatal("foreign actor admitted", response.Code, response.Body.String())
		}
	}
	if response := do(key, "POST", "/api/v2/jobs/"+task.ID+"/cancel"); response.Code != 202 {
		t.Fatal("owner could not stop foreground", response.Code, response.Body.String())
	}
	if response := do(key, "DELETE", "/api/v2/jobs/"+task.ID); response.Code != 409 {
		t.Fatal("legacy deletion orphaned foreground input", response.Code, response.Body.String())
	}
	retained, err := s.agentManager.ConversationInput(scope, c.TargetID, input.ID)
	if err != nil || retained.RunStatus != agents.TaskCancelled || retained.State != "failed" || retained.RunID != task.ID {
		t.Fatal(retained, err)
	}
}
