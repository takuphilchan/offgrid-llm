package server

import (
	"context"
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

func TestAuthenticatedSessionIsolation(t *testing.T) {
	store := users.NewUserStore("")
	keys := map[string]string{}
	ids := map[string]string{}
	for name, role := range map[string]users.Role{"alice": users.RoleUser, "bob": users.RoleUser, "admin": users.RoleAdmin, "viewer": users.RoleViewer} {
		user, key, err := store.CreateUser(name, "test-password", role)
		if err != nil {
			t.Fatal(err)
		}
		keys[name], ids[name] = key, user.ID
	}
	h := NewSessionHandlers(t.TempDir())
	h.requireAuth = true
	calls := 0
	h.SetCompleter(func(context.Context, string, []api.ChatMessage, bool) (string, error) { calls++; return "answer", nil })
	h.streamer = func(context.Context, *api.ChatCompletionRequest, string, func(ChatStreamEvent) error) (ChatStreamResult, error) {
		calls++
		return ChatStreamResult{Answer: "answer", FinishReason: "stop"}, nil
	}
	if err := h.manager.Save(sessions.NewSession("legacy", "model")); err != nil {
		t.Fatal(err)
	}
	middleware := users.NewMiddleware(store)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(http.HandlerFunc(h.HandleSessions))
	do := func(user, method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			req.Header.Set("Authorization", "Bearer "+keys[user])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	created := do("alice", "POST", "/v1/sessions", `{"name":"private","model_id":"model","owner_id":"forged"}`)
	if created.Code != 201 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var session sessions.Session
	if err := json.Unmarshal(created.Body.Bytes(), &session); err != nil || session.OwnerID != ids["alice"] {
		t.Fatalf("owner: %+v %v", session, err)
	}
	for _, test := range []struct {
		user, method, path, body string
		status                   int
	}{
		{"", "GET", "/v1/sessions", "", 401},
		{"viewer", "GET", "/v1/sessions", "", 403},
		{"bob", "GET", "/v1/sessions/private", "", 404},
		{"bob", "DELETE", "/v1/sessions/private", "", 404},
		{"bob", "POST", "/v1/sessions/private/messages", `{"role":"user","content":"bad"}`, 404},
		{"bob", "POST", "/v1/sessions/private/generate", `{"content":"bad"}`, 404},
		{"bob", "POST", "/v1/sessions/private/generate", `{"content":"bad","stream":true}`, 404},
		{"bob", "POST", "/v1/sessions", `{"name":"private","model_id":"model"}`, 409},
		{"alice", "POST", "/v1/sessions", `{"name":"private","model_id":"model"}`, 409},
		{"alice", "GET", "/v1/sessions/legacy", "", 404},
		{"admin", "GET", "/v1/sessions/legacy", "", 200},
		{"admin", "GET", "/v1/sessions/private", "", 200},
	} {
		w := do(test.user, test.method, test.path, test.body)
		if w.Code != test.status {
			t.Errorf("%s %s %s: %d %s", test.user, test.method, test.path, w.Code, w.Body.String())
		}
	}
	if calls != 0 {
		t.Fatalf("unauthorized inference executed %d times", calls)
	}
	for user, count := range map[string]int{"alice": 1, "bob": 0, "admin": 2} {
		w := do(user, "GET", "/v1/sessions", "")
		var result struct {
			Sessions []sessions.Session `json:"sessions"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || len(result.Sessions) != count {
			t.Fatalf("%s list: %d %s %v", user, w.Code, w.Body.String(), err)
		}
	}
	if w := do("alice", "POST", "/v1/sessions/private/generate", `{"content":"hello"}`); w.Code != 200 {
		t.Fatalf("owner generation: %d %s", w.Code, w.Body.String())
	}
	if calls != 1 {
		t.Fatalf("owner inference calls: %d", calls)
	}
	if len(h.sessionLocks) != 0 {
		t.Fatalf("retained idle session locks: %d", len(h.sessionLocks))
	}
}

func TestKnowledgeAuthorizationAppliesToAllChatTransports(t *testing.T) {
	store := users.NewUserStore("")
	keys := map[users.Role]string{}
	for _, role := range []users.Role{users.RoleGuest, users.RoleViewer, users.RoleUser} {
		_, key, err := store.CreateUser(string(role), "test-password", role)
		if err != nil {
			t.Fatal(err)
		}
		keys[role] = key
	}
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	// No inference engine/registry: an authorization regression must not reach them.
	server := &Server{config: &config.Config{RequireAuth: true}}
	for _, role := range []users.Role{users.RoleGuest, users.RoleViewer} {
		for _, stream := range []bool{false, true} {
			body, _ := json.Marshal(api.ChatCompletionRequest{Model: "model", Messages: []api.ChatMessage{{Role: "user", Content: "private documents"}}, Stream: stream, UseKnowledgeBase: boolPointer(true)})
			req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(string(body)))
			req.Header.Set("Authorization", "Bearer "+keys[role])
			w := httptest.NewRecorder()
			auth.Wrap(http.HandlerFunc(server.handleChatCompletions)).ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Fatalf("role=%s stream=%v: %d %s", role, stream, w.Code, w.Body.String())
			}
		}
	}
	// Exercise the service authorization directly, not just the route wrapper.
	for _, role := range []users.Role{users.RoleGuest, users.RoleViewer, users.RoleUser} {
		req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
		req.Header.Set("Authorization", "Bearer "+keys[role])
		auth.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := authorizeChatAccess(r.Context(), true, false); err != nil {
				t.Fatalf("ordinary chat denied: %v", err)
			}
			err := server.authorizeChat(r.Context(), &api.ChatCompletionRequest{UseKnowledgeBase: boolPointer(true)})
			if (err == nil) != (role == users.RoleUser) {
				t.Fatalf("RAG authorization for %s: %v", role, err)
			}
			if role != users.RoleUser {
				_, err := server.completeChat(r.Context(), &api.ChatCompletionRequest{
					Model: "model", Messages: []api.ChatMessage{{Role: "user", Content: "private documents"}}, UseKnowledgeBase: boolPointer(true),
				})
				denied := httptest.NewRecorder()
				if err == nil {
					t.Fatal("service allowed unauthorized knowledge")
				}
				writeServiceError(denied, err)
				if denied.Code != http.StatusForbidden {
					t.Fatalf("service authorization: %d %s", denied.Code, denied.Body.String())
				}
			}
		})).ServeHTTP(httptest.NewRecorder(), req)
	}
}

func boolPointer(value bool) *bool { return &value }
