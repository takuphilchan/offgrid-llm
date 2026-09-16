package server

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/runs"
	"github.com/takuphilchan/offgrid-llm/internal/sessions"
)

func TestDeleteSessionRejectsBusyWithoutWaiting(t *testing.T) {
	h := NewSessionHandlers(t.TempDir())
	if _, err := h.manager.WithAccess(sessions.Access{All: true}).Create("日本語-chat", "model"); err != nil {
		t.Fatal(err)
	}
	unlock := h.lockSession("日本語-chat")
	w := httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("DELETE", "/v1/sessions/日本語-chat", nil))
	unlock()
	if w.Code != 409 {
		t.Fatalf("busy deletion: %d %s", w.Code, w.Body.String())
	}
	if _, err := h.manager.Load("日本語-chat"); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.HandleSessions(w, httptest.NewRequest("DELETE", "/v1/sessions/日本語-chat", nil))
	if w.Code != 200 {
		t.Fatalf("idle deletion: %d %s", w.Code, w.Body.String())
	}
	if len(h.sessionLocks) != 0 {
		t.Fatal("leaked session locks")
	}
}

func TestDeletedAgentIsAbsentFromTasksAndActivityAfterRestart(t *testing.T) {
	dir := t.TempDir()
	m := agents.NewManagerWithPersistence(nil, nil, nil, dir)
	r := agents.NewRunner(m, nil, nil)
	log, err := runs.NewLog(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{agentManager: m, agentRunner: r, runLog: log}
	task, err := r.Create("history-test", "model", "local-admin", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := r.Create("other actor", "model", "someone-else", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handleAgentTaskAction(w, httptest.NewRequest(method, path, nil))
		return w
	}
	if w := request("DELETE", "/v1/agents/tasks/"+foreign.ID); w.Code != 404 {
		t.Fatalf("cross-actor: %d", w.Code)
	}
	if w := request("DELETE", "/v1/agents/tasks/"+task.ID); w.Code != 409 {
		t.Fatalf("deleted pending run: %d", w.Code)
	}
	if _, err := r.Stop(task.ID, "local-admin", "cancel", ""); err != nil {
		t.Fatal(err)
	}
	s.publishRunEvent(context.Background(), task.ID, runs.RunStateChanged, map[string]any{"status": "cancelled", "prompt": "history-test"})
	if w := request("DELETE", "/v1/agents/tasks/"+task.ID); w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	s.agentManager = agents.NewManagerWithPersistence(nil, nil, nil, dir)
	s.agentRunner = agents.NewRunner(s.agentManager, nil, nil)
	if w := request("GET", "/v1/agents/tasks/"+task.ID); w.Code != 404 {
		t.Fatalf("restored task: %d", w.Code)
	}
	for _, path := range []string{"/v1/runs", "/v1/agents/tasks"} {
		w := httptest.NewRecorder()
		if path == "/v1/runs" {
			s.handleRuns(w, httptest.NewRequest("GET", path, nil))
		} else {
			s.handleAgentTasks(w, httptest.NewRequest("GET", path, nil))
		}
		if w.Code != 200 || strings.Contains(w.Body.String(), task.ID) {
			t.Fatalf("ghost history: %d %s", w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.handleRuns(w, httptest.NewRequest("GET", "/v1/runs/"+task.ID+"/events", nil))
	if w.Code != 404 {
		t.Fatalf("deleted run events visible: %d", w.Code)
	}
}
