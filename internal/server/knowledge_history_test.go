package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestKnowledgeHistoryRechecksCollectionForOwnerAndSessionsAdmin(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	store, _ := e.ImportStore()
	accounts := users.NewUserStore("")
	owner, ownerKey, err := accounts.CreateUser("owner", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	admin, adminKey, err := accounts.CreateUser("admin", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	doc := &rag.Document{ID: "private", Name: "confidential.pdf", RawContent: "secret passage", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.AddDocument(doc); err != nil {
		t.Fatal(err)
	}
	p, err := store.CreateCollection(admin.ID, "Private", []string{owner.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AssignDocumentCollection(admin.ID, doc.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: e, userStore: accounts, config: &config.Config{RequireAuth: true}}
	h := NewSessionHandlers(t.TempDir())
	h.requireAuth, h.knowledgeAccess = true, s.savedKnowledgeAllowed
	called := false
	h.completer = func(context.Context, string, []api.ChatMessage, bool) (string, error) {
		called = true
		return "wrong", nil
	}
	seed := h.manager.WithAccess(sessions.Access{UserID: owner.ID})
	if _, err := seed.Create("private-chat", "model"); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.AppendExchange("private-chat", "model", "question", "secret answer", []sessions.KnowledgePassage{{DocumentID: doc.ID, Content: doc.RawContent}}); err != nil {
		t.Fatal(err)
	}
	mw := users.NewMiddleware(accounts)
	mw.SetRequireAuth(true)
	handler := mw.Wrap(http.HandlerFunc(h.HandleSessions))
	do := func(key, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := do(ownerKey, "GET", "/v1/sessions/private-chat", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "secret answer") {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := store.UpdateCollection(admin.ID, p.ID, p.Revision, p.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path, body string }{
		{"GET", "/v1/sessions/private-chat", ""},
		{"POST", "/v1/sessions/private-chat/generate", `{"content":"repeat the answer"}`},
		{"GET", "/v1/sessions/private-chat/turn", ""},
	} {
		w := do(ownerKey, request.method, request.path, request.body)
		if w.Code != 403 || strings.Contains(w.Body.String(), "secret") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("revoked history leaked", w.Code, w.Body.String())
		}
	}
	if called {
		t.Fatal("revoked history sent to model")
	}
	if w := do(ownerKey, "GET", "/v1/sessions", ""); w.Code != 200 || strings.Contains(w.Body.String(), "private-chat") {
		t.Fatal("list leaked", w.Code, w.Body.String())
	}
	// sessions:all alone is insufficient: move the document to another owner's collection.
	other, _, err := accounts.CreateUser("other", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	q, err := store.CreateCollection(other.ID, "Other", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Give admin the destination permission for the move, then revoke it.
	q, err = store.UpdateCollection(other.ID, q.ID, q.Revision, q.Name, nil, []string{admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AssignDocumentCollection(admin.ID, doc.ID, q.ID, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateCollection(other.ID, q.ID, q.Revision, q.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	if w := do(adminKey, "GET", "/v1/sessions/private-chat", ""); w.Code != 403 {
		t.Fatal("administrator bypass", w.Code, w.Body.String())
	}
	if w := do(ownerKey, "DELETE", "/v1/sessions/private-chat", ""); w.Code != 200 {
		t.Fatal("owner cannot delete", w.Code, w.Body.String())
	}
}
