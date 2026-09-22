package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func TestComputerApprovalPoliciesUseCompanionClassification(t *testing.T) {
	tests := []struct {
		name          string
		mode          computer.ApprovalMode
		class         computer.ActionClass
		wantApproval  bool
		wantForbidden bool
	}{
		{"ask reviews reversible", computer.ApprovalAskEveryTime, computer.ActionReversible, true, false},
		{"scoped allows reversible", computer.ApprovalScopedChanges, computer.ActionReversible, false, false},
		{"scoped reviews consequential", computer.ApprovalScopedChanges, computer.ActionConsequential, true, false},
		{"full allows consequential", computer.ApprovalFullTask, computer.ActionConsequential, false, false},
		{"full cannot override forbidden", computer.ApprovalFullTask, computer.ActionForbidden, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hub := computer.NewBrowserHub()
			code, _ := hub.PairCode("alice")
			session, token, err := hub.Pair(code, "https://example.com", computer.ProtocolVersion, tc.mode)
			if err != nil {
				t.Fatal(err)
			}
			manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
			config := agents.DefaultAgentConfig()
			config.ComputerSession = session.ID
			config.ComputerDriver = "browser"
			config.ComputerApprovalMode = string(tc.mode)
			runner := agents.NewRunner(manager, agents.NewToolRegistry(), nil)
			task, err := runner.Create("Update form", "model", "alice", config)
			if err != nil {
				t.Fatal(err)
			}
			tools := &browserRunTools{server: &Server{browserHub: hub, agentManager: manager}}
			descriptor, _ := browserDescriptor("browser_fill")
			done := make(chan error, 1)
			go func() {
				deadline := time.Now().Add(time.Second)
				for time.Now().Before(deadline) {
					action, e := hub.Poll(token)
					if e != nil {
						done <- e
						return
					}
					if action != nil {
						if action.Kind != "computer_prepare" {
							done <- errors.New("mutation dispatched during authorization")
							return
						}
						payload, _ := json.Marshal(map[string]any{"prepared": true, "approval_class": tc.class})
						done <- hub.Reply(token, computer.BrowserReply{ID: action.ID, Result: string(payload)})
						return
					}
					time.Sleep(time.Millisecond)
				}
				done <- errors.New("prepare not delivered")
			}()
			err = tools.Authorize(context.Background(), "browser_fill", json.RawMessage(`{"observation_id":"seen","element":"1","text":"value"}`), agents.ToolExecution{RunID: task.ID, Actor: "alice", ExpectedCapability: &descriptor})
			if e := <-done; e != nil {
				t.Fatal(e)
			}
			if tc.wantApproval != errors.Is(err, capabilities.ErrApprovalRequired) {
				t.Fatalf("approval=%v err=%v", tc.wantApproval, err)
			}
			if tc.wantForbidden != errors.Is(err, computer.ErrProhibitedAction) {
				t.Fatalf("forbidden=%v err=%v", tc.wantForbidden, err)
			}
		})
	}
}

func TestBrowserTypedArguments(t *testing.T) {
	for _, tc := range []struct {
		name, args string
		valid      bool
	}{
		{"browser_observe", `{}`, true}, {"browser_observe", `null`, false}, {"browser_observe", `[]`, false},
		{"browser_observe", `{"script":"forbidden"}`, false},
		{"browser_select", `{"observation_id":"seen","element":"1","option":"2"}`, true},
		{"browser_select", `{"observation_id":"seen","element":"1","option":2}`, false},
		{"browser_set_checked", `{"observation_id":"seen","element":"1","checked":false}`, true},
		{"browser_set_checked", `{"observation_id":"seen","element":"1","checked":"false"}`, false},
		{"browser_fill", `{"observation_id":"seen","element":"1","text":""}`, true},
		{"browser_fill", `{"observation_id":"seen","element":"1","text":"` + strings.Repeat("語", 1334) + `"}`, false},
		{"browser_verify", `{"text":"` + strings.Repeat("語", 334) + `"}`, false},
		{"browser_click", `{"observation_id":"","element":"1"}`, false},
	} {
		if (validateBrowserArguments(tc.name, json.RawMessage(tc.args)) == nil) != tc.valid {
			t.Errorf("incorrect %s argument decision", tc.name)
		}
	}
}

func TestFormActionsRequireExactCapabilitiesAndApproval(t *testing.T) {
	hub := computer.NewBrowserHub()
	code, _ := hub.PairCode("alice")
	session, token, err := hub.Pair(code, "https://example.com", computer.ProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	manager := agents.NewManagerWithPersistence(nil, nil, nil, t.TempDir())
	runner := agents.NewRunner(manager, agents.NewToolRegistry(), nil)
	config := agents.DefaultAgentConfig()
	config.ComputerSession = session.ID
	task, err := runner.Create("Update form", "model", "alice", config)
	if err != nil {
		t.Fatal(err)
	}
	tools := &browserRunTools{server: &Server{browserHub: hub, agentManager: manager}}
	for name, args := range map[string]string{
		"browser_select":      `{"observation_id":"seen","element":"1","option":"2"}`,
		"browser_set_checked": `{"observation_id":"seen","element":"1","checked":false}`,
	} {
		descriptor, ok := browserDescriptor(name)
		if !ok || descriptor.Risk != capabilities.RiskHigh {
			t.Fatal("form mutation must be high risk")
		}
		e := agents.ToolExecution{RunID: task.ID, Actor: "alice", ExpectedCapability: &descriptor}
		if !errors.Is(tools.Authorize(context.Background(), name, json.RawMessage(args), e), capabilities.ErrApprovalRequired) {
			t.Fatal("mutation bypassed approval")
		}
		e.Approved = true
		if err := tools.Authorize(context.Background(), name, json.RawMessage(args), e); err != nil {
			t.Fatal(err)
		}
		e.Actor = "bob"
		if !errors.Is(tools.Authorize(context.Background(), name, json.RawMessage(args), e), agents.ErrTaskNotFound) {
			t.Fatal("cross-actor action allowed")
		}
		if action, _ := hub.Poll(token); action != nil {
			t.Fatal("authorization dispatched an action")
		}
	}
}

func TestFormMutationCannotCompleteUsingUnchangedHeading(t *testing.T) {
	for _, name := range []string{"browser_select", "browser_set_checked"} {
		task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "session", ComputerVerification: "page-evidence-v1"}, Steps: []agents.Step{
			{ToolName: "browser_observe", ToolResult: `{"text":"Preferences"}`},
			{ToolName: name, ToolResult: `{"changed":true,"verified":true}`},
			{ToolName: "browser_verify", ToolArgs: `{"text":"Preferences"}`, ToolResult: `{"verified":true,"check":"page_contains_text","text":"Preferences"}`},
		}}
		if (&browserRunTools{}).ValidateCompletion(task) == nil {
			t.Fatal("unchanged heading accepted as evidence of form changes")
		}
	}
}
