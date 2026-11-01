package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestWorkSearchAuthorizationAndQueryValidation(t *testing.T) {
	store := users.NewUserStore("")
	keys := map[string]string{}
	h := NewSessionHandlers(t.TempDir())
	h.requireAuth = true
	for name, role := range map[string]users.Role{"alice": users.RoleUser, "bob": users.RoleUser, "viewer": users.RoleViewer} {
		user, key, err := store.CreateUser(name, "test-password", role)
		if err != nil {
			t.Fatal(err)
		}
		keys[name] = key
		session := sessions.NewSession("demo-"+name, "model")
		session.OwnerID = user.ID
		session.Messages = []sessions.Message{{Content: "secret-content"}}
		if err := h.manager.Save(session); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{config: &config.Config{RequireAuth: true}, sessionHandlers: h}
	middleware := users.NewMiddleware(store)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(http.HandlerFunc(server.handleWorkSearch))
	for _, tc := range []struct {
		user, method, query string
		status              int
	}{{"alice", "GET", "demo", 200}, {"bob", "GET", "demo", 200}, {"viewer", "GET", "demo", 200}, {"", "GET", "demo", 401}, {"alice", "POST", "demo", 405}, {"alice", "GET", "a", 400}, {"alice", "GET", strings.Repeat("a", 257), 400}} {
		r := httptest.NewRequest(tc.method, "/api/v2/work/search?query="+tc.query, nil)
		if tc.user != "" {
			r.Header.Set("Authorization", "Bearer "+keys[tc.user])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%+v: %d %s", tc, w.Code, w.Body.String())
		}
		if tc.status == 200 {
			var result api.WorkSearchResponse
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			// Viewers retain Chat permission but cannot search session names.
			if tc.user == "viewer" {
				if len(result.Results) != 0 || len(result.Scopes) != 0 {
					t.Fatal(w.Body.String())
				}
				continue
			}
			if len(result.Results) != 1 || result.Results[0].ID != "demo-"+tc.user || strings.Contains(w.Body.String(), "secret-content") || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal(w.Body.String())
			}
		}
	}
}
