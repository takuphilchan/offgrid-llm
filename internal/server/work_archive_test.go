package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

func TestWorkArchiveAPIAndProjection(t *testing.T) {
	m := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	s := &Server{agentManager: m, agentRunner: agents.NewRunner(m, nil, nil)}
	const id = "run-archive-legacy"
	if _, err := m.CreateTask(id, "historical test", nil); err != nil {
		t.Fatal(err)
	}
	if err := m.CompleteTask(id, "preserved result", nil); err != nil {
		t.Fatal(err)
	}
	command := func(method, action, body string, want int) {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/api/v2/jobs/"+id+"/"+action, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		s.handleJobCommand(w, req, id, action)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, action, w.Code, w.Body.String())
		}
	}
	read := func(bucket string) workPage {
		t.Helper()
		w := httptest.NewRecorder()
		s.handleWorkSummary(w, httptest.NewRequest("GET", "/api/v2/work?bucket="+bucket, nil))
		var page workPage
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &page) != nil {
			t.Fatalf("summary: %d %s", w.Code, w.Body.String())
		}
		return page
	}
	command("GET", "archive", "", 405)
	command("POST", "archive", `{"status":"completed"}`, 400)
	command("POST", "archive", "{}", 202)
	command("POST", "archive", "{}", 202)
	for _, bucket := range []string{"all", "attention", "running", "history"} {
		page := read(bucket)
		if len(page.Items) != 0 || page.Counts["archived"] != 1 || page.Counts["all"] != 0 {
			t.Fatalf("archive leaked into %s: %+v", bucket, page)
		}
	}
	page := read("archived")
	if len(page.Items) != 1 || page.Items[0].ArchivedAt == nil || page.Items[0].Status != agents.TaskCompleted {
		t.Fatalf("archive unavailable: %+v", page)
	}
	command("POST", "restore", "{}", 202)
	page = read("history")
	if len(page.Items) != 1 || page.Counts["archived"] != 0 || page.Counts["all"] != 1 {
		t.Fatalf("restore projection: %+v", page)
	}
	task, _ := m.GetTask(id)
	if task.Result != "preserved result" {
		t.Fatal("archive erased evidence")
	}
}
