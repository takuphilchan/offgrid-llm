package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestDocumentImportAuthenticatedRecoveryAndCLI(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	s := &Server{ragEngine: e}
	store := users.NewUserStore("")
	_, key, err := store.CreateUser("alice", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := store.CreateUser("bob", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, reader, err := store.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	auth := users.NewMiddleware(store)
	auth.SetRequireAuth(true)
	mux := http.NewServeMux()
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		return users.RequirePermission(users.PermissionRAGManage)(next).ServeHTTP
	}
	s.registerKnowledgeRoutes(mux, routeGuards{ragManagerOnly: guard, ragOnly: guard})
	handler := auth.Wrap(mux)
	request := func(token, method, query string, body *http.Request) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/v1/documents/imports"+query, nil)
		if body != nil {
			r.Body = body.Body
			r.Header = body.Header.Clone()
			r.Header.Set("Idempotency-Key", "same-upload")
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, token := range []string{"", reader} {
		w := request(token, "GET", "", nil)
		if w.Code != 401 && w.Code != 403 {
			t.Fatal("unauthorized import listing", w.Code)
		}
	}
	w := request(key, "POST", "", documentUploadRequest(t, "broken.docx", []byte("invalid ZIP original")))
	if w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	var r rag.ImportRecord
	if err = json.Unmarshal(w.Body.Bytes(), &r); err != nil || r.State != "accepted" {
		t.Fatal("acceptance not persisted", err)
	}
	if bytes.Contains(w.Body.Bytes(), []byte("invalid ZIP original")) {
		t.Fatal("original leaked")
	}
	again := request(key, "POST", "", documentUploadRequest(t, "broken.docx", []byte("invalid ZIP original")))
	var repeated rag.ImportRecord
	_ = json.Unmarshal(again.Body.Bytes(), &repeated)
	if again.Code != 202 || repeated.ID != r.ID {
		t.Fatal("duplicate acceptance")
	}
	if changed := request(key, "POST", "", documentUploadRequest(t, "broken.docx", []byte("different bytes"))); changed.Code != 409 {
		t.Fatal("changed identity accepted", changed.Code)
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		w = request(other, method, "?id="+r.ID+"&action=retry", nil)
		if w.Code != 404 {
			t.Fatal("cross-user import operation", method, w.Code)
		}
	}
	w = request(key, "POST", "?id="+r.ID+"&action=retry", nil)
	if w.Code != 422 {
		t.Fatal("invalid parse should fail", w.Code, w.Body.String())
	}
	w = request(key, "GET", "?id="+r.ID, nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &r) != nil || r.State != "needs_input" || r.Attempt != 1 {
		t.Fatal("failed import not recoverable", w.Body.String())
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, _ := serviceclient.New(server.URL, key, server.Client())
	for _, args := range [][]string{{"imports"}, {"import-status", r.ID}, {"cancel-import", r.ID}, {"remove-import", r.ID}} {
		if _, err = cli.RunKnowledgeService(context.Background(), client, args, nil, io.Discard, true); err != nil {
			t.Fatal(args, err)
		}
	}
	if w = request(key, "GET", "?id="+r.ID, nil); w.Code != 404 {
		t.Fatal("removed import remains readable")
	}
	if docs := e.ListDocuments(); len(docs) != 0 {
		t.Fatal("failed import published", docs)
	}
}
