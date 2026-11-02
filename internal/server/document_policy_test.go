package server

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/takuphilchan/offgrid-llm/internal/storage"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestDocumentCollectionPolicyProtectsHTTPReadsAndDeletion(t *testing.T) {
	dir := t.TempDir()
	engine := rag.NewEngine(nil, dir)
	defer engine.Close()
	store, err := engine.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	accounts := users.NewUserStore("")
	ids, keys := map[string]string{}, map[string]string{}
	for _, actor := range []string{"owner", "reader", "editor", "outsider", "guest"} {
		role := users.RoleAdmin
		if actor == "guest" {
			role = users.RoleGuest
		}
		u, key, err := accounts.CreateUser(actor, "isolated-password", role)
		if err != nil {
			t.Fatal(err)
		}
		ids[actor], keys[actor] = u.ID, key
	}
	data := []byte("Private café 日本語 source")
	h := sha256.Sum256(data)
	digest := hex.EncodeToString(h[:])
	doc := &rag.Document{ID: rag.GenerateDocumentID(data), Name: "private.pdf", RawContent: string(data), CreatedAt: time.Now(), UpdatedAt: time.Now(), ChunkCount: 1, IndexStatus: "ready",
		SourceRanges: []rag.SourceRange{{Kind: "page", Index: 1, Start: 0, End: len(data), SHA256: digest, Revision: "test-parser"}}}
	chunk := &rag.Chunk{ID: "private-chunk", DocumentID: doc.ID, Content: doc.RawContent, CreatedAt: time.Now()}
	if err := store.AddDocumentWithChunks(doc, []*rag.Chunk{chunk}, [][]float32{{1, 0}}); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenSQLite(filepath.Join(dir, "rag", "rag.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO document_originals VALUES(?,1,?,?,?,?,?)`, doc.ID, digest, len(data), "application/pdf", doc.Name, data); err != nil {
		t.Fatal(err)
	}
	p, err := store.CreateCollection(ids["owner"], "Private records", []string{ids["reader"], ids["guest"]}, []string{ids["editor"]})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AssignDocumentCollection(ids["owner"], doc.ID, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: engine}
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	do := func(actor, method, path string, handler http.HandlerFunc, permission users.Permission) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		if actor != "" {
			r.Header.Set("Authorization", "Bearer "+keys[actor])
		}
		w := httptest.NewRecorder()
		middleware.Wrap(users.RequirePermission(permission)(handler)).ServeHTTP(w, r)
		return w
	}
	citation := url.Values{"id": {doc.ID}, "start_byte": {"0"}, "end_byte": {strconv.Itoa(len(data))}, "source_sha256": {digest}, "extractor_revision": {"test-parser"}, "content_hash": {rag.GenerateContentHash(doc.RawContent)}}
	paths := []string{"/v1/documents/source?id=" + doc.ID, "/v1/documents/source?id=" + doc.ID + "&format=original", "/v1/documents/source?" + citation.Encode()}
	for _, path := range paths {
		for _, actor := range []string{"owner", "reader", "editor"} {
			w := do(actor, "GET", path, s.handleDocumentSource, users.PermissionRAG)
			if w.Code != 200 || !strings.Contains(w.Body.String(), doc.RawContent) {
				t.Fatal("allowed source unavailable", actor, w.Code, w.Body.String())
			}
		}
		for _, actor := range []string{"outsider", "guest", ""} {
			w := do(actor, "GET", path, s.handleDocumentSource, users.PermissionRAG)
			if w.Code != 404 && w.Code != 403 && w.Code != 401 {
				t.Fatal("source bypass", actor, w.Code)
			}
			if strings.Contains(w.Body.String(), doc.RawContent) || strings.Contains(w.Body.String(), doc.Name) || w.Header().Get("X-OffGrid-Source-SHA256") != "" {
				t.Fatal("private source leaked")
			}
			if actor == "outsider" && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("policy denial was cacheable")
			}
		}
	}
	for _, actor := range []string{"reader", "outsider"} {
		w := do(actor, "GET", "/v1/documents", s.handleDocumentsList, users.PermissionRAG)
		var list DocumentListResponse
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != 200 {
			t.Fatal(w.Code, err)
		}
		want := 0
		if actor == "reader" {
			want = 1
		}
		if len(list.Documents) != want || list.Count != want {
			t.Fatal("unscoped library", actor, list)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("scoped list was cacheable")
		}
		w = do(actor, "GET", "/v1/rag/status", s.handleRAGStatus, users.PermissionRAG)
		var status RAGStatusResponse
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.Stats["document_count"] != float64(want) {
			t.Fatal("unscoped counts", actor, w.Body.String(), err)
		}
		w = do(actor, "DELETE", "/v1/documents/delete?id="+doc.ID, s.handleDocumentDelete, users.PermissionRAGManage)
		if w.Code != 404 {
			t.Fatal("global admin permission bypassed collection editing", actor, w.Code)
		}
	}
	if _, err := store.UpdateCollection(ids["owner"], p.ID, 1, p.Name, nil, p.Editors); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if w := do("reader", "GET", path, s.handleDocumentSource, users.PermissionRAG); w.Code != 404 {
			t.Fatal("revoked source still readable", w.Code)
		}
	}
	if w := do("editor", "DELETE", "/v1/documents/delete?id="+doc.ID, s.handleDocumentDelete, users.PermissionRAGManage); w.Code != 200 {
		t.Fatal("authorized deletion failed", w.Code, w.Body.String())
	}
	if w := do("owner", "GET", paths[1], s.handleDocumentSource, users.PermissionRAG); w.Code != 404 {
		t.Fatal("deleted source returned", w.Code)
	}
}
