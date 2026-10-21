package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestRejectedAccessProjectionPreservesAuthorityAndCanonicalHistory(t *testing.T) {
	for _, tc := range []struct {
		name, code, effects, executing string
		mixed                          bool
		omitted                        bool
	}{
		{"known no effects", "computer_invalid_access_request", "none", "", false, true},
		{"unknown effects", "computer_invalid_access_request", "unknown", "", false, false},
		{"other failure", "computer_uncertain_outcome", "none", "", false, false},
		{"dispatch pending", "computer_invalid_access_request", "none", "pending", false, false},
		{"mixed response", "computer_invalid_access_request", "none", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true}, Checkpoint: &agents.Checkpoint{ProposalTool: computerAccessToolName, ExecutingCall: tc.executing}}
			failure, _ := json.Marshal(map[string]string{"error": tc.code, "effects": tc.effects})
			calls := []api.ToolCall{{ID: "rejected", Function: api.FunctionCall{Name: computerAccessToolName, Arguments: `{"target":"rejected-example"}`}}}
			if tc.mixed {
				calls = append(calls, api.ToolCall{ID: "other", Function: api.FunctionCall{Name: "other_tool", Arguments: `{}`}})
			}
			messages := []api.ChatMessage{
				{Role: "user", Content: "Use my requested application; do not change profiles."},
				{Role: "tool", Name: "write_file", ToolCallID: "earlier-dispatch", Content: "earlier effect must remain"},
				{Role: "assistant", ToolCalls: calls},
				{Role: "tool", Name: computerAccessToolName, ToolCallID: "rejected", Content: string(failure)},
				{Role: "user", Content: "Keep this correction"},
			}
			before, _ := json.Marshal(messages)
			got := (&browserRunTools{}).ProjectModelContext(task, messages)
			after, _ := json.Marshal(messages)
			if string(before) != string(after) {
				t.Fatal("canonical history changed")
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "rejected-example") == tc.omitted {
				t.Fatalf("wrong rejected-pair projection: %s", encoded)
			}
			for _, preserved := range []string{"do not change profiles", "earlier effect must remain", "Keep this correction"} {
				if !strings.Contains(string(encoded), preserved) {
					t.Fatalf("lost %s", preserved)
				}
			}
		})
	}
}

func TestSelectedWindowOffersObservationBeforeControlInput(t *testing.T) {
	s := taskFirstServer(t)
	tools := &browserRunTools{RunTools: agents.NewToolRegistry(), server: s}
	task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerSession: "fixture", ComputerDriver: "windows-uia"}}
	first := availableNames(tools.ToolsForTask(task))
	if !first["computer_observe"] || first["computer_replace_text"] || first["computer_shortcut"] || first[computerAccessToolName] {
		t.Fatal("pre-observation control menu", first)
	}
	if !first["calculator"] {
		t.Fatal("observation stage removed independent registry tools")
	}
	task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe"}}
	after := availableNames(tools.ToolsForTask(task))
	if !after["computer_replace_text"] || !after["computer_verify"] {
		t.Fatal("observation did not expose supported controls")
	}
	task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "action", ToolName: "computer_replace_text"})
	if availableNames(tools.ToolsForTask(task))["computer_replace_text"] {
		t.Fatal("dispatched input offered again before fresh observation")
	}
	task.Steps = task.Steps[:1]
	task.Config.ComputerStepOffset = 1
	if availableNames(tools.ToolsForTask(task))["computer_replace_text"] {
		t.Fatal("previous session observation reused")
	}
}
