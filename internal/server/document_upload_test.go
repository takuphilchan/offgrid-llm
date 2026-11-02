package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func documentUploadRequest(t *testing.T, filename string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/rag/documents", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return r
}

func TestDocumentUploadSpillCleanup(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(key, dir)
	}
	content := bytes.Repeat([]byte("text"), 300000)
	data, name, _, err := readDocumentUpload(documentUploadRequest(t, "test.txt", content))
	if err != nil || name != "test.txt" || !bytes.Equal(data, content) {
		t.Fatalf("upload: %s %v", name, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("multipart temp files retained: %v %v", entries, err)
	}
}

func TestDocumentUploadRejectsOversize(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(key, dir)
	}
	// Exercise both the file limit and the complete request limit.
	for _, extra := range []int{1, 2 << 20} {
		r := documentUploadRequest(t, "large.txt", bytes.Repeat([]byte("a"), int(maxDocumentUploadBytes)+extra))
		_, _, _, err := readDocumentUpload(r)
		w := httptest.NewRecorder()
		writeDocumentIngestError(w, err)
		if err == nil || w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversize status=%d err=%v", w.Code, err)
		}
		entries, readErr := os.ReadDir(dir)
		if readErr != nil || len(entries) != 0 {
			t.Fatalf("failed upload leaked temp files: %v %v", entries, readErr)
		}
	}
}

func TestDocumentUploadFailureContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid", &rag.ParseError{Code: "document_invalid", Message: "Unreadable document."}, 422, "document_invalid"},
		{"format", &rag.ParseError{Code: "document_unsupported_format", Message: "Unsupported format."}, 422, "document_unsupported_format"},
		{"busy", &rag.ParseError{Code: "document_busy", Message: "Other documents are being processed."}, 503, "document_busy"},
		{"cancel", context.Canceled, 408, "document_cancelled"},
		{"timeout", context.DeadlineExceeded, 408, "document_timeout"},
		{"private failure", errors.New("/secret/file private contents"), 500, "document_index_failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeDocumentIngestError(w, test.err)
			var response IngestResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if w.Code != test.status || response.ErrorCode != test.code || response.Success || response.Document != nil || strings.Contains(w.Body.String(), "secret") {
				t.Fatalf("bad response: %d %s", w.Code, w.Body)
			}
		})
	}
}

func TestDocumentUploadMalformedAndCancelled(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("broken multipart"))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	_, _, _, err := readDocumentUpload(r)
	var problem *rag.ParseError
	if !errors.As(err, &problem) || problem.Code != "document_invalid" {
		t.Fatalf("malformed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = httptest.NewRequest(http.MethodPost, "/", io.NopCloser(strings.NewReader("private"))).WithContext(ctx)
	if _, _, _, err := readDocumentUpload(r); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestDocumentUploadParseFailurePublishesNothing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	engine := rag.NewEngine(nil, t.TempDir())
	defer engine.Close()
	s := &Server{ragEngine: engine}
	for _, test := range []struct{ name, content string }{
		{"broken.pdf", "%PDF-1.7\ncorrupt"},
		{"broken.docx", "not a ZIP file"},
		{"legacy.xls", "old workbook"},
	} {
		doc, err := s.handleFileUpload(documentUploadRequest(t, test.name, []byte(test.content)))
		if err == nil || doc != nil {
			t.Fatalf("failed document published: %s %v", test.name, err)
		}
		if docs := engine.ListDocuments(); len(docs) != 0 {
			t.Fatalf("ghost ready records: %v", docs)
		}
	}
}
