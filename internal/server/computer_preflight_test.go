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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e.requests++
	if request.ToolChoice != "auto" || len(request.Tools) != 7 || request.ParallelToolCalls == nil || *request.ParallelToolCalls || !strings.Contains(request.Messages[0].StringContent(), computerSequentialProtocol) {
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
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: finish}}}, nil
}

func TestComputerModelProbeRequiresActualTwoStepCalls(t *testing.T) {
	for _, mode := range []string{"good", "prose", "shell", "extra-args", "null", "wrong-text", "multiple", "truncated", "runtime"} {
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
