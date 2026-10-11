package agents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

type discoveryPolicy func(context.Context, capabilities.Request) capabilities.Result

func (p discoveryPolicy) Evaluate(ctx context.Context, request capabilities.Request) capabilities.Result {
	return p(ctx, request)
}

func TestRegistryDiscoveryAppliesActorPolicyWithoutApproving(t *testing.T) {
	r := NewToolRegistry()
	r.SetCapabilityBroker(capabilities.NewBroker(discoveryPolicy(func(ctx context.Context, request capabilities.Request) capabilities.Result {
		// Reentry must not deadlock: policy is evaluated outside the registry lock.
		_ = r.GetTools()
		if request.Actor != "alice" || request.RunID != "task" || request.Capability.Name == "http_get" {
			return capabilities.Result{Decision: capabilities.Deny}
		}
		return capabilities.DefaultPolicy{}.Evaluate(ctx, request)
	})))
	if err := r.DisableTool("current_time"); err != nil {
		t.Fatal(err)
	}
	available := r.ToolsForExecution(context.Background(), ToolExecution{Actor: "alice", RunID: "task", Approved: true})
	found := map[string]bool{}
	for _, tool := range available {
		found[tool.Function.Name] = true
	}
	if !found["calculator"] || !found["write_file"] || found["http_get"] || found["current_time"] {
		t.Fatalf("unexpected catalog: %v", found)
	}
	if len(r.ToolsForExecution(context.Background(), ToolExecution{Actor: "bob", RunID: "task"})) != 0 {
		t.Fatal("cross-actor discovery")
	}
	if err := r.Authorize(context.Background(), "write_file", json.RawMessage(`{}`), ToolExecution{Actor: "alice", RunID: "task"}); !errors.Is(err, capabilities.ErrApprovalRequired) {
		t.Fatalf("discovery transferred approval: %v", err)
	}
	if err := r.DisableTool("calculator"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ExecuteWithPolicy(context.Background(), "calculator", json.RawMessage(`{"expression":"2+2"}`), ToolExecution{Actor: "alice", RunID: "task"}); err == nil {
		t.Fatal("disabled after discovery executed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if len(r.ToolsForExecution(ctx, ToolExecution{Actor: "alice", RunID: "task"})) != 0 {
		t.Fatal("canceled discovery")
	}
}
