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
)

func TestBrowserTaskPairingActualSavedInputAndCompanion(t *testing.T) {
	s := taskFirstServer(t)
	s.workspaceID = "workspace"
	code, _ := s.browserHub.PairCode("local-admin")
	features := computer.BrowserFeatures{Discovery: 1, Workspace: 1, Structured: 1}
	origins := []string{"https://one.example", "https://two.example"}
	old, _, err := s.browserHub.PairBrowserFeatures(code, origins[0], 1, computer.ApprovalScopedChanges, features, origins)
	if err != nil {
		t.Fatal(err)
	}
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ComputerSession = old.ID
	cfg.ComputerDriver = "browser"
	cfg.ComputerApprovalMode = "scoped_changes"
	cfg.RequestedComputerApprovalMode = "scoped_changes"
	task, err := s.agentRunner.Create("Inspect the same owned browser", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.browserHub.Reserve(task.Actor, old.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	s.browserHub.StopSession(task.Actor, old.ID)
	if _, err = s.agentRunner.Pause(task.ID, task.Actor, true); err != nil {
		t.Fatal(err)
	}
	task, err = s.agentRunner.RequestAccess(task.ID, task.Actor, "browser", "Documentation", origins[0])
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"protocol_version": 1, "run_id": task.ID, "input_id": task.PendingInput.ID, "previous_session": old.ID, "approval_mode": "scoped_changes"}
	post := func(body map[string]any) *httptest.ResponseRecorder {
		encoded, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/api/v2/computer/pairing", strings.NewReader(string(encoded)))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.handleBrowserPairing(w, r)
		return w
	}
	for key, value := range map[string]any{"run_id": "run-other", "input_id": "input-other", "previous_session": "another", "approval_mode": "full_task", "protocol_version": 3, "unknown": true} {
		bad := map[string]any{}
		for k, v := range body {
			bad[k] = v
		}
		bad[key] = value
		if response := post(bad); response.Code < 400 {
			t.Fatal("invalid task pairing accepted", key, response.Body.String())
		}
	}
	response := post(body)
	var enrollment struct {
		Code string `json:"code"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &enrollment) != nil || enrollment.Code == "" {
		t.Fatal(response.Code, response.Body.String())
	}
	pair, _ := json.Marshal(map[string]any{"code": enrollment.Code, "origin": origins[0], "protocol_version": 1, "approval_mode": "scoped_changes", "browser_features": features, "browser_origins": origins})
	request := httptest.NewRequest("POST", "/api/v2/computer/companion/pair", strings.NewReader(string(pair)))
	request.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	s.browserTransport(http.NotFoundHandler()).ServeHTTP(result, request)
	var paired struct {
		Session computer.BrowserSession `json:"session"`
	}
	if result.Code != 200 || json.Unmarshal(result.Body.Bytes(), &paired) != nil || paired.Session.RunID != task.ID || paired.Session.InputID != task.PendingInput.ID {
		t.Fatal(result.Code, result.Body.String())
	}
	if err = s.browserHub.ReserveTask(task.Actor, paired.Session.ID, "run-other"); err == nil {
		t.Fatal("other task claimed renewal")
	}
	if _, err = s.browserHub.ExecuteAuthorized(context.Background(), task.Actor, paired.Session.ID, "run-other", "call", "browser_observe", json.RawMessage("{}"), "auto"); err == nil {
		t.Fatal("other task dispatched")
	}
	identity := httptest.NewRecorder()
	s.handleSystemIdentity(identity, httptest.NewRequest("GET", "/api/v2/system", nil))
	if !strings.Contains(identity.Body.String(), "browser-task-pairing-v1") {
		t.Fatal("missing additive capability")
	}
}
