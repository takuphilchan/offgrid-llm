package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type compositionHTTPFixture struct{ documentSynthesisFixture }

func (p *compositionHTTPFixture) WithModel(_ context.Context, i document.SynthesisModelIdentity, run func(document.SynthesisLocalSession) error) error {
	return run(&compositionHTTPSession{documentSynthesisFixtureSession{&p.documentSynthesisFixture, i}})
}

type compositionHTTPSession struct {
	documentSynthesisFixtureSession
}

func (s *compositionHTTPSession) Generate(ctx context.Context, p document.SynthesisPrompt, n int) ([]byte, string, error) {
	var input struct {
		TaskID string `json:"task_id"`
		Inputs []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(p.User), &input); err != nil {
		return nil, "", err
	}
	if input.TaskID == "" {
		return s.documentSynthesisFixtureSession.Generate(ctx, p, n)
	}
	s.p.calls.Add(1)
	ids := []string{}
	for _, in := range input.Inputs {
		ids = append(ids, in.ID)
	}
	raw, err := json.Marshal(map[string]any{"task_id": input.TaskID, "sections": []any{map[string]any{"text": "Submit by 31 July.", "inputs": ids}}, "omitted": []string{}})
	return raw, "stop", err
}

func TestDocumentCompositionHTTPReportEvidenceHistoryAndRevocation(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	store, err := e.ImportStore()
	if err != nil {
		t.Fatal(err)
	}
	accounts := users.NewUserStore("")
	admin, adminKey, err := accounts.CreateUser("admin", "isolated-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	reader, key, err := accounts.CreateUser("reader", "isolated-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	doc := &rag.Document{ID: "policy", Name: "Policy", RawContent: "Submit by 31 July.", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	collection, err := store.CreateCollection(admin.ID, "Private", []string{reader.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.AssignDocumentCollection(admin.ID, doc.ID, collection.ID, 1); err != nil {
		t.Fatal(err)
	}
	p := &compositionHTTPFixture{documentSynthesisFixture{i: document.SynthesisModelIdentity{ModelID: "fixture", ModelSHA256: strings.Repeat("a", 64), Runtime: document.RuntimeIdentity{Name: "fixture", Revision: "1"}, Language: "en", ContextTokens: 8192, ResponseTokens: 2048, Protocol: document.SynthesisProtocol}}}
	s := &Server{ragEngine: e, userStore: accounts, config: &config.Config{RequireAuth: true}, documentSynthesisFactory: func(string, string) documentSynthesisRuntime { return p }}
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	request := func(token, method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler := s.handleDocumentCompositions
		if strings.Contains(path, "/report") || strings.Contains(path, "/evidence") {
			handler = s.handleDocumentCompositionReport
		}
		if strings.Contains(path, "/cancel") || strings.Contains(path, "/retry") {
			handler = s.handleDocumentCompositionControl
		}
		if strings.Contains(path, "/export") {
			handler = s.handleDocumentSynthesisExport
		}
		middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(handler))).ServeHTTP(w, r)
		return w
	}
	parent, err := store.CreateSynthesis(reader.ID, "parent", "summary", []document.SourceRef{{DocumentID: doc.ID, Revision: doc.ExtractionRevision}}, p.i, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.StartSynthesis(reader.ID, parent.ID, p); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		parent, err = store.Synthesis(reader.ID, parent.ID)
		if err != nil {
			t.Fatal(err)
		}
		if parent.State == "awaiting_review" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	base := "/v1/document-analyses/synthesis/compositions"
	body := compositionCreateRequest{RequestKey: "compose", AnalysisID: parent.ID, Model: "fixture", Language: "en"}
	for _, token := range []string{"", adminKey} {
		w := request(token, "POST", base, body)
		if w.Code != 401 && w.Code != 404 {
			t.Fatal("private parent admitted", w.Code, w.Body.String())
		}
	}
	if p.prepared.Load() != 0 {
		t.Fatal("unauthorized request prepared model")
	}
	w := request(key, "POST", base, body)
	var job rag.CompositionStatus
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &job) != nil || job.ID == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w = request(key, "GET", base+"?id="+job.ID, nil)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &job) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		if job.State == "awaiting_review" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	w = request(key, "GET", base+"/report?id="+job.ID, nil)
	var report rag.CompositionReportView
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil || len(report.Sections) != 1 || report.Sections[0].Contributors != 1 || !report.SemanticReviewRequired || !report.Composition.ReviewsCurrent || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(key, "GET", base+"/evidence?id="+job.ID+"&section=0", nil)
	var evidence rag.CompositionEvidencePage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &evidence) != nil || len(evidence.Items) != 1 || evidence.Items[0].Candidate.Evidence[0].Text != doc.RawContent || evidence.PlanID != job.PlanID {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(key, "GET", base+"?analysis_id="+parent.ID, nil)
	var history rag.CompositionHistory
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history.Items) != 1 || history.Items[0].ID != job.ID {
		t.Fatal(w.Code, w.Body.String())
	}
	calls := p.calls.Load()
	for _, format := range []string{"json", "csv", "markdown"} {
		w = request(key, "GET", base+"/export?id="+job.ID+"&format="+format, nil)
		var artifact rag.AnalysisExport
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &artifact) != nil || artifact.AnalysisID != job.ID || !strings.Contains(artifact.Content, "source_identity_only") || !strings.Contains(artifact.Content, doc.RawContent) {
			t.Fatal("export", w.Code, w.Body.String())
		}
	}
	w = request(key, "POST", base, body)
	var same rag.CompositionStatus
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &same) != nil || same.ID != job.ID || p.calls.Load() != calls {
		t.Fatal("idempotent create", w.Code, w.Body.String())
	}
	page, err := store.SynthesisResults(reader.ID, parent.ID, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	receipt := page.Receipts[0]
	_, err = store.SaveSynthesisReview(context.Background(), reader.ID, rag.SynthesisReviewRequest{ID: parent.ID, Batch: 0, BatchID: receipt.BatchID, ResponseSHA256: receipt.ResponseSHA256, SectionChecked: true, Decisions: []rag.SynthesisDecision{{Decision: "exclude"}}})
	if err != nil {
		t.Fatal(err)
	}
	w = request(key, "GET", base+"/report?id="+job.ID, nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &report) != nil || report.Composition.ReviewsCurrent {
		t.Fatal("stale review not disclosed", w.Body.String())
	}
	for _, path := range []string{"?id=" + job.ID, "?analysis_id=" + parent.ID, "/report?id=" + job.ID, "/evidence?section=0&id=" + job.ID} {
		w = request(adminKey, "GET", base+path, nil)
		if w.Code != 404 {
			t.Fatal("admin leaked private result", path, w.Code)
		}
	}
	if _, err = store.UpdateCollection(admin.ID, collection.ID, collection.Revision, collection.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"?id=" + job.ID, "?analysis_id=" + parent.ID, "/report?id=" + job.ID, "/evidence?section=0&id=" + job.ID} {
		w = request(key, "GET", base+path, nil)
		if w.Code != 404 {
			t.Fatal("revoked result", path, w.Code)
		}
	}
	w = request(key, "POST", base+"/cancel", map[string]string{"id": job.ID})
	if w.Code != 200 {
		t.Fatal("revoked owner cannot cancel", w.Code)
	}
}
