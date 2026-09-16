package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestAgentHTTPApprovalResumesPersistedRun(t *testing.T) {
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	registry := agents.NewToolRegistry()
	registry.SetCapabilityBroker(capabilities.NewBroker(nil))
	var executed atomic.Int32
	registry.RegisterTool(api.Tool{Type: "function", Function: api.FunctionDef{Name: "test_write"}}, func(context.Context, json.RawMessage) (string, error) { executed.Add(1); return "saved", nil })
	runner := agents.NewRunner(manager, registry, func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		message := api.ChatMessage{Role: "assistant", Content: "Finished"}
		if len(messages) == 2 {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: "test_write", Arguments: `{"value":1}`}}}
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message}}}, nil
	})
	task, err := runner.Create("write", "model", "local-admin", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	task, err = runner.Continue(context.Background(), task.ID, "local-admin", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{agentManager: manager, agentRunner: runner}
	request := func(action, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handleAgentTaskAction(w, httptest.NewRequest(http.MethodPost, "/v1/agents/tasks/"+task.ID+"/"+action, strings.NewReader(body)))
		return w
	}
	bad := request("approve", `{"approval_id":"fake"}`)
	if bad.Code != 409 {
		t.Fatalf("invalid approval: %d %s", bad.Code, bad.Body.String())
	}
	bad = request("approve", `{"approval_id":"fake","prompt":"replacement"}`)
	if bad.Code != 400 {
		t.Fatalf("accepted prompt replacement: %d", bad.Code)
	}
	body := `{"approval_id":"` + task.PendingApproval.ID + `"}`
	w := request("approve", body)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"completed"`) {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}
	if w = request("approve", body); w.Code != 409 || executed.Load() != 1 {
		t.Fatalf("duplicate: %d, calls %d", w.Code, executed.Load())
	}
	w = httptest.NewRecorder()
	s.handleAgentTaskAction(w, httptest.NewRequest("GET", "/v1/agents/tasks/"+task.ID, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"output":"Finished"`) || strings.Contains(w.Body.String(), `"checkpoint"`) {
		t.Fatalf("run snapshot: %d %s", w.Code, w.Body.String())
	}
}

func TestAgentRejectsPreapprovalOnNewRun(t *testing.T) {
	s := &Server{}
	for _, field := range []string{"approved_tools", "approved_tool_calls"} {
		w := httptest.NewRecorder()
		s.handleAgentRun(w, httptest.NewRequest("POST", "/v1/agents/run", strings.NewReader(`{"model":"model","prompt":"write","`+field+`":[]}`)))
		if w.Code != 400 {
			t.Fatalf("%s: %d", field, w.Code)
		}
	}
}

func TestRunSnapshotsRemainVisibleWithoutEventLog(t *testing.T) {
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	runner := agents.NewRunner(manager, agents.NewToolRegistry(), nil)
	task, err := runner.Create("pending task", "model", "local-admin", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{agentManager: manager, agentRunner: runner}
	w := httptest.NewRecorder()
	s.handleRuns(w, httptest.NewRequest("GET", "/v1/runs", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), task.ID) || !strings.Contains(w.Body.String(), `"event_history_available":false`) {
		t.Fatalf("hidden snapshot: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	s.handleRuns(w, httptest.NewRequest("GET", "/v1/runs/"+task.ID+"/events", nil))
	if w.Code != 503 {
		t.Fatalf("event history claimed available: %d", w.Code)
	}
}
