package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/rag"
)

func TestKnowledgeRevocationProtectsJobHistoryAndControls(t *testing.T) {
	e := rag.NewEngine(nil, t.TempDir())
	defer e.Close()
	store, _ := e.ImportStore()
	doc := &rag.Document{ID: "source", Name: "source.pdf", RawContent: "private source", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := store.AddDocument(doc); err != nil {
		t.Fatal(err)
	}
	m := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	runner := agents.NewRunner(m, nil, nil)
	artifactStore, err := artifacts.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{ragEngine: e, agentManager: m, agentRunner: runner, artifactStore: artifactStore}
	cfg := agents.DefaultAgentConfig()
	cfg.KnowledgeDependencies = []string{doc.ID}
	task, err := runner.Create("confidential task", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.AddTaskStep(task.ID, agents.Step{Type: "action", ToolResult: "confidential source excerpt"}); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handleJobRead(w, httptest.NewRequest(method, path, strings.NewReader(body)))
		return w
	}
	base := "/api/v2/jobs/" + task.ID
	if w := request("GET", base, ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := store.DeleteDocument(doc.ID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{base, base + "/events", base + "/export", base + "/artifact?digest=missing"} {
		w := request("GET", path, "")
		if w.Code != 403 || strings.Contains(w.Body.String(), "confidential") {
			t.Fatal("job data leaked", path, w.Code, w.Body.String())
		}
	}
	if w := request("POST", base+"/resume", `{}`); w.Code != 403 {
		t.Fatal("revoked resume accepted", w.Code, w.Body.String())
	}
	w := httptest.NewRecorder()
	s.handleAgentTasks(w, httptest.NewRequest("GET", "/v1/agents/tasks", nil))
	if strings.Contains(w.Body.String(), "confidential") {
		t.Fatal("legacy list leaked")
	}
	w = httptest.NewRecorder()
	s.handleAgentTaskAction(w, httptest.NewRequest("GET", "/v1/agents/tasks/"+task.ID, nil))
	if w.Code != 403 {
		t.Fatal("legacy read bypass", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleWorkSearch(w, httptest.NewRequest("GET", "/api/v2/work/search?query=confidential", nil))
	if strings.Contains(w.Body.String(), "confidential task") {
		t.Fatal("search leaked", w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleRuns(w, httptest.NewRequest("GET", "/v1/runs", nil))
	if strings.Contains(w.Body.String(), "confidential") {
		t.Fatal("activity summary leaked", w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleRuns(w, httptest.NewRequest("GET", "/v1/runs/"+task.ID+"/events", nil))
	if w.Code != 403 {
		t.Fatal("legacy events bypass", w.Code, w.Body.String())
	}
	if w := request("POST", base+"/cancel", `{}`); w.Code != 202 || strings.Contains(w.Body.String(), "confidential") {
		t.Fatal("cancel blocked or leaked", w.Code, w.Body.String())
	}
	if w := request("DELETE", base, ""); w.Code != 200 {
		t.Fatal("revocation trapped task", w.Code, w.Body.String())
	}
}
