package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type lateAgentEngine struct {
	inference.Engine
	entered chan struct{}
	finish  chan struct{}
}

func (e *lateAgentEngine) ChatCompletion(context.Context, *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	close(e.entered)
	<-e.finish // Deliberately ignores the cancelled generation context.
	return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Role: "assistant", Content: "late success"}, FinishReason: "stop"}}}, nil
}

func TestAgentStreamRejectsLateModelSuccessAfterStop(t *testing.T) {
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "fixture", 4096, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	engine := &lateAgentEngine{Engine: inference.NewMockEngine(), entered: make(chan struct{}), finish: make(chan struct{})}
	s := &Server{engine: engine, inferenceLifecycle: gate}
	task := &agents.Task{Model: "fixture", Config: agents.AgentConfig{ContextWindow: 4096, MaxTokens: 32}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		response, err := s.streamAgentModel(ctx, task, []api.ChatMessage{{Role: "user", Content: "test"}}, nil, func(string, string) error { return nil })
		if response != nil {
			done <- errors.New("late response crossed the agent boundary")
			return
		}
		done <- err
	}()
	<-engine.entered
	cancel()
	close(engine.finish)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("late model response was not cancelled: %v", err)
	}
}

func TestAgentDecisionPreservesComputerBatchRepair(t *testing.T) {
	parallel := false
	request := &api.ChatCompletionRequest{ParallelToolCalls: &parallel}
	response := &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{ToolCalls: []api.ToolCall{
		{ID: "reused", Type: "function", Function: api.FunctionCall{Name: "computer_observe", Arguments: `{}`}},
		{ID: "reused", Type: "function", Function: api.FunctionCall{Name: "computer_activate", Arguments: `{}`}},
	}}}}}
	task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "fixture"}}
	if err := validateAgentDecision(task, request, response); err != nil {
		t.Fatal("runner's whole-batch rejection was bypassed by permanent failure", err)
	}
	task.Config.ComputerSession = ""
	if err := validateAgentDecision(task, request, response); err == nil {
		t.Fatal("non-computer batch escaped the decision gate")
	}
	response.Choices[0].FinishReason = "length"
	task.Config.ComputerSession = "fixture"
	if _, ok := validateAgentDecision(task, request, response).(*inference.DecisionValidationError); !ok {
		t.Fatal("truncation bypassed the gate")
	}
}

func TestAgentStreamAssemblesToolsWithoutLeakingReasoningOrArguments(t *testing.T) {
	a := agentStreamAccumulator{calls: map[int]*api.ToolCall{}}
	chunks := []string{
		`{"choices":[{"index":0,"delta":{"reasoning_content":"PRIVATE REASONING","content":"Checking "}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"the result","tool_calls":[{"index":0,"id":"call1","type":"function","function":{"name":"calculate","arguments":"{\"secret\":"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"42}"}}]},"finish_reason":"tool_calls"}]}`,
		`{"choices":[],"usage":{"completion_tokens":20}}`,
	}
	var preview string
	for _, chunk := range chunks {
		delta, _, err := a.add(json.RawMessage(chunk))
		if err != nil {
			t.Fatal(err)
		}
		preview += delta
	}
	response := a.response()
	choice := response.Choices[0]
	if preview != "Checking the result" || choice.FinishReason != "tool_calls" || len(choice.Message.ToolCalls) != 1 || choice.Message.ToolCalls[0].Function.Arguments != `{"secret":42}` {
		t.Fatalf("bad response: %+v", response)
	}
}

func TestAgentStreamRejectsMalformedAndOversizedCalls(t *testing.T) {
	for _, chunk := range []string{
		`{"choices":[],"choices":[{"index":0,"delta":{"content":"hidden"}}]}`,
		`{"choices":[{"index":0,"delta":{"content":"\ud800"}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"function":{"name":"x"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":16,"function":{"name":"x"}}]}}]}`,
		`{"choices":[{"index":1,"delta":{"content":"wrong choice"}}]}`,
		`{"error":{"message":"private backend details"}}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"` + strings.Repeat("a", 256*1024+1) + `"}}]}}]}`,
	} {
		a := agentStreamAccumulator{calls: map[int]*api.ToolCall{}}
		if _, _, err := a.add(json.RawMessage(chunk)); err == nil {
			t.Fatal("accepted invalid chunk")
		}
	}
	a := agentStreamAccumulator{calls: map[int]*api.ToolCall{}}
	_, _, _ = a.add(json.RawMessage(`{"choices":[{"delta":{"content":"partial"}}]}`))
	if a.response().Choices[0].FinishReason != "" {
		t.Fatal("invented terminal reason")
	}
}

func TestAgentProgressReconnectRecoversSnapshotAndDisconnectDoesNotCancel(t *testing.T) {
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	runner := agents.NewRunner(manager, agents.NewToolRegistry(), nil)
	started, finish := make(chan struct{}), make(chan struct{})
	runner.StreamCaller = func(ctx context.Context, task *agents.Task, messages []api.ChatMessage, tools []api.Tool, emit func(string, string) error) (*api.ChatCompletionResponse, error) {
		if err := emit("generating", "Hello before completion"); err != nil {
			return nil, err
		}
		close(started)
		select {
		case <-finish:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: api.ChatMessage{Content: "Finished"}, FinishReason: "stop"}}}, nil
	}
	task, _ := runner.Create("test", "model", "local-admin", agents.DefaultAgentConfig())
	_, err := runner.Continue(context.Background(), task.ID, "local-admin", "start", "", true)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		_ = runner.Shutdown(ctx)
	})
	s := &Server{agentManager: manager, agentRunner: runner}
	httpServer := httptest.NewServer(http.HandlerFunc(s.handleAgentTaskAction))
	defer httpServer.Close()
	for i := 0; i < 2; i++ {
		response, err := http.Get(httpServer.URL + "/v1/agents/tasks/" + task.ID + "/events")
		if err != nil {
			t.Fatal(err)
		}
		line, err := bufio.NewReader(response.Body).ReadString('\n')
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(line, "Hello before completion") || strings.Contains(line, "checkpoint") || !strings.Contains(line, `"status":"running"`) {
			t.Fatalf("bad snapshot: %s", line)
		}
	}
	saved, _ := manager.GetTask(task.ID)
	if saved.Status != agents.TaskRunning {
		t.Fatal("viewer disconnect cancelled run")
	}
	other, _ := runner.Create("private", "model", "other-owner", agents.DefaultAgentConfig())
	denied := httptest.NewRecorder()
	s.handleAgentTaskAction(denied, httptest.NewRequest("GET", "/v1/agents/tasks/"+other.ID+"/events", nil))
	if denied.Code != 404 {
		t.Fatal("cross-owner streaming allowed")
	}
	close(finish)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		saved, _ = manager.GetTask(task.ID)
		if saved.Status == agents.TaskCompleted {
			return
		}
		time.Sleep(time.Millisecond * 10)
	}
	t.Fatal("run did not complete after disconnect")
}
