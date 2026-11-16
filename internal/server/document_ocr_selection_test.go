package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestOCRChatCatalogIsAuthenticatedAndReadOnly(t *testing.T) {
	s := newTestServer(t)
	s.config.RequireAuth = true
	s.authMiddleware.SetRequireAuth(true)
	_, key, err := s.userStore.CreateUser("ocr-reader", "fixture-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	handler := s.buildRouter()
	for _, row := range []struct {
		method, path, token string
		status              int
	}{
		{"GET", "/v1/document-ocr-languages", "", 401},
		{"GET", "/v1/document-ocr-languages", key, 200},
		{"POST", "/v1/document-ocr-languages", key, 405},
		{"POST", "/v1/documents/ocr-languages?language=eng", key, 403},
	} {
		r := httptest.NewRequest(row.method, row.path, strings.NewReader("untrusted bytes"))
		if row.token != "" {
			r.Header.Set("Authorization", "Bearer "+row.token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != row.status {
			t.Fatal(row, w.Code, w.Body.String())
		}
		if row.status == 200 && (!strings.Contains(w.Body.String(), `"language":"chi_sim"`) || w.Header().Get("Cache-Control") != "no-store") {
			t.Fatal("catalog unavailable", w.Body.String())
		}
	}
}

func TestOCRSelectionHTTPRejectsUnsupportedBeforeSaving(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	for _, q := range []string{"ocr_language=sn", "ocr_language=eng%2Beng", "ocr_language=eng%2Bfra%2Bdeu%2Bspa", "ocr_language=", "ocr_language=eng&ocr_language=fra"} {
		w := uploadChatFile(t, h, "source.txt", "native text", q)
		if w.Code != 422 || !strings.Contains(w.Body.String(), "document_ocr_language_unavailable") {
			t.Fatal(q, w.Code, w.Body.String())
		}
	}
	w := uploadChatFile(t, h, "source.txt", "native text", "ocr_language=chi_sim")
	var response struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Attachment.OCRLanguage != "chi_sim" {
		t.Fatal(w.Code, w.Body.String())
	}
	// Copying this explicit private source into Knowledge retains its language.
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	s := &Server{ragEngine: e, sessionHandlers: h}
	body, _ := json.Marshal(map[string]any{"attachment": map[string]string{"session": "chat", "id": response.Attachment.ID, "sha256": response.Attachment.SHA256}})
	r := httptest.NewRequest("POST", "/v1/collections/imports?collection_id=workspace&collection_revision=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	record, err := s.acceptDocumentImport(r)
	if err != nil || record.OCRLanguage != "chi_sim" {
		t.Fatal(record, err)
	}
}

func TestMixedOCRPrivatePublicationKeepsSelection(t *testing.T) {
	h := newStreamingSessions(t)
	defer h.closeTurns()
	w := uploadChatFile(t, h, "source.txt", "native source text", "ocr_language=chi_sim%2Bfra")
	var response struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	s := &Server{ragEngine: e, sessionHandlers: h}
	body, _ := json.Marshal(map[string]any{"attachment": map[string]string{"session": "chat", "id": response.Attachment.ID, "sha256": response.Attachment.SHA256}})
	r := httptest.NewRequest("POST", "/v1/collections/imports?collection_id=workspace&collection_revision=1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	record, err := s.acceptDocumentImport(r)
	if err != nil || record.OCRLanguage != "chi_sim+fra" || record.SHA256 != response.Attachment.SHA256 {
		t.Fatal("publication changed selected packs or source", record, err)
	}
}

// Actual worker execution followed by source inspection. Generated answers are
// deliberately not part of this recognition regression.
func TestOCRSelectionRealPrivateRetryHTTP(t *testing.T) {
	testOCRSelectionRealPrivateRetryHTTP(t, "chi_sim")
}

func TestMixedOCRSelectionRealPrivateRetryHTTP(t *testing.T) {
	testOCRSelectionRealPrivateRetryHTTP(t, "chi_sim+fra")
}

func testOCRSelectionRealPrivateRetryHTTP(t *testing.T, language string) {
	packs := os.Getenv("OFFGRID_TEST_OCR_PACKS")
	if packs == "" || os.Getenv("OFFGRID_TEST_OCR_HTTP") != "1" {
		t.Skip("requires explicit packs and compiled test beside OCR worker")
	}
	source, err := os.ReadFile("../rag/testdata/documents/ocr/languages/chi_sim.png")
	if err != nil {
		t.Fatal(err)
	}
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	h := newStreamingSessions(t)
	defer h.closeTurns()
	h.agentServer = &Server{ragEngine: e}
	w := uploadChatFile(t, h, "contract.png", string(source), "ocr_language="+url.QueryEscape(language))
	var response struct {
		Attachment sessions.Attachment `json:"attachment"`
	}
	if w.Code != 422 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Attachment.OCRLanguage != language || response.Attachment.ErrorCode != "document_ocr_language_missing" {
		t.Fatal(w.Code, w.Body.String())
	}
	id := response.Attachment.ID
	for _, id := range strings.Split(language, "+") {
		pack, err := os.ReadFile(filepath.Join(packs, id+".traineddata"))
		if err != nil {
			t.Fatal(err)
		}
		if err = e.ImportOCRLanguage(context.Background(), id, bytes.NewReader(pack)); err != nil {
			t.Fatal(err)
		}
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("POST", "/v1/sessions/chat/attachments/"+id+"/prepare", nil))
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Attachment.OCRLanguage != language || response.Attachment.ExtractionRevision == "" || response.Attachment.OCRCompletedPages != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", "/v1/sessions/chat/attachments/"+id+"/preview", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "合同") || !strings.Contains(w.Body.String(), "42") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("GET", "/v1/sessions/chat/attachments/"+id+"/structure?revision="+response.Attachment.Revision, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "合同") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, id := range strings.Split(language, "+") {
		if !strings.Contains(w.Body.String(), "tesseract-"+id) {
			t.Fatal("source lost language provenance", id)
		}
	}
}
