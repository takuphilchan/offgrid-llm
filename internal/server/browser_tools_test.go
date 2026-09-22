package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTransientCaptureIsAttachedOnlyToNextVisionTurn(t *testing.T) {
	server := newTestServer(t)
	modelName := "qwen2.5-vl-7b-vision"
	for name, content := range map[string]string{modelName + ".gguf": "model", "qwen2.5-vl-7b-vision-mmproj-f16.gguf": "projector"} {
		if err := os.WriteFile(filepath.Join(server.config.ModelsDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := server.registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	key, ok := server.visionCheckKey(modelName)
	if !ok {
		t.Fatal("vision fixture was not discovered")
	}
	server.computerVisionChecks.Store(key, struct{}{})
	code, _ := server.browserHub.PairCode("alice")
	session, token, err := server.browserHub.Pair(code, "offgrid-demo://research", computer.ProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.browserHub.Reserve("alice", session.ID, "run"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := server.browserHub.Execute(context.Background(), "alice", session.ID, "run", "capture", "browser_capture", json.RawMessage(`{"observation_id":"observed"}`))
		done <- err
	}()
	var action *computer.BrowserAction
	deadline := time.Now().Add(time.Second)
	for action == nil && time.Now().Before(deadline) {
		action, _ = server.browserHub.Poll(token)
		time.Sleep(time.Millisecond)
	}
	if action == nil {
		t.Fatal("capture action not delivered")
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	reference, err := server.browserHub.StoreCapture(token, action.ID, "observed", "image/png", 2, 2, base64.StdEncoding.EncodeToString(imageData.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	result, _ := json.Marshal(map[string]any{"captured": true, "image_ref": reference, "observation_id": "observed", "width": 2, "height": 2})
	if err := server.browserHub.Reply(token, computer.BrowserReply{ID: action.ID, Result: string(result)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	task := &agents.Task{ID: "run", Actor: "alice", Model: modelName, Config: agents.AgentConfig{ComputerSession: session.ID, ComputerDriver: "browser"}}
	messages := []api.ChatMessage{{Role: "tool", Name: "browser_capture", Content: string(result)}}
	prepared, err := server.attachTransientComputerCapture(task, messages)
	if err != nil || len(prepared) != 2 {
		t.Fatalf("capture not attached: %v", err)
	}
	if strings.Contains(messages[0].StringContent(), "base64") || !strings.Contains(prepared[1].StringContent(), "Current selected-browser viewport") {
		t.Fatal("capture bytes leaked into durable result or text projection was lost")
	}
	if _, err := server.attachTransientComputerCapture(task, messages); err == nil {
		t.Fatal("transient image reference was reusable")
	}
}

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
		if len(available) != 9 {
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
