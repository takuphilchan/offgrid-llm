package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestComputerLegacyMutationDoesNotAcceptApprovalBoolean(t *testing.T) {
	s := &Server{}
	for _, path := range []string{"session", "action", "reset", "stop"} {
		w := httptest.NewRecorder()
		s.handleComputerUpgradeRequired(w, httptest.NewRequest(http.MethodPost, "/v1/computer/"+path, strings.NewReader(`{"approved":true,"target":"desktop"}`)))
		if w.Code != http.StatusUpgradeRequired || !strings.Contains(w.Body.String(), "computer_api_upgrade_required") {
			t.Fatalf("legacy mutation accepted: %s", w.Body.String())
		}
	}
}

func TestComputerCapabilitiesNeverAdvertiseUninstalledDrivers(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	s.handleComputerCapabilities(w, httptest.NewRequest(http.MethodGet, "/api/v2/computer/capabilities", nil))
	var result computer.Capabilities
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
		t.Fatal(w.Body.String())
	}
	if result.Available || !result.Preview || !result.LocalOnly || result.ApprovalMode != "supervised" || result.ProtocolVersion != 1 || result.ReasonCode != "companion_unavailable" {
		t.Fatalf("dishonest readiness: %+v", result)
	}
	if len(result.Drivers) != 4 {
		t.Fatal("missing platform capability states")
	}
	for _, driver := range result.Drivers {
		if driver.Available || driver.Qualified {
			t.Fatal("driver claimed ready without evidence")
		}
	}
	w = httptest.NewRecorder()
	s.handleComputerCapabilities(w, httptest.NewRequest(http.MethodPost, "/api/v2/computer/capabilities", nil))
	if w.Code != 405 {
		t.Fatal("accepted mutation")
	}
}

func TestComputerStopIsSafeBeforeCompanionStartup(t *testing.T) {
	for _, s := range []*Server{{}, {browserHub: computer.NewBrowserHub()}} {
		w := httptest.NewRecorder()
		s.handleComputerStop(w, httptest.NewRequest(http.MethodPost, "/api/v2/computer/stop", nil))
		if w.Code != 200 {
			t.Fatal("stop unavailable")
		}
		w = httptest.NewRecorder()
		s.handleComputerStop(w, httptest.NewRequest(http.MethodGet, "/api/v2/computer/stop", nil))
		if w.Code != 405 {
			t.Fatal("GET changed stop state")
		}
	}
}

func TestScopedSessionStopCannotRevokeAnotherSession(t *testing.T) {
	s := &Server{browserHub: computer.NewBrowserHub()}
	pair := func(actor string) (string, string) {
		t.Helper()
		code, err := s.browserHub.PairCode(actor)
		if err != nil {
			t.Fatal(err)
		}
		session, token, err := s.browserHub.Pair(code, "offgrid-demo://research", computer.ProtocolVersion)
		if err != nil {
			t.Fatal(err)
		}
		return session.ID, token
	}
	old, oldToken := pair("local-admin")
	s.browserHub.StopSession("local-admin", old)
	current, currentToken := pair("local-admin")
	stop := func(body string, status int) {
		t.Helper()
		w := httptest.NewRecorder()
		s.handleComputerSessionStop(w, httptest.NewRequest("POST", "/api/v2/computer/sessions/stop", strings.NewReader(body)))
		if w.Code != status {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
	}
	for _, id := range []string{old, "unknown"} {
		stop(`{"session_id":"`+id+`"}`, 200)
	}
	for _, body := range []string{`{}`, `{"session_id":""}`, `{"session_id":"` + current + `","all":true}`, `{"session_id":"` + current + `"} {}`} {
		stop(body, 400)
	}
	if s.browserHub.Heartbeat(currentToken) != nil {
		t.Fatal("unrelated session revoked")
	}
	stop(`{"session_id":"`+current+`"}`, 200)
	stop(`{"session_id":"`+current+`"}`, 200)
	if s.browserHub.Heartbeat(currentToken) == nil || s.browserHub.Heartbeat(oldToken) == nil {
		t.Fatal("incorrect revocation scope")
	}
	foreign, foreignToken := pair("another-admin")
	stop(`{"session_id":"`+foreign+`"}`, 200)
	if s.browserHub.Heartbeat(foreignToken) != nil {
		t.Fatal("foreign session revoked")
	}
}
