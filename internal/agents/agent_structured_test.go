package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestStructuredAgentUsesNativeToolMessages(t *testing.T) {
	tool := api.Tool{Type: "function", Function: api.FunctionDef{Name: "lookup"}}
	calls := 0
	caller := func(_ context.Context, messages []api.ChatMessage, tools []api.Tool, _ map[string]interface{}) (*api.ChatCompletionResponse, error) {
		calls++
		if len(tools) != 1 || tools[0].Function.Name != "lookup" {
			t.Fatalf("tools were not forwarded: %#v", tools)
		}
		if calls == 1 {
			return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "tool_calls", Message: api.ChatMessage{
				Role: "assistant",
				ToolCalls: []api.ToolCall{{ID: "call-1", Type: "function", Function: api.FunctionCall{
					Name: "lookup", Arguments: `{"key":"answer"}`,
				}}},
			}}}}, nil
		}
		last := messages[len(messages)-1]
		if last.Role != "tool" || last.ToolCallID != "call-1" || last.StringContent() != "42" {
			t.Fatalf("native tool result was not preserved: %#v", last)
		}
		return &api.ChatCompletionResponse{Choices: []api.ChatCompletionChoice{{FinishReason: "stop", Message: api.ChatMessage{Role: "assistant", Content: "The answer is 42."}}}}, nil
	}
	executor := func(_ context.Context, name string, arguments json.RawMessage) (string, error) {
		if name != "lookup" || string(arguments) != `{"key":"answer"}` {
			t.Fatalf("unexpected call: %s %s", name, arguments)
		}
		return "42", nil
	}
	config := DefaultAgentConfig()
	config.MaxIterations = 2
	agent := NewStructuredAgent(config, []api.Tool{tool}, executor, caller)
	answer, err := agent.Run(context.Background(), "Find the answer")
	if err != nil {
		t.Fatal(err)
	}
	if answer != "The answer is 42." {
		t.Fatalf("unexpected answer %q", answer)
	}
}

func TestRegistryRejectsDisabledToolExecution(t *testing.T) {
	registry := NewToolRegistry()
	if err := registry.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}
	_, err := registry.Execute(context.Background(), "calculator", json.RawMessage(`{"expression":"2+2"}`))
	if err == nil {
		t.Fatal("disabled tool was executed")
	}
}

func TestRegistryCapabilityPolicy(t *testing.T) {
	registry := NewToolRegistry()
	registry.SetCapabilityBroker(capabilities.NewBroker(capabilities.DefaultPolicy{}))

	result, err := registry.ExecuteWithPolicy(context.Background(), "calculator", json.RawMessage(`{"expression":"2+2"}`), ToolExecution{RunID: "run-1"})
	if err != nil || result != "4" {
		t.Fatalf("low-risk tool should execute without approval: %q %v", result, err)
	}

	_, err = registry.ExecuteWithPolicy(context.Background(), "write_file", json.RawMessage(`{"path":"ignored","content":"ignored"}`), ToolExecution{RunID: "run-1"})
	if !errors.Is(err, capabilities.ErrApprovalRequired) {
		t.Fatalf("high-risk tool should require approval, got %v", err)
	}
}

func TestRegistryApprovedHighRiskExtensionExecutes(t *testing.T) {
	registry := NewToolRegistry()
	called := false
	registry.RegisterTool(api.Tool{Type: "function", Function: api.FunctionDef{Name: "mutate"}}, func(context.Context, json.RawMessage) (string, error) {
		called = true
		return "done", nil
	})
	registry.SetCapabilityBroker(capabilities.NewBroker(capabilities.DefaultPolicy{}))
	result, err := registry.ExecuteWithPolicy(context.Background(), "mutate", nil, ToolExecution{RunID: "run-2", Actor: "admin", Approved: true})
	if err != nil || result != "done" || !called {
		t.Fatalf("approved extension did not execute: %q called=%v err=%v", result, called, err)
	}
}

func TestCalculatorDoesNotEvaluateCode(t *testing.T) {
	if value, err := executeCalculator("sqrt(16) + 2"); err != nil || value != "6" {
		t.Fatalf("safe expression failed: %q %v", value, err)
	}
	_, err := executeCalculator(`__import__("os")`)
	if err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("code-like expression was not rejected: %v", err)
	}
}
