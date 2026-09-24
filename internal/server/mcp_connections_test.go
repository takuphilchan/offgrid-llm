package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestMCPRemovalAPIRequiresAdminAndExactName(t *testing.T) {
	r := agents.NewToolRegistry()
	path := filepath.Join(t.TempDir(), "tools.json")
	if err := r.LoadUserTools(path); err != nil {
		t.Fatal(err)
	}
	name := "Docs & research / 日本語"
	if err := r.PersistMCPServer(agents.MCPServerConfig{Name: name, URL: "https://example.com/mcp"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{config: &config.Config{RequireAuth: true}, toolRegistry: r}
	store := users.NewUserStore("")
	_, userKey, err := store.CreateUser("member", "password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, adminKey, err := store.CreateUser("owner", "password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	middleware := users.NewMiddleware(store)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(s.requirePermissionWhenAuthEnabled(users.PermissionAdmin, s.handleAgentMCP))
	for _, tc := range []struct {
		key, query string
		status     int
	}{
		{"", "?name=" + url.QueryEscape(name), 401},
		{userKey, "?name=" + url.QueryEscape(name), 403},
		{adminKey, "", 400},
		{adminKey, "?name=" + url.QueryEscape(name), 200},
		{adminKey, "?name=" + url.QueryEscape(name), 200},
	} {
		request := httptest.NewRequest(http.MethodDelete, "/v1/agents/mcp"+tc.query, nil)
		if tc.key != "" {
			request.Header.Set("Authorization", "Bearer "+tc.key)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, request)
		if w.Code != tc.status {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if tc.status != 200 && len(r.GetMCPServers()) != 1 {
			t.Fatal("unauthorized/invalid request removed connection")
		}
	}
	if len(r.GetMCPServers()) != 0 {
		t.Fatal("connection not removed")
	}
	if err := os.WriteFile(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.handleAgentMCP(w, httptest.NewRequest(http.MethodDelete, "/v1/agents/mcp?name=docs", nil))
	if w.Code != 500 || !strings.Contains(w.Body.String(), "mcp_remove_failed") || strings.Contains(w.Body.String(), path) {
		t.Fatalf("unsafe or missing persistence error: %s", w.Body.String())
	}
}
