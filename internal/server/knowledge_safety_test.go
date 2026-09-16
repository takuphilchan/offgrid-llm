package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type knowledgeFixture struct {
	enabled bool
	result  *rag.RAGContext
	err     error
}

func (f knowledgeFixture) IsEnabled() bool { return f.enabled }
func (f knowledgeFixture) EnhancePrompt(context.Context, string) (string, *rag.RAGContext, error) {
	return "", f.result, f.err
}

func TestKnowledgeFailuresNeverSilentlyBecomeGeneralChat(t *testing.T) {
	for _, test := range []struct {
		name   string
		source knowledgeRetriever
		status int
	}{
		{"missing", nil, 503},
		{"disabled", knowledgeFixture{}, 503},
		{"retrieval failure", knowledgeFixture{enabled: true, err: errors.New("private database path")}, 503},
		{"no context", knowledgeFixture{enabled: true}, 422},
		{"no matches", knowledgeFixture{enabled: true, result: &rag.RAGContext{}}, 422},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := &api.ChatCompletionRequest{UseKnowledgeBase: boolPointer(true), Messages: []api.ChatMessage{{Role: "user", Content: "question"}}}
			err := injectKnowledge(context.Background(), req, test.source)
			if err == nil {
				t.Fatal("silently continued without knowledge")
			}
			w := httptest.NewRecorder()
			writeServiceError(w, err)
			if w.Code != test.status || strings.Contains(w.Body.String(), "private database path") {
				t.Fatalf("response: %d %s", w.Code, w.Body.String())
			}
			if len(req.Messages) != 1 {
				t.Fatalf("failed retrieval mutated conversation: %+v", req.Messages)
			}
		})
	}
	if err := injectKnowledge(context.Background(), &api.ChatCompletionRequest{}, nil); err != nil {
		t.Fatalf("ordinary chat requires optional knowledge: %v", err)
	}
}

func TestKnowledgeInjectionPreservesUserMessageAndProvenance(t *testing.T) {
	result := &rag.RAGContext{Results: []rag.SearchResult{{DocumentID: "doc", DocName: "Notes", Chunk: &rag.Chunk{ID: "chunk", Content: "evidence"}}}}
	result.FormatContext()
	req := &api.ChatCompletionRequest{UseKnowledgeBase: boolPointer(true), Messages: []api.ChatMessage{{Role: "system", Content: "original policy"}, {Role: "user", Content: "question"}}}
	if err := injectKnowledge(context.Background(), req, knowledgeFixture{enabled: true, result: result}); err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 3 || req.Messages[0].StringContent() != "original policy" || req.Messages[2].StringContent() != "question" || !strings.Contains(req.Messages[1].StringContent(), `trust="untrusted"`) {
		t.Fatalf("incorrect injection: %+v", req.Messages)
	}
}

func TestBrokenKnowledgeStorageIsReportedByAPI(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rag"), []byte("cannot create database here"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: rag.NewEngine(nil, root)}
	for _, test := range []struct {
		name, method string
		handler      http.HandlerFunc
	}{
		{"enable", "POST", s.handleRAGEnable},
		{"list", "GET", s.handleDocumentsList},
		{"ingest", "POST", s.handleDocumentIngest},
		{"url", "POST", s.handleDocumentIngestURL},
		{"delete", "DELETE", s.handleDocumentDelete},
		{"search", "POST", s.handleDocumentSearch},
		{"reindex", "POST", s.handleDocumentReindex},
		{"evaluate", "POST", s.handleRAGEvaluate},
	} {
		t.Run(test.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			test.handler(w, httptest.NewRequest(test.method, "/", strings.NewReader(`{}`)))
			if w.Code != 503 || strings.Contains(w.Body.String(), root) {
				t.Fatalf("response: %d %s", w.Code, w.Body.String())
			}
		})
	}
	w := httptest.NewRecorder()
	s.handleRAGStatus(w, httptest.NewRequest("GET", "/v1/rag/status", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"storage_available":false`) {
		t.Fatalf("readiness: %d %s", w.Code, w.Body.String())
	}
}
