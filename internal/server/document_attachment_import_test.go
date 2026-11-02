package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestDocumentAttachmentImportScopeIdentityAndCLI(t *testing.T) {
	engine := rag.NewEngine(nil, t.TempDir())
	defer engine.Close()
	h := NewSessionHandlers(t.TempDir())
	defer h.closeTurns()
	h.requireAuth = true
	s := &Server{ragEngine: engine, sessionHandlers: h}
	us := users.NewUserStore("")
	owner, key, err := us.CreateUser("publisher", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, reader, err := us.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	scope := h.manager.WithAccess(sessions.Access{UserID: owner.ID})
	if _, err = scope.Create("private", "model"); err != nil {
		t.Fatal(err)
	}
	original := []byte("Exact café 日本語\nSecond line.\n")
	a, err := scope.CreateAttachment("private", "source.txt", "text/plain", original)
	if err != nil {
		t.Fatal(err)
	}
	mw := users.NewMiddleware(us)
	mw.SetRequireAuth(true)
	handler := mw.Wrap(users.RequirePermission(users.PermissionRAGManage)(http.HandlerFunc(s.handleDocumentImports)))
	body := func(digest string) []byte {
		data, _ := json.Marshal(map[string]any{"attachment": map[string]string{"session": "private", "id": a.ID, "sha256": digest}})
		return data
	}
	request := func(token string, data []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/v1/documents/imports", bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, token := range []string{"", reader} {
		w := request(token, body(a.SHA256))
		if w.Code != 401 && w.Code != 403 {
			t.Fatal("Knowledge permission bypass", w.Code)
		}
	}
	// Independently exercise the source guard with a reader who does have session
	// access but does not own this source, even without the outer import guard.
	readSource := mw.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, _, err := s.documentImportSource(r)
		if err == nil {
			t.Error("cross-owner source read")
		}
		w.WriteHeader(422)
	}))
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body(a.SHA256)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+reader)
	readSource.ServeHTTP(httptest.NewRecorder(), r)
	for _, data := range [][]byte{[]byte(`{"attachment":{"session":"private","id":"file-../","sha256":"x"}}`), append(body(a.SHA256), []byte(`{}`)...), []byte(strings.Repeat(" ", 8193)), []byte(`{"attachment":{"session":"private","id":"` + a.ID + `","sha256":"` + a.SHA256 + `","owner":"forged"}}`)} {
		if w := request(key, data); w.Code != 422 {
			t.Fatal("invalid reference accepted", w.Code, w.Body.String())
		}
	}
	if w := request(key, body(strings.Repeat("0", 64))); w.Code != 409 {
		t.Fatal("changed source accepted", w.Code)
	}
	w := request(key, body(a.SHA256))
	var record rag.ImportRecord
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &record) != nil || record.State != "accepted" || record.Attempt != 0 || record.SHA256 != a.SHA256 {
		t.Fatal("not accepted without execution", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), original) {
		t.Fatal("source text leaked in metadata")
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client, _ := serviceclient.New(server.URL, key, server.Client())
	result, err := cli.RunKnowledgeService(context.Background(), client, []string{"import-attachment", "private", a.ID, a.SHA256}, nil, io.Discard, true)
	if err != nil || result.(map[string]any)["id"] != record.ID {
		t.Fatal("CLI duplicated or lost source", result, err)
	}
	if _, _, err := scope.AttachmentOriginal("private", a.ID); err != nil {
		t.Fatal("publication moved private source", err)
	}
	if err := scope.DeleteAttachment("private", a.ID); err != nil {
		t.Fatal(err)
	}
	if w := request(key, body(a.SHA256)); w.Code != 422 {
		t.Fatal("deleted source reused", w.Code)
	}
	store, _ := engine.ImportStore()
	saved, err := store.Import(owner.ID, record.ID)
	if err != nil || saved.SHA256 != a.SHA256 || saved.State != "accepted" {
		t.Fatal("explicitly copied import lost after private source removal", saved, err)
	}
}
