package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestDocumentRetentionRoutesAuthorityHoldAndExpiry(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	store, _ := e.ImportStore()
	doc := &rag.Document{ID: "retained", Name: "record.txt", RawContent: "private record", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.AddDocument(doc); err != nil {
		t.Fatal(err)
	}
	accounts := users.NewUserStore("")
	keys, ids := map[string]string{}, map[string]string{}
	for _, name := range []string{"owner", "editor", "outsider", "reader"} {
		role := users.RoleAdmin
		if name == "reader" {
			role = users.RoleUser
		}
		u, k, err := accounts.CreateUser(name, "isolated-password", role)
		if err != nil {
			t.Fatal(err)
		}
		keys[name], ids[name] = k, u.ID
	}
	p, err := store.CreateCollection(ids["owner"], "Records", []string{ids["reader"]}, []string{ids["editor"]})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.AssignDocumentCollection(ids["owner"], doc.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: e}
	mux := http.NewServeMux()
	guard := func(permission users.Permission) func(http.HandlerFunc) http.HandlerFunc {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return users.RequirePermission(permission)(next).ServeHTTP
		}
	}
	s.registerKnowledgeRoutes(mux, routeGuards{ragOnly: guard(users.PermissionRAG), ragManagerOnly: guard(users.PermissionRAGManage)})
	auth := users.NewMiddleware(accounts)
	auth.SetRequireAuth(true)
	handler := auth.Wrap(mux)
	do := func(actor, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if keys[actor] != "" {
			r.Header.Set("Authorization", "Bearer "+keys[actor])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	path := "/v1/documents/retention?id=retained"
	for _, actor := range []string{"owner", "editor", "reader"} {
		if w := do(actor, "GET", path, ""); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal(actor, w.Code, w.Body.String())
		}
	}
	if w := do("outsider", "GET", path, ""); w.Code != 404 {
		t.Fatal("private retention leaked", w.Code)
	}
	if w := do("", "GET", path, ""); w.Code != 401 {
		t.Fatal("anonymous retention", w.Code)
	}
	body := `{"version":1,"revision":1,"delete_after":1,"legal_hold":true}`
	for _, actor := range []string{"reader", "editor", "outsider"} {
		w := do(actor, "PUT", path, body)
		if w.Code != 403 && w.Code != 404 {
			t.Fatal("unauthorized retention update", actor, w.Code)
		}
	}
	for _, body := range []string{`{"version":1,"revision":1}`, `{"version":99,"revision":1,"delete_after":0,"legal_hold":false}`, `{"version":1,"revision":1,"delete_after":0,"legal_hold":false,"actor":"owner"}`, `{"version":1,"revision":1,"delete_after":0,"legal_hold":false} {}`} {
		if w := do("owner", "PUT", path, body); w.Code != 400 {
			t.Fatal("malformed update", w.Code)
		}
	}
	w := do("owner", "PUT", path, body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var policy rag.DocumentRetention
	if err = json.Unmarshal(w.Body.Bytes(), &policy); err != nil || policy.Revision != 2 || !policy.LegalHold {
		t.Fatal(policy, err)
	}
	if w = do("owner", "PUT", path, body); w.Code != 409 {
		t.Fatal("stale update", w.Code)
	}
	if w = do("editor", "DELETE", "/v1/documents/delete?id=retained", ""); w.Code != 409 || !strings.Contains(w.Body.String(), "document_held") {
		t.Fatal("hold was concealed or bypassed", w.Code, w.Body.String())
	}
	if n, err := e.ExpireDocuments(time.Now(), 100); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if w = do("owner", "PUT", path, `{"version":1,"revision":2,"delete_after":1,"legal_hold":false}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if n, err := e.ExpireDocuments(time.Now(), 100); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	for _, path := range []string{path, "/v1/documents/source?id=retained", "/v1/documents/source?id=retained&format=original", "/v1/documents/source?id=retained&format=preview", "/v1/documents/source?id=retained&format=preview&page=1"} {
		if w = do("owner", "GET", path, ""); w.Code != 404 {
			t.Fatal("expired source exposed", w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), doc.RawContent) {
			t.Fatal("expired source denial leaked content or allowed caching", path)
		}
	}
}
