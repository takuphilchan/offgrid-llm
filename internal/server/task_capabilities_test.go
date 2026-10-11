package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

type compositionPolicy struct{ denied atomic.Bool }

func (p *compositionPolicy) Evaluate(ctx context.Context, request capabilities.Request) capabilities.Result {
	if request.Actor != "alice" || p.denied.Load() {
		return capabilities.Result{Decision: capabilities.Deny}
	}
	return capabilities.DefaultPolicy{}.Evaluate(ctx, request)
}

func compositionFixture(t *testing.T) (*Server, *browserRunTools, *agents.ToolRegistry, *compositionPolicy, *atomic.Int32) {
	t.Helper()
	remote := agents.NewToolRegistry()
	var calls atomic.Int32
	remote.RegisterTool(api.Tool{Type: "function", Function: api.FunctionDef{Name: "docs", Description: "Read synthetic documentation", Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}}}, func(context.Context, json.RawMessage) (string, error) {
		calls.Add(1)
		return `{"text":"Synthetic notes"}`, nil
	})
	endpoint := httptest.NewServer(remote.MCPHTTPHandler("composition-fixture"))
	t.Cleanup(endpoint.Close)
	registry := agents.NewToolRegistry()
	policy := &compositionPolicy{}
	registry.SetCapabilityBroker(capabilities.NewBroker(policy))
	if _, err := registry.LoadMCPTools("docs", endpoint.URL); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := registry.RemoveMCPServer("docs"); err != nil {
			t.Error(err)
		}
	})
	s := &Server{agentManager: agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir()), browserHub: computer.NewBrowserHub()}
	adapter := &browserRunTools{RunTools: registry, server: s}
	return s, adapter, registry, policy, &calls
}

func compositionConfig(driver string, taskFirst bool) agents.AgentConfig {
	cfg := agents.DefaultAgentConfig()
	cfg.TaskFirst, cfg.ComputerDriver, cfg.ComputerSession = taskFirst, driver, "test-session"
	cfg.ComputerApprovalMode = "full_task"
	cfg.ContextWindow = 32768
	return cfg
}

func availableNames(tools []api.Tool) map[string]bool {
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Function.Name] = true
	}
	return names
}

func TestComposedDiscoveryAndProviderAuthorization(t *testing.T) {
	s, adapter, registry, policy, calls := compositionFixture(t)
	registry.RegisterTool(api.Tool{Type: "function", Function: api.FunctionDef{Name: "renamed_script", Parameters: map[string]interface{}{"type": "object"}}}, func(context.Context, json.RawMessage) (string, error) { t.Fatal("script executed"); return "", nil })
	for _, driver := range []string{"browser", "windows-uia", "macos-accessibility", "linux-atspi"} {
		t.Run(driver, func(t *testing.T) {
			runner := agents.NewRunner(s.agentManager, adapter, nil)
			task, err := runner.Create("Research notes", "model", "alice", compositionConfig(driver, true))
			if err != nil {
				t.Fatal(err)
			}
			names := availableNames(adapter.ToolsForTaskContext(context.Background(), task))
			observe := "computer_observe"
			if driver == "browser" {
				observe = "browser_observe"
			}
			if !names["mcp__docs__docs"] || !names[observe] || !names["calculator"] || names["shell"] || names["renamed_script"] {
				t.Fatalf("composition: %v", names)
			}
			if driver == "windows-uia" {
				desktop := *task
				desktop.Config.ComputerScope = "desktop"
				combined := availableNames(adapter.ToolsForTask(&desktop))
				if !combined["mcp__docs__docs"] || !combined["computer_applications"] || !combined["computer_open_application"] || combined[computerAccessToolName] {
					t.Fatal("desktop replaced MCP or retained redundant access requests", combined)
				}
			}
			for _, name := range []string{"mcp__docs__docs", "write_file"} {
				d, _ := adapter.Capability(name)
				e := agents.ToolExecution{Actor: "alice", RunID: task.ID, ExpectedCapability: &d}
				if err := adapter.Authorize(context.Background(), name, json.RawMessage(`{}`), e); !errors.Is(err, capabilities.ErrApprovalRequired) {
					t.Fatalf("computer policy leaked to %s: %v", name, err)
				}
				e.Actor = "bob"
				if err := adapter.Authorize(context.Background(), name, json.RawMessage(`{}`), e); !errors.Is(err, agents.ErrTaskNotFound) {
					t.Fatalf("wrong actor: %v", err)
				}
			}
			for _, name := range []string{"shell", "renamed_script"} {
				d, _ := adapter.Capability(name)
				if err := adapter.Authorize(context.Background(), name, json.RawMessage(`{}`), agents.ToolExecution{Actor: "alice", RunID: task.ID, Approved: true, ExpectedCapability: &d}); !errors.Is(err, capabilities.ErrDenied) {
					t.Fatalf("local execute permitted: %s %v", name, err)
				}
			}
			legacy := *task
			legacy.Config.TaskFirst = false
			if availableNames(adapter.ToolsForTask(&legacy))["mcp__docs__docs"] {
				t.Fatal("legacy grant broadened")
			}
			noAccess := *task
			noAccess.Config.ComputerSession = ""
			if availableNames(adapter.ToolsForTask(&noAccess))[observe] {
				t.Fatal("connection granted computer access")
			}
			policy.denied.Store(true)
			if availableNames(adapter.ToolsForTask(task))["mcp__docs__docs"] {
				t.Fatal("actor-denied tool disclosed")
			}
			d, _ := adapter.Capability("mcp__docs__docs")
			if _, err := adapter.ExecuteWithPolicy(context.Background(), d.Name, json.RawMessage(`{}`), agents.ToolExecution{Actor: "alice", RunID: task.ID, Approved: true, ExpectedCapability: &d}); err == nil {
				t.Fatal("policy changed but dispatched")
			}
			policy.denied.Store(false)
		})
	}
	if calls.Load() != 0 {
		t.Fatal("discovery/authorization executed MCP")
	}
}

func TestComposedHostPathCannotBecomeServiceFileAccess(t *testing.T) {
	s, adapter, _, _, _ := compositionFixture(t)
	runner := agents.NewRunner(s.agentManager, adapter, nil)
	task, err := runner.Create("Use a host file", "model", "alice", compositionConfig("windows-uia", true))
	if err != nil {
		t.Fatal(err)
	}
	path := `D:\private`
	if runtime.GOOS == "windows" {
		path = "/Users/alice/private"
	}
	args, _ := json.Marshal(map[string]string{"path": path})
	d, _ := adapter.Capability("read_file")
	err = adapter.Authorize(context.Background(), d.Name, args, agents.ToolExecution{Actor: "alice", RunID: task.ID, ExpectedCapability: &d})
	var denied *agents.ToolAuthorizationError
	if !errors.As(err, &denied) || denied.Code != "host_access_required" {
		t.Fatalf("host path escaped: %v", err)
	}
}

func TestComposedDurableMCPAndBrowserWorkflow(t *testing.T) {
	s, adapter, _, _, calls := compositionFixture(t)
	code, _ := s.browserHub.PairCode("alice")
	session, token, err := s.browserHub.Pair(code, "https://example.com", 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := compositionConfig("browser", true)
	cfg.ComputerSession = session.ID
	cfg.ComputerExpectedText = "Synthetic notes"
	index := 0
	sequence := []api.FunctionCall{{Name: "mcp__docs__docs", Arguments: `{}`}, {Name: "browser_observe", Arguments: `{}`}, {Name: "browser_verify", Arguments: `{"text":"Synthetic notes"}`}}
	caller := func(_ context.Context, _ *agents.Task, _ []api.ChatMessage, available []api.Tool) (*api.ChatCompletionResponse, error) {
		names := availableNames(available)
		if !names["mcp__docs__docs"] || !names["browser_observe"] {
			t.Fatal("provider disappeared")
		}
		message := api.ChatMessage{Role: "assistant", Content: "Verified the synthetic page notes."}
		reason := "stop"
		if index < len(sequence) {
			message.Content = ""
			message.ToolCalls = []api.ToolCall{{Type: "function", Function: sequence[index]}}
			reason = "tool_calls"
		}
		index++
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{Message: message, FinishReason: reason}}}, nil
	}
	runner := agents.NewRunner(s.agentManager, adapter, caller)
	task, err := runner.Create("Read documentation and inspect the page", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.browserHub.Reserve("alice", session.ID, task.ID); err != nil {
		t.Fatal(err)
	}
	task, err = runner.Continue(context.Background(), task.ID, "alice", "start", "", false)
	if err != nil || task.Status != agents.TaskWaiting || task.PendingApproval.Tool != "mcp__docs__docs" || calls.Load() != 0 {
		t.Fatalf("MCP approval/ordering: %+v %v", task, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		for n := 0; n < 2; {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case <-time.After(time.Millisecond):
			}
			a, e := s.browserHub.Poll(token)
			if e != nil {
				done <- e
				return
			}
			if a == nil {
				continue
			}
			result := `{"observation_id":"observed","text":"Synthetic notes","elements":[]}`
			if a.Kind == "browser_verify" {
				result = `{"verified":true,"check":"page_contains_text","text":"Synthetic notes"}`
			}
			if e = s.browserHub.Reply(token, computer.BrowserReply{ID: a.ID, Result: result}); e != nil {
				done <- e
				return
			}
			n++
		}
		done <- nil
	}()
	task, err = runner.Continue(ctx, task.ID, "alice", "approve", task.PendingApproval.ID, false)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if err != nil || task.Status != agents.TaskCompleted || calls.Load() != 1 {
		t.Fatalf("mixed workflow: %+v %v calls=%d", task, err, calls.Load())
	}
	if len(task.Steps) != 3 || task.Steps[0].Authorization != "exact_approval" || strings.HasPrefix(task.Steps[0].Authorization, "automatic:") {
		t.Fatalf("wrong provider evidence: %+v", task.Steps)
	}
	// A later remote call must not reuse the browser's prior outcome as current.
	task.Steps = append(task.Steps, agents.Step{Type: "action", ToolName: "mcp__docs__docs", ToolResult: `{"verified":true}`})
	if err := adapter.ValidateCompletion(task); err == nil {
		t.Fatal("MCP output fabricated computer verification")
	}
}
