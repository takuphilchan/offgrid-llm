package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type documentSynthesisFixture struct {
	prepared, calls atomic.Int32
	i               document.SynthesisModelIdentity
}

func (p *documentSynthesisFixture) Prepare(_ context.Context, model, language string) (document.SynthesisModelIdentity, error) {
	p.prepared.Add(1)
	i := p.i
	i.ModelID = model
	i.Language = language
	return i, i.Validate()
}
func (p *documentSynthesisFixture) WithModel(_ context.Context, i document.SynthesisModelIdentity, run func(document.SynthesisLocalSession) error) error {
	return run(&documentSynthesisFixtureSession{p: p, i: i})
}

type documentSynthesisFixtureSession struct {
	p *documentSynthesisFixture
	i document.SynthesisModelIdentity
}

func (s *documentSynthesisFixtureSession) Identity() document.SynthesisModelIdentity { return s.i }
func (s *documentSynthesisFixtureSession) Count(context.Context, document.SynthesisPrompt) (int, error) {
	return 1000, nil
}
func (s *documentSynthesisFixtureSession) Generate(_ context.Context, p document.SynthesisPrompt, _ int) ([]byte, string, error) {
	s.p.calls.Add(1)
	var input struct {
		Batch document.SynthesisBatch `json:"batch"`
	}
	if err := json.Unmarshal([]byte(p.User), &input); err != nil {
		return nil, "", err
	}
	b := input.Batch
	ids := []string{}
	for _, e := range b.Evidence {
		ids = append(ids, e.ID)
	}
	draft := document.SynthesisDraft{BatchID: b.ID, ReviewedEvidence: ids, Items: []document.SynthesisItem{{Text: b.Evidence[0].Text, Evidence: []document.SynthesisQuote{{EvidenceID: "e0", Quote: b.Evidence[0].Text}}, Conditions: []document.SynthesisQuote{}, Exceptions: []document.SynthesisQuote{}}}}
	raw, err := json.Marshal(draft)
	return raw, "stop", err
}

func TestDocumentSynthesisHTTPTraversalPrivacyAndRevocation(t *testing.T) {
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
	doc := &rag.Document{ID: "policy", Name: "Policy", RawContent: "Policy 17.25. Exception July. 中文", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err = store.AddDocumentWithChunks(doc, nil, nil); err != nil {
		t.Fatal(err)
	}
	policy, err := store.CreateCollection(owner.ID, "Private", []string{reader.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.AssignDocumentCollection(owner.ID, doc.ID, policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	p := &documentSynthesisFixture{i: document.SynthesisModelIdentity{ModelID: "fixture", ModelSHA256: strings.Repeat("a", 64), Runtime: document.RuntimeIdentity{Name: "fixture", Revision: "1"}, Language: "en", ContextTokens: 8192, ResponseTokens: 2048, Protocol: document.SynthesisProtocol}}
	s := &Server{ragEngine: e, userStore: accounts, config: &config.Config{RequireAuth: true}, documentSynthesisFactory: func(string, string) documentSynthesisRuntime { return p }}
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	request := func(key, method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		handler := s.handleDocumentSynthesis
		if strings.Contains(path, "/results") {
			handler = s.handleDocumentSynthesisResults
		}
		if strings.Contains(path, "/retry") {
			handler = s.handleDocumentSynthesisRetry
		}
		if strings.Contains(path, "/export") {
			handler = s.handleDocumentSynthesisExport
		}
		if strings.Contains(path, "/review") {
			handler = s.handleDocumentSynthesisReview
		}
		middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(handler))).ServeHTTP(w, r)
		return w
	}
	body := synthesisCreateRequest{RequestKey: "question", Workflow: "requirements", Sources: []document.SourceRef{{DocumentID: doc.ID, Revision: doc.ExtractionRevision}}, Model: "fixture", Language: "zh-Hans"}
	w := request(readerKey, "POST", "/v1/document-analyses/synthesis", body)
	var job rag.SynthesisRecord
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &job) != nil || job.ID == "" {
		t.Fatal(w.Code, w.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w = request(readerKey, "GET", "/v1/document-analyses/synthesis?id="+job.ID, nil)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &job) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		if job.State == "awaiting_review" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if job.State != "awaiting_review" || job.CompletedUnits != len(job.Plan.Units) || !job.ReviewRequired {
		t.Fatal(job)
	}
	w = request(readerKey, "GET", "/v1/document-analyses/synthesis/results?id="+job.ID, nil)
	var page rag.SynthesisResultPage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || len(page.Receipts) != 1 || len(page.Receipts[0].Result.Candidates) != 1 || strings.Contains(w.Body.String(), "raw_response") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	quote := page.Receipts[0].Result.Candidates[0].Evidence[0]
	reviewRequest := rag.SynthesisReviewRequest{ID: job.ID, Batch: 0, BatchID: page.Receipts[0].BatchID, ResponseSHA256: page.Receipts[0].ResponseSHA256, SectionChecked: true, Decisions: []rag.SynthesisDecision{{Decision: "keep", Note: "Checked source"}}}
	w = request(readerKey, "POST", "/v1/document-analyses/synthesis/review", reviewRequest)
	var review rag.SynthesisReview
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &review) != nil || review.Revision != 1 || review.Author != reader.ID {
		t.Fatal("review", w.Code, w.Body.String())
	}
	w = request(readerKey, "POST", "/v1/document-analyses/synthesis/review", reviewRequest)
	if w.Code != 409 {
		t.Fatal("stale review", w.Code)
	}
	w = request(ownerKey, "GET", "/v1/document-analyses/synthesis/review?batch=0&id="+job.ID, nil)
	if w.Code != 404 {
		t.Fatal("admin read private review", w.Code)
	}
	if quote.Text != doc.RawContent || quote.Citation.Revision != doc.ExtractionRevision {
		t.Fatal(quote)
	}
	for _, format := range []string{"json", "csv", "markdown"} {
		w = request(readerKey, "GET", "/v1/document-analyses/synthesis/export?id="+job.ID+"&format="+format, nil)
		var artifact rag.AnalysisExport
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &artifact) != nil || artifact.AnalysisID != job.ID || !strings.Contains(artifact.Content, "source_identity_only") || !strings.Contains(artifact.Content, "semantic_review_required") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("export", w.Code, w.Body.String())
		}
	}
	w = request(ownerKey, "GET", "/v1/document-analyses/synthesis/export?id="+job.ID+"&format=json", nil)
	if w.Code != 404 {
		t.Fatal("admin exported private job", w.Code)
	}
	w = request(ownerKey, "GET", "/v1/document-analyses/synthesis?id="+job.ID, nil)
	if w.Code != 404 {
		t.Fatal("admin read private job", w.Code)
	}
	if _, err = store.UpdateCollection(owner.ID, policy.ID, policy.Revision, policy.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/document-analyses/synthesis?id=", "/v1/document-analyses/synthesis/results?id=", "/v1/document-analyses/synthesis/export?format=json&id=", "/v1/document-analyses/synthesis/review?batch=0&id="} {
		w = request(readerKey, "GET", path+job.ID, nil)
		if w.Code != 404 {
			t.Fatal("revoked evidence", w.Code)
		}
	}
	w = request(readerKey, "GET", "/v1/document-analyses/synthesis", nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), job.ID) {
		t.Fatal("history leaked revoked job", w.Code, w.Body.String())
	}
	before := p.prepared.Load()
	reviewRequest.ExpectedRevision = 1
	w = request(readerKey, "POST", "/v1/document-analyses/synthesis/review", reviewRequest)
	if w.Code != 404 {
		t.Fatal("revoked review write", w.Code)
	}
	w = request(readerKey, "POST", "/v1/document-analyses/synthesis", body)
	if w.Code != 404 || p.prepared.Load() != before {
		t.Fatal("model prepared before source authorization", w.Code)
	}
}
