package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
)

// Anonymous local mode is not a restricted guest workspace. Clients must know
// which mode is enforced before offering management actions.
func TestCurrentUserReportsAuthenticationEnforcement(t *testing.T) {
	for _, required := range []bool{false, true} {
		server := &Server{config: &config.Config{RequireAuth: required}}
		response := httptest.NewRecorder()
		server.handleCurrentUser(response, httptest.NewRequest("GET", "/v1/users/me", nil))
		var body struct {
			Required      *bool `json:"auth_required"`
			Authenticated bool  `json:"authenticated"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Required == nil || *body.Required != required || body.Authenticated {
			t.Fatalf("required=%v: unexpected identity %s", required, response.Body.String())
		}
	}
}
