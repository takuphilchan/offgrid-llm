package server

import (
	"context"
	"encoding/json"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"strings"
	"testing"
)

func TestComputerCompletionRequiresUserCriterion(t *testing.T) {
	tools := &browserRunTools{}
	for _, test := range []struct {
		name, criterion, result string
		ok                      bool
	}{
		{"unrelated heading", "Draft saved: OffGrid test", `{"verified":true,"check":"page_contains_text","text":"Research notes"}`, false},
		{"legacy missing criterion", "", `{"verified":true}`, false},
		{"missing check type", "Saved", `{"verified":true,"text":"Saved"}`, false},
		{"negative check", "Saved", `{"verified":false,"check":"page_contains_text","text":"Saved"}`, false},
		{"matching check", "Saved", `{"verified":true,"check":"page_contains_text","text":"Saved"}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			args, _ := json.Marshal(map[string]string{"text": test.criterion})
			task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "session", ComputerExpectedText: test.criterion}, Steps: []agents.Step{{ToolName: "browser_verify", ToolArgs: string(args), ToolResult: test.result}}}
			if (tools.ValidateCompletion(task) == nil) != test.ok {
				t.Fatal("incorrect completion decision")
			}
			if test.ok {
				task.Steps = append(task.Steps, agents.Step{ToolName: "browser_click", ToolResult: `{}`})
				if tools.ValidateCompletion(task) == nil {
					t.Fatal("later mutation invalidates verification")
				}
			}
		})
	}
	config := agents.DefaultAgentConfig()
	config.ComputerExpectedText = "Quoted \"Unicode 日本語\""
	configureComputerTask(&config)
	encoded, _ := json.Marshal(config.ComputerExpectedText)
	if !strings.Contains(config.SystemPrompt, string(encoded)) {
		t.Fatal("invalid criterion")
	}
}

func TestComputerIntermediateVerificationReachesSessionBoundary(t *testing.T) {
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	runner := agents.NewRunner(manager, agents.NewToolRegistry(), nil)
	config := agents.DefaultAgentConfig()
	config.ComputerSession = "paired"
	config.ComputerExpectedText = "Draft saved"
	task, err := runner.Create("Save draft", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{server: &Server{agentManager: manager}}
	descriptor, _ := browserDescriptor("browser_verify")
	execution := agents.ToolExecution{RunID: task.ID, Actor: "alice", ExpectedCapability: &descriptor}
	err = tools.Authorize(context.Background(), "browser_verify", json.RawMessage(`{"text":"Research notes"}`), execution)
	if err != computer.ErrSession {
		t.Fatalf("intermediate check did not reach session boundary: %v", err)
	}
	err = tools.Authorize(context.Background(), "browser_verify", json.RawMessage(`{"text":"Draft saved"}`), execution)
	if err != computer.ErrSession {
		t.Fatalf("valid criterion did not reach session boundary: %v", err)
	}
}

func TestComputerAutomaticCompletionRequiresPageEvidence(t *testing.T) {
	for _, test := range []struct {
		name, result, argument string
		changed, ok            bool
	}{
		{"read-only result", "Research notes", "Research notes", false, true},
		{"changed result", "Draft saved: Zimbabwe research — final (2026)", "Draft saved: Zimbabwe research — final (2026)", true, true},
		{"unchanged heading after edit", "Research notes", "Research notes", true, false},
		{"mismatched evidence", "Draft saved", "Different text", true, false},
		{"empty evidence", "", "", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "paired", ComputerVerification: "page-evidence-v1"}, Steps: []agents.Step{{ToolName: "browser_observe", ToolResult: `{"text":"Research notes"}`}}}
			if test.changed {
				task.Steps = append(task.Steps, agents.Step{ToolName: "browser_fill", ToolResult: `{"changed":true}`}, agents.Step{ToolName: "browser_click", ToolResult: `{"dispatched":true}`})
			}
			result, _ := json.Marshal(map[string]any{"verified": true, "check": "page_contains_text", "text": test.result})
			args, _ := json.Marshal(map[string]string{"text": test.argument})
			task.Steps = append(task.Steps, agents.Step{ToolName: "browser_verify", ToolArgs: string(args), ToolResult: string(result)})
			tools := &browserRunTools{}
			if (tools.ValidateCompletion(task) == nil) != test.ok {
				t.Fatal("incorrect automatic completion decision")
			}
			if test.ok {
				task.Steps = append(task.Steps, agents.Step{ToolName: "browser_click"})
				if tools.ValidateCompletion(task) == nil {
					t.Fatal("later action invalidates check")
				}
			}
		})
	}
}
