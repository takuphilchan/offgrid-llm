package server

import (
	"reflect"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func actionEvidenceConfig() agents.AgentConfig {
	return agents.AgentConfig{TaskFirst: true, NativeWorkflowVersion: agents.DocumentWorkflowEvidenceVersion, NativeActionContractVersion: agents.NativeActionContractVersion, ComputerSession: "session", ComputerDriver: "windows-uia"}
}

func actionEvidenceApplicationFixture() *agents.Task {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	result := computer.ApplicationEvidence{Application: "edge", Check: "application_open", Verified: true, Target: &target, View: &computer.NativeView{Observation: computer.ControlObservation{ID: "ready", Target: target}}}
	return &agents.Task{
		Config: actionEvidenceConfig(),
		Workflow: &agents.WorkflowEvidence{Version: agents.DocumentWorkflowEvidenceVersion, Checks: []agents.OutcomeCheck{{
			Expectation: agents.OutcomeExpectation{ID: "opened", Title: "Application opened", Kind: "application_open", ApplicationID: "edge"}, ActionDerivedCallID: "open-source",
		}}},
		Steps: []agents.Step{
			{ID: 1, Type: "action", CallID: "open-source", ToolName: "computer_open_application", ToolArgs: `{"application_id":"edge"}`},
			{ID: 2, Type: "action", ToolName: "computer_verify_application", ToolArgs: `{"application_id":"edge"}`, ToolResult: documentJSON(result), Timestamp: time.Now().UTC()},
		},
	}
}

func TestActionBoundApplicationEvidenceSurvivesLaterNavigation(t *testing.T) {
	for _, mode := range []string{"derived", "legacy", "new-contract-explicit"} {
		t.Run(mode, func(t *testing.T) {
			task := actionEvidenceApplicationFixture()
			if mode != "derived" {
				task.Workflow.Checks[0].ActionDerivedCallID = ""
			}
			if mode == "legacy" {
				task.Config.NativeActionContractVersion = 0
			}
			tools := &browserRunTools{}
			before := tools.AssessWorkflow(task)[0]
			if before.State != "verified" {
				t.Fatalf("fixture did not independently verify: %+v", before)
			}
			task.Workflow.Checks[0] = before
			task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "action", ToolName: "computer_shortcut", ToolArgs: `{"observation_id":"ready","shortcut":"enter"}`})
			after := tools.AssessWorkflow(task)[0]
			if (after.State == "verified") != (mode == "derived") {
				t.Fatalf("unexpected historical semantics for %s: %+v", mode, after)
			}
			if mode == "derived" && (!reflect.DeepEqual(after.EvidenceSteps, before.EvidenceSteps) || !after.CheckedAt.Equal(*before.CheckedAt)) {
				t.Fatalf("navigation replaced the independent proof: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestActionBoundApplicationEvidenceRequiresMatchingSourceAndLaterProof(t *testing.T) {
	for _, fault := range []string{"missing-source", "rejected-source", "uncertain-source", "wrong-application", "wrong-tool", "invalid-step-id", "old-contract", "unknown-contract", "earlier-only-proof", "stored-state-only"} {
		t.Run(fault, func(t *testing.T) {
			task := actionEvidenceApplicationFixture()
			task.Workflow.Checks[0].State = "verified" // Stored claims are never evidence.
			task.Workflow.Checks[0].EvidenceSteps = []int{2}
			switch fault {
			case "missing-source":
				task.Workflow.Checks[0].ActionDerivedCallID = "not-dispatched"
			case "rejected-source":
				task.Steps[0].Type = "rejected"
			case "uncertain-source":
				task.Steps[0].Type = "tool_error"
			case "wrong-application":
				task.Steps[0].ToolArgs = `{"application_id":"notepad"}`
			case "wrong-tool":
				task.Steps[0].ToolName = "mcp_open_application"
			case "invalid-step-id":
				task.Steps[0].ID = 99
			case "old-contract":
				task.Config.NativeActionContractVersion = 0
			case "unknown-contract":
				task.Config.NativeActionContractVersion = 900
			case "earlier-only-proof":
				task.Steps[0], task.Steps[1] = task.Steps[1], task.Steps[0]
				task.Steps[0].ID, task.Steps[1].ID = 1, 2
			case "stored-state-only":
				task.Steps = task.Steps[:1]
			}
			got := (&browserRunTools{}).AssessWorkflow(task)[0]
			if got.State == "verified" || len(got.EvidenceSteps) != 0 || got.CheckedAt != nil {
				t.Fatalf("invalid source/proof established completion: %+v", got)
			}
		})
	}
}

func actionEvidenceControlFixture(kind string) (*agents.Task, computer.NativeView) {
	target := computer.TargetIdentity{ID: "window", OSSession: "os", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	text, checked := "tourism", true
	e := agents.OutcomeExpectation{ID: "edited", Title: "Control changed", Kind: kind, TargetID: target.ID, Element: "before-control"}
	writeTool, verifyTool, field, value := "computer_replace_text", "computer_verify", "text", any(text)
	if kind == "control_checked_equals" {
		writeTool, verifyTool, field, value, e.Checked = "computer_set_checked", "computer_verify_checked", "checked", checked, &checked
	} else {
		e.Text = &text
	}
	view := computer.NativeView{Observation: computer.ControlObservation{ID: "after", Target: target}, Elements: []computer.NativeElement{{ID: "after-control", Identity: "stable", Writable: true, Text: &text, Checkable: true, Checked: &checked}}}
	result := map[string]any{"verified": true, "check": kind, "observation_id": "after", "element": "after-control", field: value, "target": target}
	task := &agents.Task{
		Config: actionEvidenceConfig(),
		Workflow: &agents.WorkflowEvidence{Version: agents.DocumentWorkflowEvidenceVersion, Checks: []agents.OutcomeCheck{{
			Expectation: e, ActionDerivedCallID: "edit-source", Binding: &agents.OutcomeBinding{Session: "session", TargetSHA256: evidenceTargetDigest(target), Observation: "before", Identity: "stable", Step: 1},
		}}},
		Steps: []agents.Step{
			{ID: 1, Type: "task_metadata"},
			{ID: 2, Type: "action", CallID: "edit-source", ToolName: writeTool, ToolArgs: documentJSON(map[string]any{"observation_id": "before", "element": "before-control", field: value})},
			{ID: 3, Type: "action", ToolName: "computer_observe", ToolResult: documentJSON(view)},
			{ID: 4, Type: "action", ToolName: verifyTool, ToolArgs: documentJSON(map[string]any{"observation_id": "after", "element": "after-control", field: value}), ToolResult: documentJSON(result), Timestamp: time.Now().UTC()},
		},
	}
	return task, view
}

func TestActionBoundControlEvidenceSurvivesEnterAndChangedView(t *testing.T) {
	for _, kind := range []string{"control_text_equals", "control_checked_equals"} {
		for _, mode := range []string{"derived", "legacy", "new-contract-explicit"} {
			for _, transition := range []string{"enter", "changed-view"} {
				t.Run(kind+"/"+mode+"/"+transition, func(t *testing.T) {
					task, view := actionEvidenceControlFixture(kind)
					if mode != "derived" {
						task.Workflow.Checks[0].ActionDerivedCallID = ""
					}
					if mode == "legacy" {
						task.Config.NativeActionContractVersion = 0
					}
					tools := &browserRunTools{}
					before := tools.AssessWorkflow(task)[0]
					if before.State != "verified" {
						t.Fatalf("fixture did not independently verify: %+v", before)
					}
					task.Workflow.Checks[0] = before
					if transition == "enter" {
						task.Steps = append(task.Steps, agents.Step{ID: 5, Type: "action", ToolName: "computer_shortcut", ToolArgs: `{"observation_id":"after","shortcut":"enter"}`})
					} else {
						url, unchecked := "https://example.test/search?q=tourism", false
						view.Observation.ID = "navigated"
						view.Elements[0].Text, view.Elements[0].Checked = &url, &unchecked
						task.Steps = append(task.Steps, agents.Step{ID: 5, Type: "action", ToolName: "computer_observe", ToolResult: documentJSON(view)})
					}
					after := tools.AssessWorkflow(task)[0]
					if (after.State == "verified") != (mode == "derived") {
						t.Fatalf("unexpected historical semantics: %+v", after)
					}
					if mode == "derived" && !reflect.DeepEqual(after.EvidenceSteps, []int{3, 4}) {
						t.Fatalf("changed original independent evidence: %+v", after)
					}
				})
			}
		}
	}
}

func TestActionBoundControlEvidenceRejectsForgedBindingAndEarlierProof(t *testing.T) {
	for _, fault := range []string{"missing-source", "different-input", "different-element", "different-observation", "earlier-only-proof", "no-independent-read", "rejected-source"} {
		t.Run(fault, func(t *testing.T) {
			task, _ := actionEvidenceControlFixture("control_text_equals")
			task.Workflow.Checks[0].State = "verified"
			switch fault {
			case "missing-source":
				task.Workflow.Checks[0].ActionDerivedCallID = "missing"
			case "different-input":
				task.Steps[1].ToolArgs = `{"observation_id":"before","element":"before-control","text":"other"}`
			case "different-element":
				task.Steps[1].ToolArgs = `{"observation_id":"before","element":"other-control","text":"tourism"}`
			case "different-observation":
				task.Steps[1].ToolArgs = `{"observation_id":"stale","element":"before-control","text":"tourism"}`
			case "earlier-only-proof":
				source := task.Steps[1]
				task.Steps = append(task.Steps[:1], task.Steps[2:]...)
				task.Steps = append(task.Steps, source)
				for i := range task.Steps {
					task.Steps[i].ID = i + 1
				}
			case "no-independent-read":
				task.Steps = task.Steps[:2]
			case "rejected-source":
				task.Steps[1].Type = "rejected"
			}
			if got := (&browserRunTools{}).AssessWorkflow(task)[0]; got.State == "verified" || len(got.EvidenceSteps) != 0 {
				t.Fatalf("invalid control source/proof verified: %+v", got)
			}
		})
	}
}

func actionEvidenceDocumentFixture(kind string) *agents.Task {
	task, action, read := documentAssessmentFixture(kind)
	task.Config.TaskFirst = true
	task.Config.NativeActionContractVersion = agents.NativeActionContractVersion
	task.Workflow.Checks[0].ActionDerivedCallID = "document-source"
	task.Steps[1].CallID = "document-source"
	task.Steps[1].ToolResult, task.Steps[2].ToolResult = documentJSON(action), documentJSON(read)
	if kind == "document_blank" {
		task.Steps[1].ToolArgs = `{"observation_id":"before","command":"create"}`
	} else {
		task.Steps[1].ToolArgs = documentJSON(map[string]any{"observation_id": "before", "document": "doc", "snapshot": "before-snapshot", "text": *task.Workflow.Checks[0].Expectation.Text})
	}
	return task
}

func TestActionBoundDocumentEvidenceRequiresSourceSpecificReadback(t *testing.T) {
	for _, kind := range []string{"document_blank", "document_edit_matches"} {
		for _, fault := range []string{"none", "legacy", "missing-source", "earlier-only-proof", "no-independent-read"} {
			t.Run(kind+"/"+fault, func(t *testing.T) {
				task := actionEvidenceDocumentFixture(kind)
				switch fault {
				case "legacy":
					task.Config.NativeActionContractVersion = 0
					task.Workflow.Checks[0].ActionDerivedCallID = ""
				case "missing-source":
					task.Workflow.Checks[0].ActionDerivedCallID = "missing"
				case "earlier-only-proof":
					// A persisted binding to an earlier receipt cannot prove a later
					// source call, even when the same typed arguments were supplied.
					later := task.Steps[1]
					task.Steps[1].CallID = "earlier-call"
					later.ID = 4
					task.Steps = append(task.Steps, later)
				case "no-independent-read":
					task.Steps = task.Steps[:2]
				}
				task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: "action", ToolName: "computer_shortcut", ToolArgs: `{"shortcut":"enter"}`})
				got := (&browserRunTools{}).AssessWorkflow(task)[0]
				if (got.State == "verified") != (fault == "none") {
					t.Fatalf("incorrect action-specific document evidence: %+v", got)
				}
			})
		}
	}
}
