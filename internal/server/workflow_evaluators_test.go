package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// Records the design boundary: providers mint fresh element IDs on observation.
// A label or matching text must not be used to pretend an old ID is the new one.
func TestWorkflowReadbackCannotGuessCrossObservationControlIdentity(t *testing.T) {
	for _, driver := range []string{"windows-uia", "macos-accessibility", "linux-atspi"} {
		t.Run(driver, func(t *testing.T) {
			target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: driver}
			text := "Expected text"
			view := computer.NativeView{Observation: computer.ControlObservation{ID: "after", Target: target}, Elements: []computer.NativeElement{{ID: "new-control-id", Identity: "stable-host-identity", Name: "Editor", Text: &text}}}
			observed, _ := json.Marshal(view)
			result, _ := json.Marshal(map[string]any{"verified": true, "check": "control_text_equals", "observation_id": "after", "element": "new-control-id", "text": text, "target": target})
			task := &agents.Task{Config: agents.AgentConfig{ComputerDriver: driver}, Workflow: &agents.WorkflowEvidence{Version: 1, Checks: []agents.OutcomeCheck{{Expectation: agents.OutcomeExpectation{ID: "text", Title: "Editor text", Kind: "control_text_equals", TargetID: target.ID, Element: "old-control-id", Text: &text}, State: "pending"}}}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(observed)}, {ID: 2, Type: "action", ToolName: "computer_verify", ToolArgs: `{"observation_id":"after","element":"new-control-id","text":"Expected text"}`, ToolResult: string(result), Timestamp: time.Now().UTC()}}}
			tools := &browserRunTools{}
			if checks := tools.AssessWorkflow(task); checks[0].State != "pending" {
				t.Fatal("guessed cross-observation identity", checks)
			}
			before := view
			before.Observation.ID = "before"
			before.Elements = []computer.NativeElement{{ID: "old-control-id", Identity: "stable-host-identity", Name: "Editor"}}
			oldJSON, _ := json.Marshal(before)
			steps := task.Steps
			task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: string(oldJSON)}}
			if err := tools.BindWorkflowExpectations(task); err != nil {
				t.Fatal(err)
			}
			task.Steps = append(task.Steps, steps...)
			for i := range task.Steps {
				task.Steps[i].ID = i + 1
			}
			checks := tools.AssessWorkflow(task)
			if checks[0].State != "verified" {
				t.Fatal("matching identity not checked", checks)
			}
			binding := task.Workflow.Checks[0].Binding
			binding.Identity = "replacement"
			if tools.AssessWorkflow(task)[0].State != "pending" {
				t.Fatal("replacement control proved old expectation")
			}
			binding.Identity = ""
			if tools.AssessWorkflow(task)[0].State != "unavailable" {
				t.Fatal("old host claimed identity")
			}
			binding.Identity = "stable-host-identity"
			binding.Session = "foreign"
			if tools.AssessWorkflow(task)[0].State != "pending" {
				t.Fatal("foreign session accepted")
			}
			binding.Session = ""
			task.Steps = append(task.Steps, agents.Step{ID: 4, Type: "task_metadata", ToolName: "task_plan"})
			if tools.AssessWorkflow(task)[0].State != "verified" {
				t.Fatal("metadata erased evidence")
			}
			task.Steps = append(task.Steps, agents.Step{ID: 5, Type: "action", ToolName: "computer_shortcut"})
			if tools.AssessWorkflow(task)[0].State != "pending" {
				t.Fatal("later input did not invalidate check")
			}
		})
	}
}

func TestWorkflowTypedEvidenceRejectsForeignFailedAndArbitraryResults(t *testing.T) {
	for _, kind := range []string{"control_text_equals", "control_checked_equals"} {
		for _, fault := range []string{"none", "failed", "foreign-target", "arbitrary-step", "duplicate-identity", "wrong-argument", "newer-change"} {
			t.Run(kind+"/"+fault, func(t *testing.T) {
				target := computer.TargetIdentity{ID: "target", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
				text, checked := "Expected", true
				e := agents.OutcomeExpectation{ID: "check", Title: "Read-back", Kind: kind, TargetID: target.ID, Element: "control-before"}
				tool := "computer_verify"
				if kind == "control_text_equals" {
					e.Text = &text
				} else {
					e.Checked = &checked
					tool = "computer_verify_checked"
				}
				view := computer.NativeView{Observation: computer.ControlObservation{ID: "before", Target: target}, Elements: []computer.NativeElement{{ID: "control-before", Identity: "stable", Text: &text, Checkable: true, Checked: &checked}}}
				marshal := func(v any) string { data, _ := json.Marshal(v); return string(data) }
				task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "session", ComputerDriver: target.Driver}, Workflow: &agents.WorkflowEvidence{Version: 1, Checks: []agents.OutcomeCheck{{Expectation: e}}}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: marshal(view)}}}
				tools := &browserRunTools{}
				if err := tools.BindWorkflowExpectations(task); err != nil {
					t.Fatal(err)
				}
				view.Observation.ID = "after"
				view.Elements[0].ID = "control-after"
				args := map[string]any{"observation_id": "after", "element": "control-after"}
				result := map[string]any{"verified": true, "check": kind, "target": target, "observation_id": "after", "element": "control-after"}
				if kind == "control_text_equals" {
					args["text"] = text
					result["text"] = text
				} else {
					args["checked"] = checked
					result["checked"] = checked
				}
				if fault == "failed" {
					result["verified"] = false
				}
				if fault == "foreign-target" {
					other := target
					other.ProcessGeneration = "replacement"
					result["target"] = other
				}
				if fault == "duplicate-identity" {
					duplicate := view.Elements[0]
					duplicate.ID = "another"
					view.Elements = append(view.Elements, duplicate)
				}
				if fault == "wrong-argument" {
					args["element"] = "unknown"
				}
				task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "action", ToolName: "computer_observe", ToolResult: marshal(view)}, agents.Step{ID: 3, Type: "action", ToolName: tool, ToolArgs: marshal(args), ToolResult: marshal(result)})
				if fault == "arbitrary-step" {
					task.Steps[2].ID = 99
				}
				if fault == "newer-change" {
					view.Elements[0].Identity = "new-control"
					task.Steps = append(task.Steps, agents.Step{ID: 4, Type: "action", ToolName: "computer_observe", ToolResult: marshal(view)})
				}
				state := tools.AssessWorkflow(task)[0].State
				if (state == "verified") != (fault == "none") {
					t.Fatal("incorrect assessment", state, fault)
				}
			})
		}
	}
}

func TestWorkflowApplicationReadinessCannotProveResearchOrSavedFile(t *testing.T) {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	raw, _ := json.Marshal(computer.ApplicationEvidence{Application: "app", Check: "application_open", Verified: true, Target: &target, View: &computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}}})
	task := &agents.Task{Config: agents.AgentConfig{ComputerDriver: target.Driver}, Workflow: &agents.WorkflowEvidence{Version: 1, Checks: []agents.OutcomeCheck{
		{Expectation: agents.OutcomeExpectation{Kind: "application_open", ApplicationID: "app"}},
		{Expectation: agents.OutcomeExpectation{Kind: "research"}},
		{Expectation: agents.OutcomeExpectation{Kind: "saved_file"}},
	}}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_verify_application", ToolArgs: `{"application_id":"app"}`, ToolResult: string(raw)}}}
	checks := (&browserRunTools{}).AssessWorkflow(task)
	if checks[0].State != "verified" || checks[1].State != "unavailable" || checks[2].State != "unavailable" {
		t.Fatal(checks)
	}
}

func TestWorkflowMultipleControlChecksRemainIndependent(t *testing.T) {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	text, checked := "Expected", true
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}, Elements: []computer.NativeElement{
		{ID: "first", Identity: "identity-first", Text: &text},
		{ID: "second", Identity: "identity-second", Text: &text},
		{ID: "checkbox", Identity: "identity-checkbox", Checkable: true, Checked: &checked},
	}}
	marshal := func(v any) string { data, _ := json.Marshal(v); return string(data) }
	task := &agents.Task{Config: agents.AgentConfig{ComputerSession: "session", ComputerDriver: target.Driver}, Workflow: &agents.WorkflowEvidence{Version: 1}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: marshal(view)}}}
	for _, element := range view.Elements {
		e := agents.OutcomeExpectation{ID: element.ID, Title: element.ID, Kind: "control_text_equals", TargetID: target.ID, Element: element.ID, Text: element.Text}
		if element.Checkable {
			e.Kind, e.Checked = "control_checked_equals", &checked
		}
		task.Workflow.Checks = append(task.Workflow.Checks, agents.OutcomeCheck{Expectation: e})
	}
	tools := &browserRunTools{}
	if err := tools.BindWorkflowExpectations(task); err != nil {
		t.Fatal(err)
	}
	for _, c := range task.Workflow.Checks {
		a := map[string]any{"observation_id": "view", "element": c.Expectation.Element}
		result := map[string]any{"verified": true, "check": c.Expectation.Kind, "observation_id": "view", "element": c.Expectation.Element, "target": target}
		tool := "computer_verify"
		if c.Expectation.Text != nil {
			a["text"], result["text"] = text, text
		} else {
			tool = "computer_verify_checked"
			a["checked"], result["checked"] = checked, checked
		}
		task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: "action", ToolName: tool, ToolArgs: marshal(a), ToolResult: marshal(result)})
	}
	for _, check := range tools.AssessWorkflow(task) {
		if check.State != "verified" {
			t.Fatalf("later unrelated verification erased %s: %s", check.Expectation.ID, check.State)
		}
	}
	// A failed re-check of the first control invalidates only that predicate.
	failed := task.Steps[1]
	failed.ID = len(task.Steps) + 1
	failed.ToolResult = `{"verified":false}`
	task.Steps = append(task.Steps, failed)
	for i, check := range tools.AssessWorkflow(task) {
		want := "verified"
		if i == 0 {
			want = "pending"
		}
		if check.State != want {
			t.Fatalf("failed re-check affected %s: %s, want %s", check.Expectation.ID, check.State, want)
		}
	}
}
