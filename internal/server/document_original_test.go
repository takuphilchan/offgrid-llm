package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestDocumentOriginalDownloadAndLegacy(t *testing.T) {
	dir := t.TempDir()
	engine := rag.NewEngine(nil, dir)
	defer engine.Close()
	store, err := rag.NewSQLiteStore(filepath.Join(dir, "rag"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	data := []byte("<script>private original café 日本語</script>")
	doc := &rag.Document{ID: rag.GenerateDocumentID(data), Name: "source.html", RawContent: "private extracted source", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: engine}
	userStore := users.NewUserStore("")
	_, key, err := userStore.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	_, guest, err := userStore.CreateUser("guest", "isolated-password", users.RoleGuest)
	if err != nil {
		t.Fatal(err)
	}
	middleware := users.NewMiddleware(userStore)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(s.handleDocumentSource)))
	read := func(token, format string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/v1/documents/source?id="+doc.ID+"&format="+format, nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if w := read(key, "original"); w.Code != 410 {
		t.Fatal("legacy original fabricated", w.Code)
	}
	if w := read(key, "text"); w.Code != 200 {
		t.Fatal("legacy text lost", w.Code)
	}
	db, err := storage.OpenSQLite(filepath.Join(dir, "rag", "rag.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := sha256.Sum256(data)
	if _, err = db.Exec(`INSERT INTO document_originals VALUES(?,1,?,?,?, ?,?)`, doc.ID, hex.EncodeToString(h[:]), len(data), "text/html", doc.Name, data); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", guest} {
		if w := read(token, "original"); w.Code != 401 && w.Code != 403 {
			t.Fatal("unauthorized original read", w.Code)
		}
	}
	w := read(key, "original")
	if w.Header().Get("X-OffGrid-Source-SHA256") != hex.EncodeToString(h[:]) {
		t.Fatal("original digest header missing or incorrect")
	}
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Type") != "application/octet-stream" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe or incorrect download: %d %v", w.Code, w.Header())
	}
	if w := read(key, "unknown"); w.Code != 400 {
		t.Fatal("unknown format accepted")
	}
	if _, err = db.Exec("UPDATE document_originals SET schema_version=99"); err != nil {
		t.Fatal(err)
	}
	if w := read(key, "original"); w.Code != 410 || bytes.Contains(w.Body.Bytes(), data) {
		t.Fatal("future source delivered")
	}
	if !engine.DeleteDocument(doc.ID) {
		t.Fatal("delete failed")
	}
	if w := read(key, "original"); w.Code != 404 {
		t.Fatal("deleted original available", w.Code)
	}
}
