package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/models"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Test-only image reader for the synthetic glyph fixture. This exercises the
// real preflight path; it is not evidence of real-model visual qualification.
type jobVisionEngine struct {
	probeEngine
	visionCalls int
	wrong       bool
}

func (e *jobVisionEngine) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if len(req.Tools) != 1 {
		return e.probeEngine.ChatCompletion(ctx, req)
	}
	e.visionCalls++
	parts, ok := req.Messages[1].Content.([]api.ChatContentPart)
	if !ok || len(parts) != 2 || parts[1].ImageURL == nil {
		return nil, fmt.Errorf("missing synthetic image")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(parts[1].ImageURL.URL, "data:image/png;base64,"))
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var value strings.Builder
	for index := 0; index < 12; index++ {
		var actual [7]byte
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				r, _, _, _ := img.At(20+(index*6+col)*10+5, 20+row*10+5).RGBA()
				if r < 0x8000 {
					actual[row] |= 1 << uint(4-col)
				}
			}
		}
		for char, glyph := range visionGlyphs {
			if glyph == actual {
				value.WriteRune(char)
				break
			}
		}
	}
	text := value.String()
	if e.wrong {
		text = "incorrect"
	}
	args, _ := json.Marshal(map[string]string{"text": text})
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{Role: "assistant", ToolCalls: []api.ToolCall{{ID: "image-check", Type: "function", Function: api.FunctionCall{Name: "browser_verify", Arguments: string(args)}}}}}}}, nil
}

func TestTaskFirstBrowserInputChecksVisionAndPreservesStructuredFallback(t *testing.T) {
	for _, scenario := range []string{"missing", "valid", "wrong", "cached", "restart"} {
		t.Run(scenario, func(t *testing.T) {
			s := taskFirstServer(t)
			model := "qwen2.5vl-7b-test"
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, model+".gguf"), []byte("synthetic model"), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario != "missing" {
				projector := models.GetProjectorFilename(model, model+".gguf")
				if projector == "" {
					t.Fatal("fixture has no registered projector mapping")
				}
				if err := os.WriteFile(filepath.Join(dir, projector), []byte("synthetic projector"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			s.registry = models.NewRegistry(dir)
			if err := s.registry.ScanModels(); err != nil {
				t.Fatal(err)
			}
			engine := &jobVisionEngine{probeEngine: probeEngine{mode: "good"}, wrong: scenario == "wrong"}
			s.engine = engine
			s.inferenceLifecycle = inference.NewLifecycleGate(1)
			release, err := s.inferenceLifecycle.AcquireWithContext(context.Background(), model, 8192, func(context.Context, string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			release()
			if scenario == "cached" || scenario == "restart" {
				if check := s.checkComputerVision(context.Background(), model); !check.Passed {
					t.Fatal(check)
				}
				if scenario == "restart" {
					s.computerVisionChecks.Range(func(key, _ any) bool { s.computerVisionChecks.Delete(key); return true })
				}
			}
			cfg := agents.DefaultAgentConfig()
			cfg.TaskFirst = true
			cfg.ContextWindow = 8192
			task, err := s.agentRunner.Create("Read the browser", model, "local-admin", cfg)
			if err != nil {
				t.Fatal(err)
			}
			task, err = s.agentRunner.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
			if err != nil || task.PendingInput == nil {
				t.Fatalf("missing input: %+v %v", task, err)
			}
			s.browserHub, cfg.ComputerSession, _ = pairedBrowserForPreflight(t)
			body, _ := json.Marshal(map[string]string{"input_id": task.PendingInput.ID, "computer_session": cfg.ComputerSession})
			w := httptest.NewRecorder()
			s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/input", bytes.NewReader(body)), task.ID)
			if w.Code != 202 {
				t.Fatalf("structured flow blocked: %d %s", w.Code, w.Body.String())
			}
			want := scenario != "missing" && scenario != "wrong"
			if s.computerVisionReady(model) != want {
				t.Fatalf("incorrect vision state: %+v", engine)
			}
			calls := 1
			if scenario == "missing" {
				calls = 0
			}
			if scenario == "restart" {
				calls = 2
			}
			if engine.visionCalls != calls {
				t.Fatalf("vision calls %d, want %d", engine.visionCalls, calls)
			}
			task.Config.ComputerSession = cfg.ComputerSession
			task.Config.ComputerDriver = "browser"
			tools := (&browserRunTools{RunTools: agents.NewToolRegistry(), server: s}).ToolsForTask(task)
			capture, observe := false, false
			for _, tool := range tools {
				capture = capture || tool.Function.Name == "browser_capture"
				observe = observe || tool.Function.Name == "browser_observe"
			}
			if capture != want || !observe {
				t.Fatal("incorrect structured/vision tool availability")
			}
		})
	}
}

func taskFirstServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("test-only model fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	s := &Server{registry: registry, agentManager: agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir()), browserHub: computer.NewBrowserHub(), config: &config.Config{MaxContextSize: 8192}}
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	s.agentRunner = agents.NewRunner(s.agentManager, tools, func(_ context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool) (*api.ChatCompletionResponse, error) {
		msg := api.ChatMessage{Role: "assistant", Content: "Stopped without changes."}
		if len(messages) == 2 {
			found := false
			for _, tool := range tools {
				if tool.Function.Name == computerAccessToolName {
					found = true
				}
			}
			if !found {
				t.Error("task did not receive access tool")
			}
			msg.Content = ""
			msg.ToolCalls = []api.ToolCall{{Type: "function", Function: api.FunctionCall{Name: computerAccessToolName, Arguments: `{"mode":"browser","target":"Documentation","url":"https://example.com"}`}}}
		}
		reason := "stop"
		if len(msg.ToolCalls) > 0 {
			reason = "tool_calls"
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: msg, FinishReason: reason}}}, nil
	})
	t.Cleanup(func() { _ = s.agentRunner.Shutdown(context.Background()) })
	return s
}

func TestJobSubmissionPersistsInputAndDeduplicates(t *testing.T) {
	s := taskFirstServer(t)
	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handleJobSubmit(w, httptest.NewRequest("POST", "/api/v2/jobs", strings.NewReader(body)))
		return w
	}
	body := `{"prompt":"Read documentation","model":"model","request_id":"request-001"}`
	response := post(body)
	if response.Code != 202 {
		t.Fatalf("submit: %d %s", response.Code, response.Body.String())
	}
	var submitted struct {
		ID string `json:"run_id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &submitted); err != nil {
		t.Fatal(err)
	}
	if response.Header().Get("Location") != "/api/v2/jobs/"+submitted.ID {
		t.Fatal("no durable location")
	}
	deadline := time.Now().Add(5 * time.Second)
	var saved *agents.Task
	for time.Now().Before(deadline) {
		saved, _ = s.agentManager.GetTask(submitted.ID)
		if saved.Status == agents.TaskInput {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if saved.Status != agents.TaskInput || saved.PendingInput == nil || saved.Config.ComputerSession != "" {
		t.Fatalf("no saved interruption: %+v", saved)
	}
	response = post(body)
	if response.Code != 202 || !strings.Contains(response.Body.String(), submitted.ID) {
		t.Fatalf("retry created work: %s", response.Body.String())
	}
	if response = post(strings.Replace(body, "Read documentation", "Different work", 1)); response.Code != 409 {
		t.Fatal("conflicting request accepted")
	}
	if response = post(body + ` {}`); response.Code != 400 {
		t.Fatal("trailing envelope accepted")
	}
	read := httptest.NewRecorder()
	s.handleJobRead(read, httptest.NewRequest("GET", "/api/v2/jobs/"+submitted.ID, nil))
	if !strings.Contains(read.Body.String(), `"prompt":"Read documentation"`) {
		t.Fatal("owner snapshot missing task text")
	}
	// Ordinary and foreign tasks cannot acquire authority through the new tool.
	descriptor := computerAccessDescriptor()
	ordinary, err := s.agentRunner.Create("other", "model", "other-actor", agents.DefaultAgentConfig())
	if err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	if err := tools.Authorize(context.Background(), computerAccessToolName, json.RawMessage(`{}`), agents.ToolExecution{Actor: "local-admin", RunID: ordinary.ID, ExpectedCapability: &descriptor}); err == nil {
		t.Fatal("foreign task authorized")
	}
	input := httptest.NewRecorder()
	s.handleJobInput(input, httptest.NewRequest("POST", "/api/v2/jobs/"+ordinary.ID+"/input", strings.NewReader(`{"input_id":"fake","computer_session":"fake"}`)), ordinary.ID)
	if input.Code != 404 {
		t.Fatal("foreign task input visible")
	}
}

func TestJobAccessModelFailureRetainsTaskAndCannotReserveOtherScope(t *testing.T) {
	s := taskFirstServer(t)
	s.engine = &probeEngine{mode: "prose"}
	s.inferenceLifecycle = inference.NewLifecycleGate(1)
	release, err := s.inferenceLifecycle.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst = true
	cfg.ContextWindow = 8192
	task, err := s.agentRunner.Create("read documentation", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = s.agentRunner.Continue(context.Background(), task.ID, "local-admin", "start", "", false)
	if err != nil {
		t.Fatal(err)
	}
	code, _ := s.browserHub.PairCode("local-admin")
	host, _, err := s.browserHub.Pair(code, "offgrid-demo://research", computer.ProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"input_id":"` + task.PendingInput.ID + `","computer_session":"` + host.ID + `"}`
	w := httptest.NewRecorder()
	s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/input", strings.NewReader(body)), task.ID)
	if w.Code != 422 {
		t.Fatalf("bad model accepted: %d %s", w.Code, w.Body.String())
	}
	saved, _ := s.agentManager.GetTask(task.ID)
	if saved.Status != agents.TaskInput || saved.Config.ComputerSession != "" {
		t.Fatal("preflight changed saved task")
	}
	if err = s.browserHub.ReserveTask("local-admin", host.ID, "different-job"); err != nil {
		t.Fatal("failed preflight reserved access")
	}
	if err = s.browserHub.ReserveTask("local-admin", host.ID, task.ID); err == nil {
		t.Fatal("reserved session stolen")
	}
}

func TestInvalidAccessProposalsCannotBecomeConsent(t *testing.T) {
	for _, args := range []string{`{"mode":"app","target":"Editor","url":"https://example.com"}`, `{"mode":"browser","target":"site","url":"file:///etc/passwd"}`, `{"mode":"app","target":"Editor","url":"","approved":true}`, `{"mode":"desktop","target":"all","url":""}`} {
		err := computerAccessRequest(json.RawMessage(args))
		if _, ok := err.(*agents.InputRequired); ok || err == nil {
			t.Fatalf("invalid proposal accepted: %s", args)
		}
	}
}

func TestLegacyWorkflowRegistrationAndOrchestrationFailHonestly(t *testing.T) {
	s := &Server{}
	for _, handler := range []http.HandlerFunc{s.handleAgentWorkflows, s.handleAgentOrchestrate} {
		w := httptest.NewRecorder()
		handler(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"id":"pretend","prompt":"perform changes"}`)))
		if w.Code != 501 || !strings.Contains(w.Body.String(), "durable_coordination_unavailable") {
			t.Fatalf("claimed unsupported success: %d %s", w.Code, w.Body.String())
		}
		w = httptest.NewRecorder()
		handler(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"available":false`) {
			t.Fatal("false availability")
		}
	}
}
