package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestBrowserFileInitialPairingUsesActualActorAndSavedRequest(t *testing.T) {
	s := taskFirstServer(t)
	s.workspaceID = "workspace"
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	alice, ownerKey, err := s.userStore.CreateUser("file-owner", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, foreignKey, err := s.userStore.CreateUser("file-foreign", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.ContextWindow = true, 8192
	cfg.RequestedComputerApprovalMode = "scoped_changes"
	task, err := s.agentRunner.Create("Read documentation", "model", alice.ID, cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = s.agentRunner.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.PendingInput == nil || task.PendingInput.PreviousSession != "" {
		t.Fatal(task, err)
	}
	body := map[string]any{"protocol_version": 1, "run_id": task.ID, "input_id": task.PendingInput.ID, "previous_session": "", "approval_mode": "scoped_changes"}
	middleware := users.NewMiddleware(s.userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(http.HandlerFunc(s.handleBrowserPairing))
	post := func(key string, body map[string]any) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/api/v2/computer/pairing", strings.NewReader(string(encoded)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if response := post(foreignKey, body); response.Code != 409 {
		t.Fatal("foreign actor paired saved task", response.Code, response.Body.String())
	}
	for key, value := range map[string]any{"run_id": "run-22222222222222222222222222222222", "input_id": "input-stale", "previous_session": "22222222222222222222222222222222", "approval_mode": "full_task"} {
		changed := map[string]any{}
		for k, v := range body {
			changed[k] = v
		}
		changed[key] = value
		if response := post(ownerKey, changed); response.Code != 409 {
			t.Fatal("changed saved request accepted", key, response.Code, response.Body.String())
		}
	}
	response := post(ownerKey, body)
	var enrollment struct {
		Code string `json:"code"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &enrollment) != nil || enrollment.Code == "" {
		t.Fatal(response.Code, response.Body.String())
	}
	features := computer.BrowserFeatures{Discovery: 1, Workspace: 1, Structured: 1, Transfers: 1}
	pair := func(origin string, feature computer.BrowserFeatures, extra ...map[string]any) *httptest.ResponseRecorder {
		body := map[string]any{"code": enrollment.Code, "origin": origin, "protocol_version": 1, "approval_mode": "scoped_changes", "browser_features": feature, "browser_origins": []string{origin}}
		for _, fields := range extra {
			for key, value := range fields {
				body[key] = value
			}
		}
		encoded, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/api/v2/computer/companion/pair", strings.NewReader(string(encoded)))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.browserTransport(http.NotFoundHandler()).ServeHTTP(w, r)
		return w
	}
	if response := pair("https://foreign.example", features); response.Code != 403 {
		t.Fatal("saved requested origin changed", response.Code, response.Body.String())
	}
	if response := pair("https://example.com", features, map[string]any{"file_binding": map[string]any{"actor_id": "forged"}}); response.Code != 400 {
		t.Fatal("client-provided private binding accepted", response.Code, response.Body.String())
	}
	old := features
	old.Transfers = 0
	if response := pair("https://example.com", old); response.Code != 403 {
		t.Fatal("old companion claimed new task code", response.Code, response.Body.String())
	}
	response = pair("https://example.com", features)
	var paired struct {
		Session computer.BrowserSession      `json:"session"`
		Token   string                       `json:"token"`
		Binding *computer.BrowserFileBinding `json:"file_binding"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &paired) != nil || paired.Binding == nil {
		t.Fatal(response.Code, response.Body.String())
	}
	if paired.Binding.Validate() != nil || paired.Binding.ActorID != alice.ID || paired.Binding.RunID != task.ID || paired.Binding.InputID != task.PendingInput.ID || paired.Binding.SessionID != paired.Session.ID || paired.Binding.WorkspaceID != s.workspaceID || paired.Token == "" {
		t.Fatal(paired.Binding)
	}
	if paired.Session.Actor != "" {
		t.Fatal("actor leaked through public session JSON")
	}
	if response := pair("https://example.com", features); response.Code != 403 {
		t.Fatal("one-time code replayed", response.Code)
	}
	identity := httptest.NewRecorder()
	s.handleSystemIdentity(identity, httptest.NewRequest("GET", "/api/v2/system", nil))
	if strings.Contains(identity.Body.String(), "browser-file-transfers-v1") {
		t.Fatal("incomplete host/companion support advertised")
	}
}

func TestBrowserFileLegacyPairingProjectionUnchanged(t *testing.T) {
	s := taskFirstServer(t)
	code, err := s.browserHub.PairCode("local-admin")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(map[string]any{"code": code, "origin": "https://example.com", "protocol_version": 1, "approval_mode": "scoped_changes"})
	r := httptest.NewRequest("POST", "/api/v2/computer/companion/pair", strings.NewReader(string(encoded)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.browserTransport(http.NotFoundHandler()).ServeHTTP(w, r)
	if w.Code != 200 || strings.Contains(w.Body.String(), "file_binding") || strings.Contains(w.Body.String(), "actor_id") {
		t.Fatal(w.Code, w.Body.String())
	}
}
