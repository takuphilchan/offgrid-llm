package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestCollectionHTTPPrivateDefaultsRevisionAndRevocation(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	s := &Server{ragEngine: e}
	accounts := users.NewUserStore("")
	ids, keys := map[string]string{}, map[string]string{}
	for _, name := range []string{"owner", "reader", "editor", "outsider", "guest"} {
		role := users.RoleAdmin
		if name == "guest" {
			role = users.RoleGuest
		}
		u, key, err := accounts.CreateUser(name, "isolated-password", role)
		if err != nil {
			t.Fatal(err)
		}
		ids[name], keys[name] = u.ID, key
	}
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	mux := http.NewServeMux()
	guard := func(permission users.Permission) func(http.HandlerFunc) http.HandlerFunc {
		return func(next http.HandlerFunc) http.HandlerFunc {
			return users.RequirePermission(permission)(next).ServeHTTP
		}
	}
	s.registerKnowledgeRoutes(mux, routeGuards{ragOnly: guard(users.PermissionRAG), ragManagerOnly: guard(users.PermissionRAGManage)})
	handler := middleware.Wrap(mux)
	do := func(actor, method, path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		if actor != "" {
			r.Header.Set("Authorization", "Bearer "+keys[actor])
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := do("owner", "POST", "/v1/collections", map[string]any{"name": "Department records"})
	var p rag.CollectionPolicy
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &p) != nil || p.Owner != ids["owner"] || len(p.Readers) != 0 || len(p.Editors) != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	path := "/v1/collections?id=" + url.QueryEscape(p.ID)
	for _, actor := range []string{"outsider", "guest", ""} {
		w = do(actor, "GET", path, nil)
		if w.Code != 404 && w.Code != 403 && w.Code != 401 {
			t.Fatal("private policy disclosed", actor, w.Code)
		}
	}
	update := rag.CollectionUpdate{ExpectedRevision: 1, Name: p.Name, Readers: []string{ids["reader"]}, Editors: []string{ids["editor"]}}
	w = do("owner", "PUT", path, update)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = do("owner", "PUT", path, update); w.Code != 409 {
		t.Fatal("stale sharing accepted", w.Code)
	}
	update.ExpectedRevision = 2
	if w = do("editor", "PUT", path, update); w.Code != 404 {
		t.Fatal("editor changed membership", w.Code)
	}
	if w = do("owner", "PUT", path, map[string]any{"expected_revision": 2, "name": p.Name}); w.Code != 400 {
		t.Fatal("missing lists silently revoked access", w.Code)
	}
	if w = do("owner", "POST", "/v1/collections", map[string]any{"name": "Spoof", "owner": ids["outsider"]}); w.Code != 400 {
		t.Fatal("client supplied owner", w.Code)
	}
	store, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	listener := httptest.NewServer(handler)
	defer listener.Close()
	client, err := serviceclient.New(listener.URL, keys["owner"], nil)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "private.txt")
	if err = os.WriteFile(file, []byte("Private original never published to workspace"), 0600); err != nil {
		t.Fatal(err)
	}
	accepted, err := cli.RunKnowledgeService(context.Background(), client, []string{"import-collection", p.ID, "2", file}, nil, io.Discard, true)
	if err != nil {
		t.Fatal("reviewed CLI import", err)
	}
	importID, ok := accepted.(map[string]any)["id"].(string)
	if !ok {
		t.Fatal("missing import receipt")
	}
	retained, err := store.Import(ids["owner"], importID)
	if err != nil || retained.CollectionID != p.ID || retained.CollectionRevision != 2 {
		t.Fatal("destination not retained", retained, err)
	}
	if _, err = store.Import(ids["outsider"], importID); err == nil {
		t.Fatal("pending original exposed to admin outsider")
	}
	if err = store.AddDocument(&rag.Document{ID: "private-policy", Name: "Secret", RawContent: "Protected evidence"}); err != nil {
		t.Fatal(err)
	}
	move := rag.CollectionMove{DocumentID: "private-policy", ExpectedRevision: 1, ExpectedSourceRevision: 1, DestinationID: p.ID, ExpectedDestinationRevision: 2}
	if w = do("owner", "PUT", "/v1/documents/collection", move); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = do("reader", "GET", "/v1/documents/source?id=private-policy", nil); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = do("reader", "GET", "/v1/documents", nil)
	var listing struct {
		Documents []rag.Document `json:"documents"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &listing) != nil || len(listing.Documents) != 1 || listing.Documents[0].CollectionID != p.ID || listing.Documents[0].CollectionRevision != 2 {
		t.Fatal("authorized list lost collection binding", w.Code, w.Body.String())
	}
	if w = do("outsider", "GET", "/v1/documents", nil); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(p.ID)) || bytes.Contains(w.Body.Bytes(), []byte("private-policy")) {
		t.Fatal("document projection disclosed private collection", w.Code, w.Body.String())
	}
	if w = do("outsider", "GET", "/v1/documents/source?id=private-policy", nil); w.Code != 404 {
		t.Fatal("admin bypassed source policy", w.Code)
	}
	if w = do("outsider", "GET", "/v1/collections", nil); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(p.ID)) {
		t.Fatal("collection list leaked", w.Body.String())
	}
	update.Readers = []string{}
	if w = do("owner", "PUT", path, update); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = do("reader", "GET", "/v1/documents", nil); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(p.ID)) || bytes.Contains(w.Body.Bytes(), []byte("private-policy")) {
		t.Fatal("revoked list retained private collection", w.Code, w.Body.String())
	}
	if w = do("owner", "POST", "/v1/documents/imports?id="+importID+"&action=retry", nil); w.Code != 409 {
		t.Fatal("stale reviewed audience executed import", w.Code, w.Body.String())
	}
	if w = do("owner", "POST", "/v1/documents/imports?id="+importID+"&action=cancel", nil); w.Code != 200 {
		t.Fatal("cancel prevented after audience change", w.Code, w.Body.String())
	}
	for _, suffix := range []string{"/v1/documents/source?id=private-policy", "/v1/documents/collection?id=private-policy"} {
		if w = do("reader", "GET", suffix, nil); w.Code != 404 {
			t.Fatal("revocation ineffective", w.Code)
		}
	}
	if w = do("owner", "GET", path+"&view=audit", nil); w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("Protected evidence")) || !bytes.Contains(w.Body.Bytes(), []byte("document_moved")) {
		t.Fatal("audit missing or leaks content", w.Body.String())
	}
	if w = do("editor", "GET", path+"&view=audit", nil); w.Code != 404 {
		t.Fatal("audit exposed beyond owner", w.Code)
	}
}
