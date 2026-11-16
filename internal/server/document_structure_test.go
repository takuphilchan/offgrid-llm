package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestStructureHTTPPaginationIdentityAndRevocation(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	store, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	accounts := users.NewUserStore("")
	owner, ownerKey, err := accounts.CreateUser("owner", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	reader, readerKey, err := accounts.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	text := "First paragraph.\nSecond paragraph.\n第三段。"
	doc := &rag.Document{ID: "policy", Name: "Policy", RawContent: text, SourceRanges: []rag.SourceRange{{Kind: "section", Index: 1, Start: 0, End: len(text), SHA256: strings.Repeat("a", 64), Revision: "test"}}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	p, err := store.CreateCollection(owner.ID, "Private", []string{reader.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.AssignDocumentCollection(owner.ID, doc.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: e}
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	handler := middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(s.handleDocumentStructure)))
	read := func(key, query string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/v1/documents/structure?id=policy&"+query, nil)
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	w := read(readerKey, "limit=1")
	var page document.StructurePage
	if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 || len(page.Blocks) != 1 || page.TotalBlocks != 3 || page.NextOffset == nil || *page.NextOffset != 1 || page.CanCorrect || page.Blocks[0].Editable || page.Revision != doc.ExtractionRevision || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	if page.Navigation == nil || page.NavigationOffset != 0 || page.Units[0].BlockCount != 3 {
		t.Fatal("missing retained navigation projection", page)
	}
	w = read(ownerKey, "revision="+page.Revision+"&offset=2&limit=1")
	if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 || page.NextOffset != nil || len(page.Blocks) != 1 || page.Blocks[0].Text != "第三段。" {
		t.Fatal(w.Code, w.Body.String(), err)
	}
	for _, q := range []string{"offset=-1", "limit=201", "unit=missing", "relationship_offset=-1", "navigation_offset=-1", "navigation_offset=200001", "navigation_offset=bad"} {
		if w := read(readerKey, q); w.Code != 400 {
			t.Fatal("invalid pagination accepted", q, w.Code)
		}
	}
	if _, err = store.UpdateCollection(owner.ID, p.ID, p.Revision, p.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	if w = read(readerKey, "revision="+doc.ExtractionRevision); w.Code != 404 || strings.Contains(w.Body.String(), "First paragraph") {
		t.Fatal("revoked graph returned", w.Code, w.Body.String())
	}
}

func TestCorrectionHTTPRejectsForgedAuthorAndInvalidEnvelope(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	s := &Server{ragEngine: e}
	for _, body := range []string{`{}`, `{"document_id":"x","expected_revision":"y","edits":[],"author":"owner"}`, `{"document_id":"x","expected_revision":"y","edits":[{}]} {}`, strings.Repeat(" ", 2<<20) + "{}"} {
		w := httptest.NewRecorder()
		s.handleDocumentCorrections(w, httptest.NewRequest("POST", "/v1/documents/corrections", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
