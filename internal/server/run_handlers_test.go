package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/artifacts"
	"github.com/takuphilchan/offgrid-llm/internal/runs"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestRunHistoryAndEventReplay(t *testing.T) {
	log, err := runs.NewLog(t.TempDir() + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{runLog: log}
	server.publishRunEvent(context.Background(), "run-1", runs.RunStarted, map[string]any{"model": "test-model"})
	server.publishRunEvent(context.Background(), "run-1", runs.RunCompleted, map[string]any{"ok": true})
	server.publishRunEvent(context.Background(), "computer-system", runs.ComputerSession, map[string]any{"state": "stopped"})

	recorder := httptest.NewRecorder()
	server.handleRuns(recorder, httptest.NewRequest(http.MethodGet, "/v1/runs", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var list struct {
		Runs []runSummary `json:"runs"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Runs) != 1 || list.Runs[0].Status != "completed" || list.Runs[0].EventCount != 2 {
		t.Fatalf("unexpected summaries: %#v", list.Runs)
	}

	recorder = httptest.NewRecorder()
	server.handleRuns(recorder, httptest.NewRequest(http.MethodGet, "/v1/runs/run-1/events?after=1", nil))
	var replay struct {
		Events []runs.Event `json:"events"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &replay); err != nil {
		t.Fatal(err)
	}
	if len(replay.Events) != 1 || replay.Events[0].Type != runs.RunCompleted {
		t.Fatalf("unexpected replay: %#v", replay.Events)
	}
}

func TestArtifactDownload(t *testing.T) {
	store, err := artifacts.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := store.Put(context.Background(), strings.NewReader("agent result"), artifacts.Metadata{MediaType: "text/plain"})
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{artifactStore: store}
	server.agentManager = agents.NewManager(nil, nil, nil)
	runner := agents.NewRunner(server.agentManager, agents.NewToolRegistry(), func(context.Context, *agents.Task, []api.ChatMessage, []api.Tool) (*api.ChatCompletionResponse, error) {
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", Content: "agent result"}, FinishReason: "stop"}}}, nil
	})
	task, _ := runner.Create("test", "model", "local-admin", agents.DefaultAgentConfig())
	if _, err := runner.Continue(context.Background(), task.ID, "local-admin", "start", "", false); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleArtifacts(recorder, httptest.NewRequest(http.MethodGet, "/v1/artifacts/"+metadata.Digest, nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "agent result" {
		t.Fatalf("artifact response = %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("ETag") == "" {
		t.Fatal("artifact response is missing ETag")
	}
}
