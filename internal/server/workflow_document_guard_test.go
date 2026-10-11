package server

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func documentGuardFixture(kind string) (*agents.Task, documentEditArguments, computer.DocumentInventory, computer.DocumentReadback) {
	now := time.Now().UTC()
	target := computer.TargetIdentity{ID: "target", Driver: "windows-uia", OSSession: "os", ProcessGeneration: "process", Surface: "window"}
	b := computer.DocumentBinding{Target: target, Document: "doc", Generation: "generation", Container: "tab", Editor: "editor", Observation: "inventory", Revision: "revision"}
	inv := computer.DocumentInventory{Version: 1, Complete: true, Active: &b, Observation: computer.ControlObservation{ID: b.Observation, Target: target, Sequence: 1, CapturedAt: now, StateDigest: strings.Repeat("a", 64)}, Documents: []computer.NativeDocument{{ID: b.Document, Generation: b.Generation, Container: b.Container, Selected: true, Capabilities: computer.DocumentCapabilities{Read: true, InsertText: true, ReplaceSelection: true, ReplaceValue: true}}}, Commands: []computer.DocumentCreationCommand{{ID: "new", Element: "button", Kind: "new_document", Profile: "fixture", Revision: "v1"}}}
	read := computer.DocumentReadback{Version: 1, Snapshot: "snapshot", Binding: b, Complete: true, Composition: "stable", TextSHA256: computer.DocumentTextDigest("prefix suffix"), Bytes: len("prefix suffix")}
	read.Binding.Observation = "read"
	text := "café 日本語 😀"
	e := agents.OutcomeExpectation{ID: "edit", Title: "Exact edit", Kind: "document_edit_matches", TargetID: target.ID, Document: b.Document, Snapshot: read.Snapshot, EditMode: kind, Text: &text}
	a := documentEditArguments{Observation: "read", Document: b.Document, Snapshot: read.Snapshot, Text: &text}
	task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, NativeWorkflowVersion: 2, ComputerSession: "session", ComputerDriver: target.Driver}, Workflow: &agents.WorkflowEvidence{Version: 2}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_documents", ToolResult: documentJSON(inv), Timestamp: now}}}
	if kind == "new_document" {
		e.Kind, e.Command, e.Document, e.Snapshot, e.EditMode, e.Text = "document_blank", "new", "", "", "", nil
		a = documentEditArguments{Observation: "inventory", Command: "new"}
	} else {
		task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "action", ToolName: "computer_read_text", ToolResult: documentJSON(read), Timestamp: now})
	}
	task.Workflow.Checks = []agents.OutcomeCheck{{Expectation: e}}
	return task, a, inv, read
}

func documentJSON(v any) string { data, _ := json.Marshal(v); return string(data) }

func TestDocumentArgumentsAreClosedBeforeBinding(t *testing.T) {
	for _, mode := range []string{"new_document", "insert_text", "replace_selection", "replace_value"} {
		task, args, _, _ := documentGuardFixture(mode)
		tools := &browserRunTools{}
		if err := tools.BindWorkflowExpectations(task); err != nil {
			t.Fatal(err)
		}
		raw := documentJSON(args)
		for _, corrupt := range []string{
			"null", "[]", raw + "{}", strings.Replace(raw, `"observation_id"`, `"Observation_id"`, 1),
			raw[:len(raw)-1] + `,"observation_id":"forged"}`,
			raw[:len(raw)-1] + `,"element":""}`,
			raw[:len(raw)-1] + `,"binding":null}`,
			strings.Replace(raw, `"observation_id":"`+args.Observation+`"`, `"observation_id":null`, 1),
		} {
			if err := tools.ValidateWorkflowEdit(task, documentOperationTool(mode), json.RawMessage(corrupt)); err == nil {
				t.Fatal("accepted ambiguous document arguments", mode, corrupt)
			}
		}
	}
}

func TestDocumentEditGuard(t *testing.T) {
	for _, mode := range []string{"new_document", "insert_text", "replace_selection", "replace_value"} {
		for _, fault := range []string{"none", "missing-check", "application-only", "wrong-text", "wrong-mode", "wrong-snapshot", "wrong-document", "wrong-session", "wrong-target", "stale-observation", "expired-session", "unknown-version", "later-input", "partial-input", "duplicate-action", "invalid-order", "untrusted-inventory", "echo"} {
			t.Run(mode+"/"+fault, func(t *testing.T) {
				task, args, _, _ := documentGuardFixture(mode)
				tools := &browserRunTools{}
				if err := tools.BindWorkflowExpectations(task); err != nil {
					t.Fatal(err)
				}
				c := &task.Workflow.Checks[0]
				want := fault == "none"
				switch fault {
				case "missing-check":
					task.Workflow.Checks = nil
				case "application-only":
					c.Expectation.Kind = "application_open"
				case "wrong-text":
					v := "wrong"
					args.Text = &v
				case "wrong-mode":
					c.Expectation.EditMode = "unrelated"
					c.Expectation.Kind = "document_text_equals"
				case "wrong-snapshot":
					args.Snapshot = "foreign"
				case "wrong-document":
					args.Document = "foreign"
				case "wrong-session":
					c.Binding.Session = "foreign"
				case "wrong-target":
					c.Binding.TargetSHA256 = strings.Repeat("f", 64)
				case "stale-observation":
					args.Observation = "old"
				case "expired-session":
					task.ComputerSessionExpired = true
				case "unknown-version":
					task.Config.NativeWorkflowVersion = 999
				case "later-input", "partial-input":
					typ := "action"
					if fault == "partial-input" {
						typ = "error"
					}
					task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: typ, ToolName: "computer_shortcut"})
				case "duplicate-action":
					if c.Binding.Document == nil {
						c.Binding.Document = &agents.OutcomeDocumentBinding{}
					}
					c.Binding.Document.ActionID = "already-dispatched"
				case "invalid-order":
					task.Steps[0].ID = 900
				case "untrusted-inventory":
					task.Steps[0].ToolName = "mcp_computer_documents"
				case "echo":
					task.Steps[0].ToolResult = documentJSON(args)
				}
				err := tools.ValidateWorkflowEdit(task, documentOperationTool(mode), json.RawMessage(documentJSON(args)))
				if (err == nil) != want {
					t.Fatalf("accepted=%v want=%v: %v", err == nil, want, err)
				}
				if fault == "missing-check" {
					if !errors.Is(err, errWorkflowControlExpectationRequired) {
						t.Fatal(err)
					}
					message := safeComputerAuthorization(err).Error()
					if !strings.Contains(message, "document_") {
						t.Fatal(message)
					}
				}
			})
		}
	}
}

func TestDocumentBindingFromRecordedActionAndIndependentRead(t *testing.T) {
	for _, mode := range []string{"new_document", "insert_text", "replace_selection", "replace_value"} {
		for _, fault := range []string{"none", "input-echo", "lost-ack", "wrong-tab", "wrong-input", "prior-input", "changed-command", "changed-snapshot"} {
			t.Run(mode+"/"+fault, func(t *testing.T) {
				task, args, inv, read := documentGuardFixture(mode)
				tools := &browserRunTools{}
				if err := tools.BindWorkflowExpectations(task); err != nil {
					t.Fatal(err)
				}
				before := read.Binding
				after := before
				after.Observation, after.Revision = "result-observation", "result-revision"
				input := ""
				if args.Text != nil {
					input = *args.Text
				}
				e := computer.DocumentActionEvidence{Version: 1, ActionID: "action", Operation: mode, Before: &before, BeforeSnapshot: read.Snapshot, After: after, AfterSnapshot: "result-snapshot", PreimageSHA256: read.TextSHA256, InputSHA256: computer.DocumentTextDigest(input), ExpectedSHA256: computer.DocumentTextDigest("prefix café 日本語 😀suffix")}
				if mode == "new_document" {
					e.Command, e.Before, e.BeforeSnapshot, e.PreimageSHA256, e.InputSHA256 = inv.Commands[0].ID, nil, "", "", ""
					e.After.Document, e.After.Container, e.After.Generation = "created", "created-tab", "created-generation"
					e.ExpectedSHA256 = computer.DocumentTextDigest("")
				}
				if e.Validate() != nil {
					t.Fatal("invalid fixture")
				}
				s := agents.Step{ID: len(task.Steps) + 1, Type: "action", ToolName: documentOperationTool(mode), ToolArgs: documentJSON(args), Timestamp: time.Now().UTC()}
				switch fault {
				case "lost-ack":
					s.Type = "error"
				case "wrong-tab":
					e.After.Document = "unrelated"
					if mode == "new_document" {
						e.After.Target.Surface = "foreign-window"
					}
				case "wrong-input":
					e.InputSHA256 = strings.Repeat("a", 64)
				case "prior-input":
					task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: "error", ToolName: "computer_shortcut"})
					s.ID++
				case "changed-command":
					args.Command = "other"
					s.ToolArgs = documentJSON(args)
				case "changed-snapshot":
					args.Snapshot = "other"
					s.ToolArgs = documentJSON(args)
				}
				s.ToolResult = documentJSON(e)
				if fault == "input-echo" {
					s.ToolResult = documentJSON(args)
				}
				task.Steps = append(task.Steps, s)
				// A real provider re-observes the active document after input.
				// This read must preserve the mutation's canonical identity.
				refreshed := inv
				active := e.After
				active.Observation = "refreshed-inventory"
				refreshed.Active = &active
				refreshed.Observation.ID = active.Observation
				refreshed.Observation.Target = active.Target
				refreshed.Documents = []computer.NativeDocument{{ID: active.Document, Generation: active.Generation, Container: active.Container, Selected: true, Capabilities: computer.DocumentCapabilities{Read: true}}}
				task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: "action", ToolName: "computer_documents", ToolResult: documentJSON(refreshed), Timestamp: time.Now().UTC()})
				read.Binding, read.Snapshot = e.After, "independent-snapshot"
				read.Binding.Observation, read.TextSHA256, read.Bytes = "independent-observation", e.ExpectedSHA256, 30
				if mode == "new_document" {
					read.Bytes = 0
				}
				task.Steps = append(task.Steps, agents.Step{ID: len(task.Steps) + 1, Type: "action", ToolName: "computer_read_text", ToolResult: documentJSON(read), Timestamp: time.Now().UTC()})
				checks := tools.AssessWorkflow(task)
				want := fault == "none"
				if (checks[0].State == "verified") != want {
					t.Fatalf("state=%s want verified=%t: %+v", checks[0].State, want, checks[0])
				}
				if task.Workflow.Checks[0].Binding.Document != nil && task.Workflow.Checks[0].Binding.Document.ActionID != "" {
					t.Fatal("assessment mutated original checkpoint")
				}
			})
		}
	}
}
