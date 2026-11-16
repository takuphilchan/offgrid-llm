package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// Called by the opt-in local embedding qualification path. No synthetic vectors
// or model answers are substituted; no runtime or model is downloaded.
func verifyRealCollectionImportHTTP(t *testing.T, s *Server, ctx context.Context) {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleCollections(w, httptest.NewRequest("POST", "/v1/collections", bytes.NewBufferString(`{"name":"Private procurement"}`)).WithContext(ctx))
	var collection rag.CollectionPolicy
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &collection) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	var upload bytes.Buffer
	form := multipart.NewWriter(&upload)
	part, err := form.CreateFormFile("file", "private-procurement.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte("Private Zirconium procurement deadline: 17 November 2026.")); err != nil {
		t.Fatal(err)
	}
	if err = form.Close(); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"collection_id": {collection.ID}, "collection_revision": {"1"}}
	r := httptest.NewRequest("POST", "/v1/collections/imports?"+query.Encode(), &upload).WithContext(ctx)
	r.Header.Set("Content-Type", form.FormDataContentType())
	w = httptest.NewRecorder()
	s.handleCollectionImport(w, r)
	var accepted rag.ImportRecord
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &accepted) != nil || accepted.CollectionID != collection.ID {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleDocumentImports(w, httptest.NewRequest("POST", "/v1/documents/imports?id="+url.QueryEscape(accepted.ID)+"&action=retry", nil).WithContext(ctx))
	var ready rag.ImportRecord
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &ready) != nil || ready.State != "ready" || ready.CollectionRevision != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	body, _ := json.Marshal(SearchRequest{Query: "Zirconium procurement deadline", TopK: 20, DocumentFilter: []string{ready.DocumentID}})
	w = httptest.NewRecorder()
	s.handleDocumentSearch(w, httptest.NewRequest("POST", "/v1/documents/search", bytes.NewReader(body)).WithContext(ctx))
	var found SearchResponse
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &found) != nil || found.RetrievalMode != "hybrid" || len(found.Results) == 0 || !bytes.Contains(w.Body.Bytes(), []byte("17 November")) {
		t.Fatal(w.Code, w.Body.String())
	}
	other, err := s.ragEngine.ForActor("unrelated-actor").Search(ctx, "Zirconium procurement deadline", rag.SearchOptions{TopK: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range other.Results {
		if result.DocumentID == ready.DocumentID {
			t.Fatal("real retrieval exposed private evidence")
		}
	}
	w = httptest.NewRecorder()
	s.handleDocumentSource(w, httptest.NewRequest("GET", "/v1/documents/source?id="+url.QueryEscape(ready.DocumentID)+"&format=original", nil).WithContext(ctx))
	if w.Code != 200 || !bytes.Contains(w.Body.Bytes(), []byte("17 November")) {
		t.Fatal("retained private source", w.Code, w.Body.String())
	}
}
