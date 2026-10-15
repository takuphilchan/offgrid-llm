package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func documentAssessmentFixture(kind string) (*agents.Task, computer.DocumentActionEvidence, computer.DocumentReadback) {
	target := computer.TargetIdentity{ID: "window", OSSession: "session", ProcessGeneration: "process", Surface: "surface", Driver: "windows-uia"}
	before := computer.DocumentBinding{Target: target, Document: "doc", Generation: "generation", Container: "tab", Editor: "shared-editor", Observation: "before", Revision: "revision-before"}
	after := before
	after.Observation = "mutation-read"
	after.Revision = "revision-after"
	input, expected := "café 日本語 😀", "prefix café 日本語 😀 suffix"
	action := computer.DocumentActionEvidence{Version: 1, ActionID: "action", Operation: "insert_text", Before: &before, BeforeSnapshot: "before-snapshot", After: after, AfterSnapshot: "mutation-snapshot", PreimageSHA256: computer.DocumentTextDigest("prefix  suffix"), InputSHA256: computer.DocumentTextDigest(input), ExpectedSHA256: computer.DocumentTextDigest(expected)}
	e := agents.OutcomeExpectation{ID: "edit", Title: "Document check", Kind: kind, TargetID: target.ID, Document: before.Document, Snapshot: "before-snapshot", EditMode: "insert_text", Text: &input}
	d := &agents.OutcomeDocumentBinding{ID: before.Document, Generation: before.Generation, Container: before.Container, Editor: before.Editor, Revision: before.Revision, Snapshot: "before-snapshot", ActionID: "action", Operation: "insert_text", PreimageSHA256: action.PreimageSHA256, ExpectedSHA256: action.ExpectedSHA256}
	tool := "computer_insert_text"
	if kind == "document_blank" {
		expected = ""
		e.Document, e.Snapshot, e.EditMode, e.Command, e.Text = "", "", "", "create", nil
		action.Operation, action.Command, action.Before, action.BeforeSnapshot, action.PreimageSHA256, action.InputSHA256, action.ExpectedSHA256 = "new_document", "create", nil, "", "", "", computer.DocumentTextDigest("")
		d.Revision, d.Snapshot, d.Operation, d.PreimageSHA256, d.ExpectedSHA256 = after.Revision, action.AfterSnapshot, action.Operation, "", action.ExpectedSHA256
		tool = "computer_create_document"
	} else if kind == "document_text_equals" {
		e.Text = &expected
		e.EditMode = ""
		d.ActionID, d.Operation, d.PreimageSHA256, d.ExpectedSHA256 = "", "", "", ""
	}
	read := computer.DocumentReadback{Version: 1, Snapshot: "independent-snapshot", Binding: after, Complete: true, Composition: "stable", TextSHA256: computer.DocumentTextDigest(expected), Bytes: len(expected)}
	read.Binding.Observation = "independent-observation"
	binding := &agents.OutcomeBinding{Session: "host-session", TargetSHA256: evidenceTargetDigest(target), Observation: before.Observation, Step: 1, Document: d}
	task := &agents.Task{Config: agents.AgentConfig{NativeWorkflowVersion: 2, ComputerSession: "host-session", ComputerDriver: target.Driver}, Workflow: &agents.WorkflowEvidence{Version: 2, Checks: []agents.OutcomeCheck{{Expectation: e, Binding: binding}}}, Steps: []agents.Step{{ID: 1, Type: "task_metadata"}, {ID: 2, Type: "action", ToolName: tool}, {ID: 3, Type: "action", ToolName: "computer_read_text", Timestamp: time.Now()}}}
	if kind == "document_text_equals" {
		task.Steps[1].Type = "task_metadata"
	}
	return task, action, read
}

func TestServiceArtifactCannotFulfillUnsavedMultilingualNotepad(t *testing.T) {
	const requested = "OffGrid test — café 日本語 12345"
	for _, tool := range []string{taskArtifactToolName, "write_file", "mcp_document_export"} {
		t.Run(tool, func(t *testing.T) {
			task, _, _ := documentAssessmentFixture("document_edit_matches")
			task.Prompt = "Open Notepad. Create a blank document and type exactly: " + requested + ". Read it back. Do not save or change existing documents."
			task.Workflow.Checks[0].Expectation.Text = func() *string { s := requested; return &s }()
			result, _ := json.Marshal(map[string]any{"verified": true, "content": requested, "sha256": computer.DocumentTextDigest(requested), "location": "service", "name": "notes.txt"})
			task.Steps = []agents.Step{{ID: 1, Type: "task_metadata"}, {ID: 2, Type: "action", ToolName: tool, ToolResult: string(result)}}
			checks := (&browserRunTools{}).AssessWorkflow(task)
			if len(checks) != 1 || checks[0].State == "verified" || len(checks[0].EvidenceSteps) != 0 {
				t.Fatal("service bytes verified host document", checks)
			}
		})
	}
}

func TestDocumentWorkflowUsesIndependentBoundReadback(t *testing.T) {
	encode := func(v any) string { data, _ := json.Marshal(v); return string(data) }
	for _, kind := range []string{"document_blank", "document_text_equals", "document_edit_matches"} {
		for _, fault := range []string{"none", "other-document", "other-generation", "other-container", "other-editor", "other-target", "other-session", "incomplete", "changed-text", "old-observation", "snapshot-echo", "failed-read", "untrusted-tool", "unknown-version", "later-input", "newer-different-tab", "wrong-action", "mutation-echo", "uncertain-input", "wrong-input", "wrong-preimage", "wrong-revision", "wrong-declaration", "broken-step-order"} {
			t.Run(kind+"/"+fault, func(t *testing.T) {
				task, action, read := documentAssessmentFixture(kind)
				switch fault {
				case "other-document":
					read.Binding.Document = "other"
				case "other-generation":
					read.Binding.Generation = "replacement"
				case "other-container":
					read.Binding.Container = "other-tab"
				case "other-editor":
					read.Binding.Editor = "replacement-editor"
				case "other-target":
					read.Binding.Target.Surface = "replacement-window"
				case "other-session":
					task.Workflow.Checks[0].Binding.Session = "foreign"
				case "incomplete":
					read.Complete = false
				case "changed-text":
					read.TextSHA256 = computer.DocumentTextDigest("different")
					read.Bytes = 9
				case "old-observation":
					read.Binding.Observation = "before"
				case "snapshot-echo":
					read.Snapshot = task.Workflow.Checks[0].Binding.Document.Snapshot
				case "failed-read":
					task.Steps[2].Type = "error"
				case "untrusted-tool":
					task.Steps[2].ToolName = "mcp_document_read"
				case "unknown-version":
					task.Workflow.Version = 900
				case "later-input":
					task.Steps = append(task.Steps, agents.Step{ID: 4, Type: "action", ToolName: "computer_shortcut"})
				case "broken-step-order":
					task.Steps = append(task.Steps, agents.Step{ID: 99, Type: "action", ToolName: "computer_read_text", ToolResult: encode(read)})
				case "newer-different-tab":
					other := read
					other.Binding.Document = "another-tab"
					task.Steps = append(task.Steps, agents.Step{ID: 4, Type: "action", ToolName: "computer_read_text", ToolResult: encode(other)})
				case "wrong-action":
					action.ActionID = "different-action"
				case "mutation-echo":
					read.Snapshot = action.AfterSnapshot
					read.Binding.Observation = action.After.Observation
				case "uncertain-input":
					task.Steps[1].Type = "uncertain"
				case "wrong-input":
					action.InputSHA256 = computer.DocumentTextDigest("wrong-input")
				case "wrong-preimage":
					action.PreimageSHA256 = computer.DocumentTextDigest("wrong-preimage")
				case "wrong-revision":
					read.Binding.Revision = "changed-revision"
				case "wrong-declaration":
					if kind == "document_blank" {
						task.Workflow.Checks[0].Expectation.Command = "other-command"
					} else {
						task.Workflow.Checks[0].Expectation.Document = "other-document"
					}
				}
				task.Steps[1].ToolResult = encode(action)
				task.Steps[2].ToolResult = encode(read)
				checks := (&browserRunTools{}).AssessWorkflow(task)
				want := fault == "none"
				// A pure exact-text predicate does not claim an edit, provenance of
				// a particular mutation, or unchanged surroundings.
				if kind == "document_text_equals" {
					switch fault {
					case "wrong-action", "mutation-echo", "uncertain-input", "wrong-input", "wrong-preimage", "wrong-revision", "newer-different-tab":
						want = true
					}
				}
				if (checks[0].State == "verified") != want {
					t.Fatalf("state=%s, wanted verified=%v", checks[0].State, want)
				}
			})
		}
	}
}

func TestExactDocumentCheckpointSurvivesOnlyIndependentDocumentWork(t *testing.T) {
	for _, fault := range []string{"none", "same-document", "foreign-target", "uncertain", "wrong-operation", "malformed"} {
		t.Run(fault, func(t *testing.T) {
			task, _, read := documentAssessmentFixture("document_text_equals")
			task.Steps[2].ToolResult = documentJSON(read)
			other := read.Binding
			other.Document, other.Container, other.Observation = "new-document", "new-tab", "created"
			action := computer.DocumentActionEvidence{Version: 1, ActionID: "create-other", Operation: "new_document", Command: "new", After: other, AfterSnapshot: "new-snapshot", ExpectedSHA256: computer.DocumentTextDigest("")}
			step := agents.Step{ID: 4, Type: "action", ToolName: "computer_create_document"}
			switch fault {
			case "same-document":
				action.After.Document = read.Binding.Document
			case "foreign-target":
				action.After.Target.Surface = "another-surface"
			case "uncertain":
				step.Type = "uncertain"
			case "wrong-operation":
				step.ToolName = "computer_insert_text"
			case "malformed":
				action.After.Generation = ""
			}
			step.ToolResult = documentJSON(action)
			task.Steps = append(task.Steps, step)
			checks := (&browserRunTools{}).AssessWorkflow(task)
			if (checks[0].State == "verified") != (fault == "none") {
				t.Fatalf("state=%s", checks[0].State)
			}
		})
	}
}

func TestDocumentCreationMilestoneSurvivesOnlyVerifiedEditChain(t *testing.T) {
	for _, fault := range []string{"none", "no-blank-read", "wrong-preimage", "wrong-before", "no-edit-read", "wrong-edit-text", "uncertain", "other-document"} {
		t.Run(fault, func(t *testing.T) {
			task, creation, blank := documentAssessmentFixture("document_blank")
			task.Steps[1].ToolResult = documentJSON(creation)
			task.Steps[2].ToolResult = documentJSON(blank)
			before := blank.Binding
			after := before
			after.Observation, after.Revision = "edit-observation", "edit-revision"
			text := "OffGrid test — café 日本語 12345"
			edit := computer.DocumentActionEvidence{Version: 1, ActionID: "edit", Operation: "insert_text", Before: &before, BeforeSnapshot: blank.Snapshot, After: after, AfterSnapshot: "edit-snapshot", PreimageSHA256: blank.TextSHA256, InputSHA256: computer.DocumentTextDigest(text), ExpectedSHA256: computer.DocumentTextDigest(text)}
			read := computer.DocumentReadback{Version: 1, Snapshot: "read-edited", Binding: after, Complete: true, Composition: "stable", Bytes: len(text), TextSHA256: edit.ExpectedSHA256}
			read.Binding.Observation = "read-edited-observation"
			task.Steps = append(task.Steps, agents.Step{ID: 4, Type: "action", ToolName: "computer_insert_text"}, agents.Step{ID: 5, Type: "action", ToolName: "computer_read_text", Timestamp: time.Now()})
			switch fault {
			case "no-blank-read":
				task.Steps[2].Type = "error"
			case "wrong-preimage":
				edit.PreimageSHA256 = computer.DocumentTextDigest("not blank")
			case "wrong-before":
				edit.Before.Revision = "replaced"
			case "no-edit-read":
				task.Steps[4].Type = "error"
			case "wrong-edit-text":
				read.TextSHA256 = computer.DocumentTextDigest("wrong")
			case "uncertain":
				task.Steps[3].Type = "uncertain"
			case "other-document":
				edit.Before.Document = "other"
			}
			task.Steps[3].ToolResult = documentJSON(edit)
			task.Steps[4].ToolResult = documentJSON(read)
			checks := (&browserRunTools{}).AssessWorkflow(task)
			if (checks[0].State == "verified") != (fault == "none") {
				t.Fatal(checks[0].State)
			}
			if fault == "none" && len(checks[0].EvidenceSteps) < 5 {
				t.Fatal("lost creation/blank/edit/read chain")
			}
		})
	}
}
