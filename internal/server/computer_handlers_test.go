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
