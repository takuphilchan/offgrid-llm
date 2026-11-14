package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestTaskResourceUsagePreservesUnknownAndDoesNotExposeCheckpoint(t *testing.T) {
	task := &agents.Task{Config: agents.AgentConfig{MaxIterations: 30}, Checkpoint: &agents.Checkpoint{
		BudgetedModelCalls: 2, ReservedChildIterations: 4, ChargedCompletionTokens: 120, MeasuredCompletionTokens: 20,
		InputBudget:         &agents.InputBudget{Window: 4096, ChargedTokens: 4196, MeasuredTokens: 100, UnmeasuredCalls: 1},
		ToolBudget:          &agents.ToolBudget{Version: 1, CallsPerDecision: 16, PerCallNS: 100, Attempts: 2, ChargedNS: 125, MeasuredNS: 25, UnsettledCalls: 1, Pending: &agents.ToolReservation{Sequence: 2, CallID: "private-call-id"}},
		ProcessBudget:       &agents.ProcessBudget{Version: 1, PerDecisionNS: 1000, StartsPerDecision: 16, ReservedStarts: 2, ChargedNS: 110, MeasuredNS: 10, UnsettledStarts: 1, Reservations: map[string]agents.ProcessReservation{"private-process-id": {RunID: "private-run-id"}}},
		ContextPreparations: 2, MeasuredContextPreparationNS: 42,
		Messages: []api.ChatMessage{{Role: "system", Content: "private-checkpoint-protocol"}},
	}}
	usage := taskResponse(task)["resource_usage"].(*api.TaskResourceUsage)
	if usage.Input.ChargedTokens != 4196 || usage.Input.UnmeasuredCalls != 1 || usage.ModelCalls != 2 || usage.ChildCallAllocations != 4 || usage.MeasuredContextPreparationNS != 42 {
		t.Fatal(usage)
	}
	encoded, err := json.Marshal(usage)
	if err != nil || strings.Contains(string(encoded), "private-") {
		t.Fatal("checkpoint exposed", err)
	}
	if usage.Processes == nil || usage.Processes.ReservedStarts != 2 || usage.Processes.ChargedNS != 110 || usage.Processes.MeasuredNS != 10 || usage.Processes.UnsettledStarts != 1 {
		t.Fatal("process usage lost", usage.Processes)
	}
	task.Checkpoint.ProcessBudget = nil
	if taskResourceUsage(task).Processes != nil {
		t.Fatal("historical process usage invented")
	}
	if usage.Tools == nil || usage.Tools.ReservedCalls != 2 || usage.Tools.ChargedNS != 125 || usage.Tools.MeasuredNS != 25 || usage.Tools.UnsettledCalls != 1 {
		t.Fatal("tool usage projection lost accounting", usage.Tools)
	}
	task.Checkpoint.ToolBudget = nil
	if taskResourceUsage(task).Tools != nil {
		t.Fatal("historical tool usage invented")
	}
	task.Checkpoint.InputBudget = nil
	if taskResourceUsage(task).Input != nil {
		t.Fatal("historical usage invented")
	}
	task.Checkpoint = nil
	if _, exists := taskResponse(task)["resource_usage"]; exists {
		t.Fatal("missing checkpoint presented as zero usage")
	}
}
