package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestAttachmentCollectionReviewedDestination(t *testing.T) {
	engine := rag.NewEngine(nil, t.TempDir())
	defer engine.Close()
	h := NewSessionHandlers(t.TempDir())
	defer h.closeTurns()
	h.requireAuth = true
	s := &Server{ragEngine: engine, sessionHandlers: h}
	accounts := users.NewUserStore("")
	owner, key, err := accounts.CreateUser("publisher", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	other, otherKey, err := accounts.CreateUser("other-admin", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	scope := h.manager.WithAccess(sessions.Access{UserID: owner.ID})
	if _, err = scope.Create("private", "model"); err != nil {
		t.Fatal(err)
	}
	original := []byte("Private policy exception: retain records for 12 years.\n")
	file, err := scope.CreateAttachment("private", "policy.txt", "text/plain", original)
	if err != nil {
		t.Fatal(err)
	}
	store, err := engine.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	collection, err := store.CreateCollection(owner.ID, "Private records", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/collections/imports", s.handleCollectionImport)
	mux.HandleFunc("/v1/documents/imports", s.handleDocumentImports)
	handler := middleware.Wrap(users.RequirePermission(users.PermissionRAGManage)(mux))
	data, _ := json.Marshal(map[string]any{"attachment": map[string]string{"session": "private", "id": file.ID, "sha256": file.SHA256}})
	request := func(token, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", path, bytes.NewReader(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	target := "/v1/collections/imports?" + url.Values{"collection_id": {collection.ID}, "collection_revision": {"1"}}.Encode()
	if w := request(key, "/v1/documents/imports"); w.Code != http.StatusUpgradeRequired {
		t.Fatal("legacy attachment silently shared", w.Code, w.Body.String())
	}
	for _, path := range []string{"/v1/collections/imports", "/v1/collections/imports?collection_id=" + collection.ID} {
		if w := request(key, path); w.Code != 400 {
			t.Fatal("unreviewed destination accepted", w.Code, w.Body.String())
		}
	}
	if w := request(otherKey, target); w.Code < 400 {
		t.Fatal("private attachment copied by outsider", w.Code)
	}
	accepted := request(key, target)
	var receipt rag.ImportRecord
	if accepted.Code != 202 || json.Unmarshal(accepted.Body.Bytes(), &receipt) != nil || receipt.CollectionID != collection.ID || receipt.CollectionRevision != 1 || receipt.SHA256 != file.SHA256 || receipt.State != "accepted" {
		t.Fatal(accepted.Code, accepted.Body.String())
	}
	repeated := request(key, target)
	var again rag.ImportRecord
	if repeated.Code != 202 || json.Unmarshal(repeated.Body.Bytes(), &again) != nil || again.ID != receipt.ID {
		t.Fatal("duplicate private copy", repeated.Code, repeated.Body.String())
	}
	if _, err = store.UpdateCollection(owner.ID, collection.ID, 1, collection.Name, []string{other.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if w := request(key, target); w.Code != 409 {
		t.Fatal("stale audience accepted", w.Code, w.Body.String())
	}
	if w := request(key, "/v1/documents/imports?id="+receipt.ID+"&action=retry"); w.Code != 409 {
		t.Fatal("unreviewed changed audience published", w.Code, w.Body.String())
	}
	if w := request(key, "/v1/documents/imports?id="+receipt.ID+"&action=cancel"); w.Code != 200 {
		t.Fatal("cannot cancel stale review", w.Code, w.Body.String())
	}
	// A different, explicitly reviewed destination is a distinct idempotency scope.
	workspace := request(key, "/v1/collections/imports?collection_id=workspace&collection_revision=1")
	var shared rag.ImportRecord
	if workspace.Code != 202 || json.Unmarshal(workspace.Body.Bytes(), &shared) != nil || shared.ID == receipt.ID || shared.CollectionID != "workspace" || shared.CollectionRevision != 1 {
		t.Fatal("destination collision", workspace.Code, workspace.Body.String())
	}
	if _, retained, err := scope.AttachmentOriginal("private", file.ID); err != nil || !bytes.Equal(retained, original) {
		t.Fatal("private original changed", err)
	}
}
