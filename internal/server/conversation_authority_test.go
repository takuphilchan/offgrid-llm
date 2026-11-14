package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestCanonicalOwnerCanStopWithoutProfileAndForeignActorCannotInspect(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.config.EnableConversationPreview = true
	s.userStore = users.NewUserStore("")
	alice, key, err := s.userStore.CreateUser("alice", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, foreign, err := s.userStore.CreateUser("bob", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	scope := agents.ControlScope{Workspace: s.workspaceID, Actor: alice.ID}
	created, err := s.agentManager.ApplyConversationCommand(scope, agents.ConversationCommand{RequestID: "owned-chat", Kind: "create_conversation"})
	if err != nil {
		t.Fatal(err)
	}
	command := agents.ConversationCommand{RequestID: "owned-work", Kind: "create_work", TargetID: created.TargetID, ExpectedRevision: 1, Payload: agents.ConversationCommandPayload{Input: &agents.ConversationInput{Content: "Wait for the owner"}}}
	config := agents.DefaultAgentConfig()
	config.ScopedCapabilities = true
	accepted, err := s.agentRunner.AdmitConversationWork(scope, command, "model", config)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/jobs/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.profileJobAccess(s.handleJobRead)))
	mux.HandleFunc("/api/v2/jobs", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleJobSubmit))
	mux.HandleFunc("/api/v2/work", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleWorkSummary))
	mux.HandleFunc("/api/v2/conversations/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleConversations))
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(mux)
	do := func(token, method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, path := range []string{"/api/v2/jobs/" + accepted.Work.RunID, "/api/v2/conversations/" + created.TargetID, "/api/v2/conversations/" + created.TargetID + "/events"} {
		if w := do(key, "GET", path); w.Code != 200 {
			t.Fatal("owner denied", w.Code, w.Body.String())
		}
		if w := do(foreign, "GET", path); w.Code != 404 {
			t.Fatal("foreign actor saw work", w.Code, w.Body.String())
		}
	}
	s.config.EnableConversationPreview = false
	for _, path := range []string{"/api/v2/jobs", "/api/v2/work"} {
		if w := do(key, "GET", path); w.Code != 200 || !strings.Contains(w.Body.String(), accepted.Work.RunID) {
			t.Fatal("canonical work missing from inspection", w.Code, w.Body.String())
		}
	}
	if w := do(key, "POST", "/api/v2/jobs/"+accepted.Work.RunID+"/cancel"); w.Code != 202 {
		t.Fatal("flag stranded Stop", w.Code, w.Body.String())
	}
	work, err := s.agentManager.WorkItem(scope, accepted.Work.ID)
	if err != nil || work.Status != agents.TaskCancelled {
		t.Fatal("Stop did not commit work state", work, err)
	}
}

func TestCanonicalCapabilityPinsStillRecheckCurrentActorAuthority(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	u, _, err := s.userStore.CreateUser("demoted", "fixture-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.TaskFirst = true
	config.ScopedCapabilities = true
	file, _ := s.toolRegistry.Capability("read_file")
	config.AllowedCapabilities = map[string]capabilities.Descriptor{"read_file": file}
	task, err := s.agentRunner.Create("Read only approved files", "model", u.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.userStore.UpdateUser(u.ID, map[string]any{"role": users.RoleUser}); err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: s.toolRegistry, server: s}
	err = tools.Authorize(context.Background(), "read_file", json.RawMessage(`{}`), agents.ToolExecution{Actor: u.ID, RunID: task.ID, Approved: true, ExpectedCapability: &file})
	if !errors.Is(err, capabilities.ErrDenied) {
		t.Fatal("stale actor authority survived", err)
	}
	for _, tool := range tools.ToolsForTaskContext(context.Background(), task) {
		if tool.Function.Name == "read_file" {
			t.Fatal("revoked resource remains discoverable")
		}
	}
}
