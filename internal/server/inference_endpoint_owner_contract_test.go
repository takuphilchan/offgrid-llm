package server

import (
	"encoding/json"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestEndpointOwnerRecoveryContractSurvivesUnavailablePolicy(t *testing.T) {
	s := endpointOwnerFixture(t)
	s.endpointOwner.mu.Lock()
	s.blockEndpointOwner()
	s.endpointOwner.mu.Unlock()
	assertEndpointOwnerRecoveryDiscoverable(t, s)
	withoutOwner := &Server{}
	response := httptest.NewRecorder()
	withoutOwner.handleSystemIdentity(response, httptest.NewRequest("GET", "/api/v2/system", nil))
	var identity SystemIdentity
	if json.Unmarshal(response.Body.Bytes(), &identity) != nil || slices.Contains(identity.Capabilities, "inference-endpoint-owner-v1") {
		t.Fatal("uninitialized owner advertised a protocol", response.Body.String())
	}
}

func assertEndpointOwnerRecoveryDiscoverable(t *testing.T, s *Server) {
	t.Helper()
	management := endpointManagementRequestMethod(s, "", "", nil, "GET")
	if management.Code != 503 {
		t.Fatal("fixture did not block ordinary endpoint management", management.Code, management.Body.String())
	}
	response := httptest.NewRecorder()
	s.handleSystemIdentity(response, httptest.NewRequest("GET", "/api/v2/system", nil))
	var identity SystemIdentity
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &identity) != nil || !slices.Contains(identity.Capabilities, "inference-endpoint-owner-v1") {
		t.Fatal("failed policy hid the recovery protocol", response.Code, response.Body.String())
	}
	state := endpointOwnerSnapshotFixture(t, s, "")
	if state.WriteAvailable || state.WriteReason == "" {
		t.Fatal("protocol support became write authority", state)
	}
}
