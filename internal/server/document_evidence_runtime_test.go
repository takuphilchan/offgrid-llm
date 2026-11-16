package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

// This opt-in test runs the actual embedding model and HTTP ingestion/search/
// citation paths with disposable state. It does not qualify generated answers.
func TestRealKnowledgeEvidenceHTTP(t *testing.T) {
	model := os.Getenv("OFFGRID_TEST_EMBEDDING_MODEL")
	if model == "" || os.Getenv("OFFGRID_LLAMA_SERVER_PATH") == "" {
		t.Skip("requires explicitly supplied local embedding model and runtime; never downloads")
	}
	cache := t.TempDir()
	t.Setenv("LOCALAPPDATA", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	worker := inference.NewEmbeddingEngine(t.TempDir())
	defer worker.Unload()
	engine := rag.NewEngine(worker, t.TempDir())
	defer engine.Close()
	engine.SetModelResolver(func(string) (string, error) { return model, nil })
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := engine.Enable(ctx, "evidence-test-embedding"); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: engine}
	t.Run("private-collection", func(t *testing.T) { verifyRealCollectionImportHTTP(t, s, ctx) })
	cases := []struct {
		name, question, want string
		data                 []byte
	}{
		{"policy.docx", "When can rural applicants apply?", "31 July", policyWithFootnote(t)},
		{"tracked.docx", "What is the application limit?", "17.25 current", trackedPolicy(t)},
		{"margin-policy.docx", "When can rural applicants apply?", "31 July", policyWithMarginException(t)},
		{"styled-policy.docx", "When do Zirconium applications close?", "30 June", longLinkedPolicy(t)},
		{"allocation.txt", "harvest?", "42", []byte("Harvest allocation: UK case 42.")},
		{"annual-caps.docx", "17.25", "17.25", policyWithDistantHeader(t)},
		{"allocations.xlsx", "1234.567", "1234.567", workbookWithDistantHeader(t)},
		{"speaker-notes.pptx", "When do Zirconium applications close?", "30 June", presentationWithNotesException(t)},
		{"budget.txt", "青岚项目预算是多少？", "七百万元", []byte("行政记录。青岚项目预算为七百万元。")},
	}
	if os.Getenv("OFFGRID_TEST_DOCUMENT_WORKER") != "" {
		data, err := os.ReadFile("../rag/testdata/documents/native-form.pdf")
		if err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			name, question, want string
			data                 []byte
		}{"native-form.pdf", "What is Amount?", "0017.25", data})
	}
	if fixture := os.Getenv("OFFGRID_TEST_IMAGE_DOCUMENT"); fixture != "" {
		data, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		pack, err := os.ReadFile(os.Getenv("OFFGRID_TEST_OCR_LANGUAGE"))
		if err != nil {
			t.Fatal(err)
		}
		if err = engine.ImportOCRLanguage(ctx, "eng", bytes.NewReader(pack)); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, struct {
			name, question, want string
			data                 []byte
		}{"receipt.png", "What is the receipt total?", "42", data})
		if os.Getenv("OFFGRID_TEST_TIFF") == "1" {
			cases = append(cases, struct {
				name, question, want string
				data                 []byte
			}{"records.tiff", "What is the receipt total?", "42", runtimeTIFF(t, data)})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var upload bytes.Buffer
			form := multipart.NewWriter(&upload)
			part, err := form.CreateFormFile("file", tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = part.Write(tc.data); err != nil {
				t.Fatal(err)
			}
			if err = form.Close(); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/v1/documents/ingest", &upload).WithContext(ctx)
			r.Header.Set("Content-Type", form.FormDataContentType())
			w := httptest.NewRecorder()
			s.handleDocumentIngest(w, r)
			var imported IngestResponse
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &imported) != nil || imported.Document == nil || imported.Document.Evidence == nil {
				t.Fatal(w.Code, w.Body.String())
			}
			body, _ := json.Marshal(SearchRequest{Query: tc.question, TopK: 20, DocumentFilter: []string{imported.Document.ID}})
			w = httptest.NewRecorder()
			s.handleDocumentSearch(w, httptest.NewRequest("POST", "/v1/documents/search", bytes.NewReader(body)).WithContext(ctx))
			var found SearchResponse
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &found) != nil || found.RetrievalMode != "hybrid" || len(found.Results) == 0 {
				t.Fatal(w.Code, w.Body.String())
			}
			c := found.Results[0].Chunk
			if c.Source == nil || !strings.Contains(c.Content, tc.want) {
				t.Fatalf("evidence missing: %+v", c)
			}
			query := url.Values{"id": {imported.Document.ID}, "start_byte": {strconv.Itoa(c.StartChar)}, "end_byte": {strconv.Itoa(c.EndChar)}, "source_sha256": {c.Source.SHA256}, "extractor_revision": {c.Source.Revision}, "content_hash": {c.ContentHash}}
			w = httptest.NewRecorder()
			s.handleDocumentSource(w, httptest.NewRequest("GET", "/v1/documents/source?"+query.Encode(), nil).WithContext(ctx))
			var source struct {
				Content  string
				Verified bool `json:"citation_verified"`
			}
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &source) != nil || !source.Verified || !strings.Contains(source.Content, tc.want) {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.name == "tracked.docx" {
				for _, result := range found.Results {
					if strings.Contains(result.Chunk.Content, "999.99") {
						t.Fatal("deleted text entered current Knowledge evidence")
					}
				}
			}
			if tc.name == "native-form.pdf" {
				w = httptest.NewRecorder()
				s.handleDocumentStructure(w, httptest.NewRequest("GET", "/v1/documents/structure?id="+url.QueryEscape(imported.Document.ID), nil).WithContext(ctx))
				var view document.StructurePage
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &view) != nil || len(view.Blocks) != 4 || view.Blocks[0].FormField == nil || view.Blocks[0].FormField.Value != "0017.25" || view.Coverage == nil || len(view.Coverage.ReviewUnits) != 1 {
					t.Fatal("native form structure/coverage lost", w.Code, w.Body.String())
				}
			}
			if tc.name == "policy.docx" {
				verifyRealCorrectionHTTP(t, s, ctx, imported.Document, tc.data, query.Encode())
			}
			if tc.name == "records.tiff" {
				verifyTIFFKnowledgePreview(t, s, ctx, imported.Document.ID)
			}
			if tc.name == "styled-policy.docx" {
				w = httptest.NewRecorder()
				s.handleDocumentStructure(w, httptest.NewRequest("GET", "/v1/documents/structure?id="+url.QueryEscape(imported.Document.ID), nil).WithContext(ctx))
				var outline document.StructurePage
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &outline) != nil || len(outline.Navigation) == 0 || outline.Navigation[0].Title != "Procurement policy" {
					t.Fatal("Knowledge lost native Word heading", w.Code, w.Body.String())
				}
				target := outline.Navigation[0]
				q := url.Values{"id": {imported.Document.ID}, "revision": {outline.Revision}, "unit": {target.UnitID}, "offset": {strconv.Itoa(target.BlockOffset)}, "limit": {"1"}}
				w = httptest.NewRecorder()
				s.handleDocumentStructure(w, httptest.NewRequest("GET", "/v1/documents/structure?"+q.Encode(), nil).WithContext(ctx))
				var opened document.StructurePage
				if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &opened) != nil || len(opened.Blocks) != 1 || opened.Blocks[0].ID != target.BlockID || opened.Revision != outline.Revision {
					t.Fatal("Knowledge navigation changed source identity", w.Code, w.Body.String())
				}
			}
			if tc.name == "annual-caps.docx" || tc.name == "allocations.xlsx" {
				verifyLinkedHeaderHTTP(t, s, ctx, found)
			}
			if tc.name == "speaker-notes.pptx" {
				note := false
				for _, r := range found.Results {
					note = note || strings.Contains(r.Chunk.Content, "31 July")
				}
				if !note {
					t.Fatal("real retrieval lost linked speaker-note exception")
				}
			}
		})
	}
}
