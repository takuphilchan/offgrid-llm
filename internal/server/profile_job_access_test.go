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

func TestOwnedProfileJobRoutesDoNotGrantAdministration(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	alice, aliceKey, err := s.userStore.CreateUser("alice", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, bobKey, err := s.userStore.CreateUser("bob", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.agentManager.SaveProfile(s.workspaceID, alice.ID, "", 0, agents.ProfileSettings{Name: "Assistant", Model: "model", MaxIterations: 5})
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.Profile = p
	owned, err := s.agentRunner.Create("Saved work", "model", alice.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.agentRunner.Pause(owned.ID, alice.ID, false); err != nil {
		t.Fatal(err)
	}
	legacy, err := s.agentRunner.Create("Previously administrative work", "model", alice.ID, agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/jobs/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.profileJobAccess(s.handleJobRead)))
	mux.HandleFunc("/api/v2/jobs", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleJobSubmit))
	mux.HandleFunc("/api/v2/agent-profiles/", s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleAgentProfiles))
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(mux)
	do := func(key, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, suffix := range []string{"", "/events", "/export"} {
		if w := do(aliceKey, "GET", "/api/v2/jobs/"+owned.ID+suffix, ""); w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if w := do(bobKey, "GET", "/api/v2/jobs/"+owned.ID+suffix, ""); w.Code != 404 {
			t.Fatal("foreign profile work", w.Code)
		}
	}
	if w := do("", "GET", "/api/v2/jobs/"+owned.ID, ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := do(aliceKey, "GET", "/api/v2/jobs/"+legacy.ID, ""); w.Code != 404 {
		t.Fatal("legacy task authority expanded", w.Code)
	}
	if w := do(aliceKey, "GET", "/api/v2/jobs", ""); w.Code != 200 || strings.Contains(w.Body.String(), legacy.ID) || !strings.Contains(w.Body.String(), owned.ID) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, action := range []string{"input", "reconnect"} {
		if w := do(aliceKey, "POST", "/api/v2/jobs/"+owned.ID+"/"+action, `{}`); w.Code != 403 {
			t.Fatal("computer administration expanded", action, w.Code)
		}
	}
	if w := do(aliceKey, "POST", "/api/v2/jobs", `{"prompt":"Do something","model":"model","request_id":"non-profile-task"}`); w.Code != 403 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := do(aliceKey, "GET", "/api/v2/agent-profiles/resources", ""); w.Code != 200 || strings.Contains(w.Body.String(), "read_file") || strings.Contains(w.Body.String(), "write_file") || !strings.Contains(w.Body.String(), "calculator") {
		t.Fatal("service authority exposed", w.Code, w.Body.String())
	}
	if w := do(aliceKey, "POST", "/api/v2/jobs/"+owned.ID+"/cancel", `{}`); w.Code != 202 {
		t.Fatal("owner could not stop own work", w.Code, w.Body.String())
	}
}

func TestProfileDispatchRechecksActorAfterRoleChange(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	u, _, err := s.userStore.CreateUser("changing", "fixture-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.agentManager.SaveProfile(s.workspaceID, u.ID, "", 0, agents.ProfileSettings{Name: "Pinned", Model: "model", MaxIterations: 5, Computer: true})
	if err != nil {
		t.Fatal(err)
	}
	config := agents.DefaultAgentConfig()
	config.Profile = p
	config.TaskFirst = true
	task, err := s.agentRunner.Create("Do not expand permissions", "model", u.ID, config)
	if err != nil {
		t.Fatal(err)
	}
	driver := &browserRunTools{RunTools: s.toolRegistry, server: s}
	file, _ := s.toolRegistry.Capability("read_file")
	if !s.profileActorCapabilityAllowed(u.ID, file) {
		t.Fatal("admin fixture invalid")
	}
	if err := s.userStore.UpdateUser(u.ID, map[string]any{"role": users.RoleUser}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"read_file", "write_file", computerAccessToolName, "computer_observe"} {
		d, ok := driver.Capability(name)
		if !ok {
			t.Fatal(name)
		}
		if s.profileActorCapabilityAllowed(u.ID, d) {
			t.Fatal("stale actor grant", name)
		}
		if err := driver.Authorize(context.Background(), name, json.RawMessage(`{}`), agents.ToolExecution{Actor: u.ID, RunID: task.ID, Approved: true, ExpectedCapability: &d}); !errors.Is(err, capabilities.ErrDenied) {
			t.Fatal(name, err)
		}
	}
	for _, tool := range driver.ToolsForTaskContext(context.Background(), task) {
		if tool.Function.Name != "calculator" && tool.Function.Name != "current_time" && tool.Function.Name != taskArtifactToolName {
			t.Fatal("revoked authority discovered", tool.Function.Name)
		}
	}
	calculator, _ := driver.Capability("calculator")
	if !s.profileActorCapabilityAllowed(u.ID, calculator) {
		t.Fatal("pure builtin unexpectedly removed")
	}
	if err := s.userStore.UpdateUser(u.ID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	if s.profileActorCapabilityAllowed(u.ID, calculator) {
		t.Fatal("disabled actor retained authority")
	}
}
