package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestComputerTaskUsesDeterministicSequentialToolProtocol(t *testing.T) {
	config := agents.DefaultAgentConfig()
	config.SystemPrompt = "Custom task context."
	configureComputerTask(&config)
	if config.Temperature != 0 || !strings.HasPrefix(config.SystemPrompt, "Custom task context.") {
		t.Fatal("computer sampling or custom context lost")
	}
	for _, instruction := range []string{"first tool call must be browser_observe", "never invent them", "call browser_observe again", "one tool call at a time", "untrusted data"} {
		if !strings.Contains(config.SystemPrompt, instruction) {
			t.Errorf("missing protocol instruction: %s", instruction)
		}
	}
	if agents.DefaultAgentConfig().Temperature != 0.7 {
		t.Fatal("ordinary agent defaults changed")
	}
}

func TestComputerRequestPolicyDoesNotChangeOrdinaryAgents(t *testing.T) {
	ordinary := &api.ChatCompletionRequest{}
	configureComputerRequest(ordinary, &agents.Task{})
	if ordinary.ParallelToolCalls != nil {
		t.Fatal("ordinary runtime default changed")
	}
	computer := &api.ChatCompletionRequest{}
	configureComputerRequest(computer, &agents.Task{Config: agents.AgentConfig{ComputerSession: "paired"}})
	if computer.ParallelToolCalls == nil || *computer.ParallelToolCalls {
		t.Fatal("computer request must explicitly disable parallel calls")
	}
}

func TestBrowserToolsUseDurableApprovalsAndCannotEscapeRegistry(t *testing.T) {
	hub := computer.NewBrowserHub()
	code, _ := hub.PairCode("alice")
	session, token, err := hub.Pair(code, "https://example.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	registry := agents.NewToolRegistry()
	registry.SetCapabilityBroker(capabilities.NewBroker(nil))
	server := &Server{browserHub: hub, agentManager: manager}
	tools := &browserRunTools{RunTools: registry, server: server}
	caller := func(_ context.Context, _ *agents.Task, messages []api.ChatMessage, available []api.Tool) (*api.ChatCompletionResponse, error) {
		if len(available) != 7 {
			t.Fatal("unscoped tools supplied")
		}
		if len(messages) == 2 {
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "observe-call", Type: "function", Function: api.FunctionCall{Name: "browser_observe", Arguments: `{}`}}}}}}}, nil
		}
		if len(messages) == 4 {
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "model-call", Type: "function", Function: api.FunctionCall{Name: "browser_navigate", Arguments: `{"url":"https://example.com/notes"}`}}}}}}}, nil
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "Finished"}}}}, nil
	}
	runner := agents.NewRunner(manager, tools, caller)
	config := agents.DefaultAgentConfig()
	config.ComputerSession = session.ID
	config.ComputerExpectedText = "Notes"
	task, err := runner.Create("Inspect notes", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	observed := make(chan error, 1)
	go func() {
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
			a, e := hub.Poll(token)
			if e != nil {
				observed <- e
				return
			}
			if a != nil {
				observed <- hub.Reply(token, computer.BrowserReply{ID: a.ID, Result: `{"observation_id":"test","elements":[]}`})
				return
			}
			time.Sleep(time.Millisecond)
		}
		observed <- errors.New("observation missing")
	}()
	task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if e := <-observed; e != nil {
		t.Fatal(e)
	}
	if err != nil || task.Status != agents.TaskWaiting {
		t.Fatalf("approval missing: %+v %v", task, err)
	}
	if a, _ := hub.Poll(token); a != nil {
		t.Fatal("action dispatched before approval")
	}
	if err = tools.Authorize(context.Background(), "shell", json.RawMessage(`{}`), agents.ToolExecution{RunID: task.ID, Actor: "alice", Approved: true}); !errors.Is(err, capabilities.ErrDenied) {
		t.Fatalf("escaped browser tools: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			a, e := hub.Poll(token)
			if e != nil {
				done <- e
				return
			}
			if a != nil {
				done <- hub.Reply(token, computer.BrowserReply{ID: a.ID, Result: `{"navigated":true}`})
				return
			}
			time.Sleep(time.Millisecond)
		}
		done <- errors.New("no approved command")
	}()
	task, err = runner.Continue(context.Background(), task.ID, "alice", "approve", task.PendingApproval.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if task.Status != agents.TaskFailed || !strings.Contains(task.Error, "verification") {
		t.Fatalf("unverified work declared complete: %+v", task)
	}
}

func TestCompanionTransportDoesNotBypassOtherAuthentication(t *testing.T) {
	s := &Server{browserHub: computer.NewBrowserHub()}
	handler := s.browserTransport(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	for _, path := range []string{"/api/v2/computer/sessions", "/api/v2/computer/companion/poll-extra", "/v1/agents/run"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", path, nil))
		if w.Code != 401 {
			t.Fatal("authentication bypass")
		}
	}
	for _, origin := range []string{"", "https://untrusted.example"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/v2/computer/companion/poll", strings.NewReader(`{}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		handler.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("unauthenticated companion: %d", w.Code)
		}
	}
}
