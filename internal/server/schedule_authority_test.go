package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/automation"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestScheduleAuthenticatedOwnershipAndLiveActorRevocation(t *testing.T) {
	s := profileFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	alice, key, err := s.userStore.CreateUser("alice", "isolated-test-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := s.userStore.CreateUser("bob", "isolated-test-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 26, 8, 59, 57, 0, time.UTC)
	s.schedules = newScheduleController(s, func() time.Time { return now })
	p, err := s.agentManager.SaveProfile(s.workspaceID, alice.ID, "", 0, agents.ProfileSettings{Name: "Owned", Model: "model", MaxIterations: 5})
	if err != nil {
		t.Fatal(err)
	}
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(s.requirePermissionWhenAuthEnabled(users.PermissionChat, s.handleAgentSchedules))
	do := func(token, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, strings.NewReader(string(data)))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	body := scheduleWriteRequest{ProfileID: p.ID, ProfileRevision: p.Revision, Settings: agents.ScheduleSettings{Name: "Owned report", Prompt: "Say hello without tools.", Enabled: true, Calendar: automation.Calendar{Zone: "UTC", Hour: 9, Weekdays: []int{1}}}}
	if w := do("", "POST", "/api/v2/agent-schedules", body); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := do(key, "POST", "/api/v2/agent-schedules", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var view struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &view)
	for _, path := range []string{"/api/v2/agent-schedules/" + view.ID, "/api/v2/agent-schedules/" + view.ID + "/occurrences"} {
		if w = do(other, "GET", path, nil); w.Code != 404 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w = do(other, "GET", "/api/v2/agent-schedules", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), view.ID) {
		t.Fatal(w.Code, w.Body.String())
	}
	policy, err := s.agentManager.GetSchedule(s.workspaceID, alice.ID, view.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.validateScheduleResources(context.Background(), alice.ID, policy.Config, false); err != nil {
		t.Fatal(err)
	}
	if err = s.userStore.UpdateUser(alice.ID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	if err = s.validateScheduleResources(context.Background(), alice.ID, policy.Config, false); err == nil {
		t.Fatal("disabled actor retained authority")
	}
	if w = do(key, "GET", "/api/v2/agent-schedules", nil); w.Code == 200 {
		t.Fatal("disabled key read schedules")
	}
	if err = s.schedules.coordinate(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	now = policy.NextAt.Add(time.Second)
	if err = s.schedules.coordinate(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	history, err := s.agentManager.ListScheduleOccurrences(s.workspaceID, alice.ID, view.ID)
	if err != nil || len(history) != 1 || history[0].State != "blocked" || history[0].Reason != "actor_permission_revoked" || len(s.agentManager.ListTasks()) != 0 {
		t.Fatal(history, err)
	}
	// Disabling authentication is not permission to impersonate old actors.
	s.config.RequireAuth = false
	if s.scheduleActorPermission(alice.ID, users.PermissionChat) {
		t.Fatal("old user promoted into anonymous authority")
	}
}

func TestScheduleRemovedModelFileBlocksBeforeRootCreation(t *testing.T) {
	s, policy, now := scheduledFixture(t, nil)
	model, err := s.registry.GetChatModel(policy.Config.Profile.Settings.Model)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(model.Path); err != nil {
		t.Fatal(err)
	} // Disposable fixture asset only.
	*now = policy.NextAt.Add(time.Second)
	if err = s.schedules.coordinate(context.Background(), *now); err != nil {
		t.Fatal(err)
	}
	history, err := s.agentManager.ListScheduleOccurrences(s.workspaceID, policy.Actor, policy.ID)
	if err != nil || len(history) != 1 || history[0].State != "blocked" || history[0].Reason != "model_unavailable" || len(s.agentManager.ListTasks()) != 0 {
		t.Fatal(history, err)
	}
}

func TestSchedulePinnedProfileAndIndependentResources(t *testing.T) {
	s, policy, _ := scheduledFixture(t, nil)
	settings := policy.Config.Profile.Settings
	settings.Model = "different-unavailable-model"
	if _, err := s.agentManager.SaveProfile(s.workspaceID, policy.Actor, policy.Config.Profile.ID, 1, settings); err != nil {
		t.Fatal(err)
	}
	if err := s.validateScheduleResources(context.Background(), policy.Actor, policy.Config, false); err != nil {
		t.Fatal("silently replaced pinned revision", err)
	}
	config := policy.Config
	config.ComputerAccessOnDemand = true
	if err := s.validateScheduleResources(context.Background(), policy.Actor, config, false); err == nil {
		t.Fatal("computer grant on unattended task")
	}
	if err := s.agentManager.ArchiveProfile(s.workspaceID, policy.Actor, policy.Config.Profile.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.validateScheduleResources(context.Background(), policy.Actor, policy.Config, false); err == nil {
		t.Fatal("archived profile remained eligible")
	}
}
