package server

import (
	"context"
	"encoding/json"
	"errors"
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

type probeEngine struct {
	inference.Engine
	mode     string
	requests int
	delay    time.Duration
}

type rawProbeBudgetEngine struct {
	inference.Engine
	calls int
}

func (e *rawProbeBudgetEngine) ChatCompletionStreamRaw(_ context.Context, request *api.ChatCompletionRequest, callback inference.ChatCompletionStreamCallback) error {
	e.calls++
	if !request.Stream || request.StreamOptions == nil || !request.StreamOptions.IncludeUsage || request.MaxTokens == nil || *request.MaxTokens != 192 {
		return errors.New("streamed probe did not request bounded usage")
	}
	return callback(json.RawMessage(`{"choices":[],"usage":{"completion_tokens":193}}`))
}

func pairedBrowserForPreflight(t *testing.T) (*computer.BrowserHub, string, string) {
	t.Helper()
	hub := computer.NewBrowserHub()
	code, err := hub.PairCode("local-admin")
	if err != nil {
		t.Fatal(err)
	}
	session, token, err := hub.Pair(code, "offgrid-demo://research", computer.ProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	return hub, session.ID, token
}
func (e *probeEngine) ChatCompletion(ctx context.Context, request *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if e.delay > 0 {
		select {
		case <-time.After(e.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.requests++
	if request.ToolChoice != "auto" || len(request.Tools) != 9 || request.ParallelToolCalls == nil || *request.ParallelToolCalls || !strings.Contains(request.Messages[0].StringContent(), computerSequentialProtocol) {
		return nil, errors.New("incorrect probe configuration")
	}
	if e.mode == "runtime" {
		return nil, errors.New("backend disconnected")
	}
	message := api.ChatMessage{Role: "assistant", Content: "I would simulate the action"}
	finish := "stop"
	if e.mode != "prose" {
		name, args := "browser_observe", "{}"
		if len(request.Messages) > 2 {
			name = "browser_verify"
			var data map[string]string
			var observed struct {
				Heading string `json:"heading"`
			}
			_ = json.Unmarshal([]byte(request.Messages[len(request.Messages)-1].StringContent()), &observed)
			data = map[string]string{"text": observed.Heading}
			if e.mode == "wrong-text" {
				data["text"] = "pretend success"
			}
			encoded, _ := json.Marshal(data)
			args = string(encoded)
		}
		if e.mode == "shell" {
			name = "shell"
		}
		if e.mode == "extra-args" {
			args = `{"url":"http://localhost"}`
		}
		if e.mode == "null" {
			args = "null"
		}
		message.ToolCalls = []api.ToolCall{{ID: "probe-call", Type: "function", Function: api.FunctionCall{Name: name, Arguments: args}}}
		if e.mode == "multiple" {
			message.ToolCalls = append(message.ToolCalls, api.ToolCall{ID: "premature-verify", Type: "function", Function: api.FunctionCall{Name: "browser_verify", Arguments: `{"text":"invented result"}`}})
		}
		finish = "tool_calls"
		if e.mode == "truncated" {
			finish = "length"
		}
	}
	response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: finish}}}
	if e.mode == "over-budget" {
		response.Usage.CompletionTokens = 193
	}
	return response, nil
}

func TestComputerModelCheckAdmissionDoesNotConsumeProbeBudget(t *testing.T) {
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	engine := &probeEngine{mode: "good"}
	s := &Server{config: &config.Config{MaxContextSize: 8192, ModelLoadTimeout: 2}, engine: engine, inferenceLifecycle: gate}
	// The model's existing request lasts longer than the entire probe budget.
	// Waiting must neither interrupt it nor expire a probe that has not started.
	go func() { time.Sleep(200 * time.Millisecond); release() }()
	result := s.checkComputerModelDriverWithBudget(context.Background(), "model", "browser", 100*time.Millisecond)
	if !result.Passed || engine.requests != 2 {
		t.Fatalf("cold/queued admission stole probe budget: %+v calls=%d", result, engine.requests)
	}
	if result.Timings == nil || result.Timings.AdmissionMS < 100 || result.Timings.ProbeMS >= 100 {
		t.Fatalf("admission/probe timing not separated: %+v", result.Timings)
	}
	if gate.Status().Active != 0 {
		t.Fatal("probe leaked inference lease")
	}
}

func TestComputerModelCheckReportsProbeDeadlineAndCancellation(t *testing.T) {
	for _, phase := range []string{"probe", "admission"} {
		t.Run(phase, func(t *testing.T) {
			gate := inference.NewLifecycleGate(1)
			release, err := gate.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			engine := &probeEngine{mode: "good", delay: time.Second}
			s := &Server{config: &config.Config{MaxContextSize: 8192}, engine: engine, inferenceLifecycle: gate}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := "computer_model_check_timeout"
			if phase == "admission" {
				cancel()
				want = "computer_model_check_cancelled"
			} else {
				release()
			}
			result := s.checkComputerModelDriverWithBudget(ctx, "model", "browser", 20*time.Millisecond)
			if result.Passed || !result.Retryable || result.Code != want {
				t.Fatalf("wrong phase failure: %+v", result)
			}
			if strings.Contains(result.Message, "Wait for current inference") {
				t.Fatal("probe failure blamed on other work")
			}
			if phase == "probe" && gate.Status().Active != 0 {
				t.Fatal("failed probe leaked lease")
			}
			if phase == "admission" && gate.Status().Active != 1 {
				t.Fatal("cancelled admission interrupted existing inference")
			}
		})
	}
}

func TestComputerModelProbeRequiresActualTwoStepCalls(t *testing.T) {
	for _, mode := range []string{"good", "prose", "shell", "extra-args", "null", "wrong-text", "multiple", "truncated", "runtime", "over-budget"} {
		t.Run(mode, func(t *testing.T) {
			engine := &probeEngine{mode: mode}
			err := probeComputerTools(context.Background(), engine, "fixture")
			if (err == nil) != (mode == "good") {
				t.Fatalf("%s: %v", mode, err)
			}
			if mode == "good" && engine.requests != 2 {
				t.Fatal("missing second check")
			}
			if mode == "runtime" && errors.Is(err, errComputerToolCalling) {
				t.Fatal("transport failure reported as incompatibility")
			}
			if mode == "multiple" && (!errors.Is(err, errComputerToolCalling) || !strings.Contains(err.Error(), "multiple tool calls")) {
				t.Fatalf("missing safe sequential failure detail: %v", err)
			}
			if mode == "over-budget" && (!errors.Is(err, errProbeOutputBudget) || engine.requests != 1) {
				t.Fatalf("over-budget synthetic answer was accepted or continued: calls=%d err=%v", engine.requests, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(probeComputerTools(ctx, &probeEngine{mode: "good"}, "fixture"), context.Canceled) {
		t.Fatal("cancellation ignored")
	}
	for _, tool := range browserTools() {
		if required, ok := tool.Function.Parameters["required"]; ok && required == nil {
			t.Fatal("invalid null schema required")
		}
	}
}

func TestStreamedComputerModelProbeRejectsReportedOutputOveruse(t *testing.T) {
	engine := &rawProbeBudgetEngine{}
	if err := probeComputerTools(context.Background(), engine, "fixture"); !errors.Is(err, errProbeOutputBudget) || engine.calls != 1 {
		t.Fatalf("streamed overuse escaped before the second check: calls=%d err=%v", engine.calls, err)
	}
}

func TestComputerSubmissionBlocksUnsupportedModelWithoutCreatingTask(t *testing.T) {
	hub, session, token := pairedBrowserForPreflight(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "model.gguf"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := models.NewRegistry(dir)
	if err := registry.ScanModels(); err != nil {
		t.Fatal(err)
	}
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	engine := &probeEngine{mode: "prose"}
	server := &Server{config: &config.Config{MaxContextSize: 8192}, registry: registry, browserHub: hub, engine: engine, inferenceLifecycle: gate, agentManager: manager, agentRunner: agents.NewRunner(manager, agents.NewToolRegistry(), nil)}
	for _, mode := range []string{"prose", "runtime"} {
		engine.mode = mode
		w := httptest.NewRecorder()
		body := `{"model":"model","prompt":"test","computer_session":"` + session + `","async":true}`
		server.handleAgentRun(w, httptest.NewRequest("POST", "/v1/agents/run", strings.NewReader(body)))
		expected := 422
		if mode == "runtime" {
			expected = 503
		}
		if w.Code != expected {
			t.Fatalf("%s: %d %s", mode, w.Code, w.Body.String())
		}
		if len(manager.ListTasks()) != 0 {
			t.Fatal("rejected check created task")
		}
		if action, err := hub.Poll(token); err != nil || action != nil {
			t.Fatal("preflight dispatched action", err)
		}
	}
}

func TestSavedTaskPreflightFailureIsRetryableWithoutDispatch(t *testing.T) {
	s := taskFirstServer(t)
	engine := &probeEngine{mode: "runtime"}
	s.engine = engine
	s.inferenceLifecycle = inference.NewLifecycleGate(1)
	release, err := s.inferenceLifecycle.AcquireWithContext(context.Background(), "model", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.ContextWindow = true, 8192
	task, err := s.agentRunner.Create("Read the browser", "model", "local-admin", cfg)
	if err != nil {
		t.Fatal(err)
	}
	task, err = s.agentRunner.Continue(context.Background(), task.ID, task.Actor, "start", "", false)
	if err != nil || task.PendingInput == nil {
		t.Fatal("missing pending input", err)
	}
	var session, token string
	s.browserHub, session, token = pairedBrowserForPreflight(t)
	inputID := task.PendingInput.ID
	for _, mode := range []string{"runtime", "prose"} {
		engine.mode = mode
		w := httptest.NewRecorder()
		body := `{"input_id":"` + inputID + `","computer_session":"` + session + `"}`
		s.handleJobInput(w, httptest.NewRequest("POST", "/api/v2/jobs/"+task.ID+"/input", strings.NewReader(body)), task.ID)
		status := 503
		if mode == "prose" {
			status = 422
		}
		if w.Code != status {
			t.Fatalf("%s: %d %s", mode, w.Code, w.Body.String())
		}
		saved, _ := s.agentManager.GetTask(task.ID)
		if saved.Status != agents.TaskInput || saved.PendingInput.ID != inputID || saved.Config.ComputerSession != "" {
			t.Fatal("failed preflight changed held task")
		}
		if action, err := s.browserHub.Poll(token); err != nil || action != nil {
			t.Fatal("failed check dispatched", err)
		}
	}
}
