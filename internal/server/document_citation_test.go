package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestDocumentCitationReadbackRequiresExactRevisionAndPermission(t *testing.T) {
	dir := t.TempDir()
	engine := rag.NewEngine(nil, dir)
	defer engine.Close()
	store, err := rag.NewSQLiteStore(filepath.Join(dir, "rag"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	prefix := strings.Repeat("prefix ", 30000)
	passage := "Exact café 日本語"
	digest := strings.Repeat("a", 64)
	doc := &rag.Document{ID: "citation", Name: "source.pdf", RawContent: prefix + passage, CreatedAt: time.Now(), UpdatedAt: time.Now(), SourceRanges: []rag.SourceRange{{Kind: "page", Index: 2, Start: len(prefix), End: len(prefix) + len(passage), SHA256: digest, Revision: "fixture"}}}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
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
	s := &Server{ragEngine: engine}
	handler := middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(s.handleDocumentSource)))
	query := url.Values{"id": {doc.ID}, "start_byte": {strconv.Itoa(len(prefix))}, "end_byte": {strconv.Itoa(len(doc.RawContent))}, "source_sha256": {digest}, "extractor_revision": {"fixture"}, "content_hash": {rag.GenerateContentHash(passage)}}
	read := func(token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/v1/documents/source?"+query.Encode(), nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := read(key)
	var body struct {
		Content   string
		Verified  bool `json:"citation_verified"`
		Truncated bool
	}
	if err = json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 200 || body.Content != passage || !body.Verified || body.Truncated || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	for _, token := range []string{"", guest} {
		if w := read(token); w.Code != 401 && w.Code != 403 {
			t.Fatal("unauthorized citation", w.Code)
		}
	}
	query.Set("extractor_revision", "old")
	if w := read(key); w.Code != 409 || strings.Contains(w.Body.String(), passage) {
		t.Fatal("stale citation leaked content", w.Code, w.Body.String())
	}
	query.Del("content_hash")
	if w := read(key); w.Code != 400 {
		t.Fatal("partial locator accepted", w.Code)
	}
	query.Set("content_hash", rag.GenerateContentHash(passage))
	query.Set("extractor_revision", "fixture")
	query.Set("format", "original")
	if w := read(key); w.Code != 400 {
		t.Fatal("original bypass accepted", w.Code)
	}
}
