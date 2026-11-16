package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestKnowledgeCompositionCommandsPreserveRequestAndControl(t *testing.T) {
	model := document.SynthesisModelIdentity{ModelID: "local", ModelSHA256: strings.Repeat("a", 64), Runtime: document.RuntimeIdentity{Name: "fixture", Revision: "1"}, Language: "en", ContextTokens: 8192, ResponseTokens: 2048, Protocol: document.SynthesisProtocol}
	job := rag.CompositionStatus{ID: "composition", AnalysisID: "parent", PlanID: "tree", SourcePlanID: "source", Model: model, State: "queued", TotalSteps: 1, ReviewsCurrent: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	posts := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts++
			if strings.HasSuffix(r.URL.Path, "/cancel") || strings.HasSuffix(r.URL.Path, "/retry") {
				_ = json.NewEncoder(w).Encode(map[string]bool{"accepted": true})
				return
			}
			var body serviceclient.DocumentCompositionRequest
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.RequestKey != "request" || body.AnalysisID != "parent" || body.Model != "local" {
				t.Error("request changed", body)
			}
		}
		if r.URL.Query().Has("analysis_id") {
			_ = json.NewEncoder(w).Encode(rag.CompositionHistory{AnalysisID: "parent", Items: []rag.CompositionStatus{job}})
			return
		}
		_ = json.NewEncoder(w).Encode(job)
	}))
	defer s.Close()
	client, _ := serviceclient.New(s.URL, "key", s.Client())
	file := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(file, []byte(`{"request_key":"request","analysis_id":"parent","model":"local","language":"en"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"compose-summary", file}, {"composition", job.ID}, {"compositions", "parent"}, {"cancel-composition", job.ID}, {"retry-composition", job.ID}} {
		if _, err := RunKnowledgeService(context.Background(), client, args, nil, nil, true); err != nil {
			t.Fatal(args, err)
		}
	}
	if posts != 3 {
		t.Fatal("unexpected writes", posts)
	}
	for _, args := range [][]string{{"compose-summary"}, {"compositions", "parent", "bad"}, {"composition-evidence", "id", "bad"}} {
		if _, err := RunKnowledgeService(context.Background(), client, args, nil, nil, true); err == nil {
			t.Fatal("invalid arguments accepted", args)
		}
	}
}
