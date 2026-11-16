package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

func TestDocumentAnalysisHTTPIdentityRecoveryAndRevocation(t *testing.T) {
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
	refs := []document.SourceRef{}
	for i, text := range []string{"政策 17.25\nException July.", "政策 18.25\nException August."} {
		d := &rag.Document{ID: []string{"before", "after"}[i], Name: "Policy", RawContent: text, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		if err = store.AddDocumentWithChunks(d, nil, nil); err != nil {
			t.Fatal(err)
		}
		refs = append(refs, document.SourceRef{DocumentID: d.ID, Revision: d.ExtractionRevision})
	}
	policy, err := store.CreateCollection(owner.ID, "Private", []string{reader.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.AssignDocumentCollection(owner.ID, "before", policy.ID, 1); err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: e}
	middleware := users.NewMiddleware(accounts)
	middleware.SetRequireAuth(true)
	request := func(key, method, path string, body any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		r.Header.Set("Authorization", "Bearer "+key)
		w := httptest.NewRecorder()
		fn := s.handleDocumentAnalyses
		switch strings.Split(path, "?")[0] {
		case "/v1/document-analyses/results":
			fn = s.handleDocumentAnalysisResults
		case "/v1/document-analyses/source":
			fn = s.handleDocumentAnalysisSource
		case "/v1/document-analyses/export":
			fn = s.handleDocumentAnalysisExport
		case "/v1/document-analyses/cancel", "/v1/document-analyses/retry":
			fn = s.handleDocumentAnalysisControl
		}
		middleware.Wrap(users.RequirePermission(users.PermissionRAG)(http.HandlerFunc(fn))).ServeHTTP(w, r)
		return w
	}
	body := map[string]any{"request_key": "first", "workflow": "comparison", "sources": refs}
	w := request(readerKey, "POST", "/v1/document-analyses", body)
	var record rag.AnalysisRecord
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &record) != nil || record.Plan == nil || record.Plan.Sources[0].Revision != refs[0].Revision {
		t.Fatal(w.Code, w.Body.String())
	}
	id := record.ID
	var history rag.AnalysisHistory
	w = request(readerKey, "GET", "/v1/document-analyses?offset=0&limit=20", nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history.Analyses) != 1 || history.Analyses[0].ID != id || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("history", w.Code, w.Body.String())
	}
	for _, query := range []string{"?offset=-1", "?offset=257", "?limit=21", "?limit=bad", "?id="} {
		if w = request(readerKey, "GET", "/v1/document-analyses"+query, nil); w.Code != 400 {
			t.Fatal("history bounds", query, w.Code)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		w = request(readerKey, "GET", "/v1/document-analyses?id="+id, nil)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &record) != nil {
			t.Fatal(w.Code, w.Body.String())
		}
		if record.State == "complete" || record.State == "incomplete" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if record.State != "incomplete" || record.CompletedUnits != 2 || len(record.Plan.Gaps) == 0 {
		t.Fatal("legacy uncertainty lost", record)
	}
	for _, format := range []string{"json", "csv", "markdown"} {
		w = request(readerKey, "GET", "/v1/document-analyses/export?id="+id+"&format="+format, nil)
		var artifact rag.AnalysisExport
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &artifact) != nil || artifact.AnalysisID != id || artifact.Format != format || !strings.Contains(artifact.Content, "17.25") || !strings.Contains(artifact.Content, "incomplete") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("export", w.Code, w.Body.String())
		}
	}
	if w = request(ownerKey, "GET", "/v1/document-analyses/export?id="+id+"&format=json", nil); w.Code != 404 {
		t.Fatal("admin exported another actor's job", w.Code)
	}
	if w = request(readerKey, "GET", "/v1/document-analyses/export?id="+id+"&format=html", nil); w.Code != 400 {
		t.Fatal("export format", w.Code)
	}
	w = request(readerKey, "GET", "/v1/document-analyses/results?id="+id+"&step=0&limit=1", nil)
	var page rag.AnalysisResultPage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil || page.NextOffset == nil || page.TotalChanges < 2 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(page.Previews) != 1 || page.Previews[0].Before == nil || !strings.Contains(page.Previews[0].Before.Text, "17.25") {
		t.Fatal("missing verified preview", w.Body.String())
	}
	w = request(readerKey, "GET", "/v1/document-analyses/source?id="+id+"&step=0&change=0&side=before", nil)
	var source rag.AnalysisSourceResult
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &source) != nil || source.DocumentID != "before" || !strings.Contains(source.Text, "17.25") || source.Citation.Revision != refs[0].Revision || source.Truncated {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(readerKey, "POST", "/v1/document-analyses", body)
	var replay rag.AnalysisRecord
	if w.Code != 202 || json.Unmarshal(w.Body.Bytes(), &replay) != nil || replay.ID != id || replay.Attempt != record.Attempt {
		t.Fatal("replayed completed work", w.Code, w.Body.String())
	}
	for _, suffix := range []string{"&step=-1", "&offset=-1", "&limit=101", "&step=bad"} {
		w = request(readerKey, "GET", "/v1/document-analyses/results?id="+id+suffix, nil)
		if w.Code != 400 {
			t.Fatal("invalid result bounds", w.Code, w.Body.String())
		}
	}
	if w = request(ownerKey, "GET", "/v1/document-analyses?id="+id, nil); w.Code != 404 {
		t.Fatal("admin read another actor's analysis", w.Code)
	}
	if _, err = store.UpdateCollection(owner.ID, policy.ID, policy.Revision, policy.Name, nil, nil); err != nil {
		t.Fatal(err)
	}
	w = request(readerKey, "GET", "/v1/document-analyses", nil)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &history) != nil || len(history.Analyses) != 0 || strings.Contains(w.Body.String(), id) {
		t.Fatal("revoked history", w.Code, w.Body.String())
	}
	for _, path := range []string{"/v1/document-analyses?id=", "/v1/document-analyses/results?id=", "/v1/document-analyses/source?step=0&change=0&side=before&id=", "/v1/document-analyses/export?format=json&id="} {
		w = request(readerKey, "GET", path+id, nil)
		if w.Code != 404 || strings.Contains(w.Body.String(), "17.25") {
			t.Fatal("revocation leaked analysis", w.Code, w.Body.String())
		}
	}
	if w = request(readerKey, "POST", "/v1/document-analyses/cancel", map[string]string{"id": id}); w.Code != 200 {
		t.Fatal("revoked owner cannot stop work", w.Code, w.Body.String())
	}
	if w = request(readerKey, "POST", "/v1/document-analyses", map[string]any{"workflow": "summary"}); w.Code != 501 {
		t.Fatal("unimplemented synthesis advertised", w.Code)
	}
}
