package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"strings"
	"testing"
	"time"
)

func TestDiscoveryUsesOnlyCurrentCanonicalEvidence(t *testing.T) {
	for _, driver := range []string{"windows-uia", "macos-accessibility", "linux-atspi"} {
		t.Run(driver, func(t *testing.T) {
			target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: driver}
			view := desktopContextFixture(target, "Fixture")
			for len(view.Elements) < 400 {
				view.Elements = append(view.Elements, computer.NativeElement{ID: fmt.Sprintf("control-%d", len(view.Elements)), Name: "Other", Role: "button"})
			}
			view.Observation.ID = "latest"
			view.Observation.CapturedAt = time.Now().Add(-time.Minute)
			view.Elements[399].Name = "Rare 旅行 control"
			data, _ := json.Marshal(view)
			task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "session", ComputerDriver: driver, TaskFirst: true, ComputerScope: "desktop"}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(data)}}}
			args := json.RawMessage(`{"observation_id":"latest","name":"旅行"}`)
			result, err := discoveryResult(task, "computer_find_controls", args, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("query=%d bytes result=%d bytes canonical=%d bytes", len(args), len(result), len(data))
			var page computer.ControlPage
			_ = json.Unmarshal([]byte(result), &page)
			if len(page.Controls) != 1 || page.Controls[0].ID != view.Elements[399].ID || page.InputFresh || page.AgeMS < 60000 || len(result) > 4096 {
				t.Fatalf("%s", result)
			}
			if task.Steps[0].ToolResult != string(data) {
				t.Fatal("changed canonical observation")
			}
			resolved := (&browserRunTools{}).ResolveModelArguments(task, "computer_inspect_control", `{"observation_id":"o1","element":"e1_399"}`)
			inspected, err := discoveryResult(task, "computer_inspect_control", json.RawMessage(resolved), time.Now())
			if err != nil || !strings.Contains(inspected, "Rare") {
				t.Fatal(inspected, err)
			}
			for _, bad := range []string{`{"observation_id":"latest","script":"x"}`, `{"observation_id":"latest","writable":"true"}`, `{"observation_id":"latest","writable":null}`, `{"observation_id":"latest","name":null}`, `{"observation_id":"old"}`} {
				if _, err = discoveryResult(task, "computer_find_controls", json.RawMessage(bad), time.Now()); err == nil {
					t.Fatal("accepted bad query", bad)
				}
			}
			task.ComputerSessionExpired = true
			if _, err = discoveryResult(task, "computer_find_controls", args, time.Now()); err == nil {
				t.Fatal("expired session")
			}
			task.ComputerSessionExpired = false
			task.Config.NativeWorkflowVersion = 900
			if _, err = discoveryResult(task, "computer_find_controls", args, time.Now()); err == nil {
				t.Fatal("unknown version")
			}
			task.Config.NativeWorkflowVersion = 0
			task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "action", ToolName: "computer_focus_window"})
			if _, err = discoveryResult(task, "computer_find_controls", args, time.Now()); err == nil {
				t.Fatal("changed target")
			}
		})
	}
}

func TestDiscoveryAuthorizationCannotContactHost(t *testing.T) {
	s, adapter, _, _, _ := compositionFixture(t)
	code, _ := s.browserHub.PairCode("alice")
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	host, token, err := s.browserHub.PairNative(code, "Fixture", target, 2, computer.ApprovalFullTask)
	if err != nil {
		t.Fatal(err)
	}
	cfg := compositionConfig("windows-uia", true)
	cfg.ComputerSession = host.ID
	view := desktopContextFixture(target, "Fixture")
	view.Observation.ID = "current"
	raw, _ := json.Marshal(view)
	runner := agents.NewRunner(s.agentManager, adapter, nil)
	task, err := runner.Create("inspect", "model", "alice", cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Persist observations through the manager's public step API, not host input.
	d, _ := adapter.Capability("computer_find_controls")
	e := agents.ToolExecution{Actor: "bob", RunID: task.ID, CallID: "read", ExpectedCapability: &d}
	if adapter.Authorize(context.Background(), "computer_find_controls", json.RawMessage(`{"observation_id":"current"}`), e) == nil {
		t.Fatal("foreign actor")
	}
	e.Actor = "alice"
	if adapter.Authorize(context.Background(), "computer_find_controls", json.RawMessage(`{"observation_id":"current"}`), e) == nil {
		t.Fatal("missing evidence")
	}
	if err = s.agentManager.AddTaskStep(task.ID, agents.Step{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(raw)}); err != nil {
		t.Fatal(err)
	}
	result, err := adapter.ExecuteWithPolicy(context.Background(), "computer_find_controls", json.RawMessage(`{"observation_id":"current","name":"Fixture"}`), e)
	if err != nil || result == "" {
		t.Fatal(result, err)
	}
	action, err := s.browserHub.Poll(token)
	if err != nil || action != nil {
		t.Fatal("discovery contacted host", action, err)
	}
	s.browserHub.StopSession("alice", host.ID)
	if adapter.Authorize(context.Background(), "computer_find_controls", json.RawMessage(`{"observation_id":"current"}`), e) == nil {
		t.Fatal("revoked access")
	}
}
