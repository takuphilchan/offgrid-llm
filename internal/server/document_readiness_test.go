package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func TestDocumentReadinessIndependentOfKnowledgeSetup(t *testing.T) {
	for _, engine := range []*rag.Engine{nil, rag.NewEngine(nil, t.TempDir())} {
		if engine != nil {
			t.Cleanup(func() { _ = engine.Close() })
		}
		s := &Server{ragEngine: engine}
		w := httptest.NewRecorder()
		s.handleRAGStatus(w, httptest.NewRequest("GET", "/v1/rag/status", nil))
		var response RAGStatusResponse
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil {
			t.Fatalf("invalid status: %d %s", w.Code, w.Body.String())
		}
		if response.Enabled || len(response.DocumentFormats) < 6 || response.Stats == nil {
			t.Fatalf("extraction depends on embedding setup: %+v", response)
		}
		for _, f := range response.DocumentFormats {
			if f.ID == "pdf" && f.Qualification != "unverified" {
				t.Fatalf("claimed PDF qualification without evidence: %+v", f)
			}
		}
	}
}
