package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func actionContractFixture(mode string) (*agents.Task, documentEditArguments) {
	task, args, _, read := documentGuardFixture(mode)
	task.Config.NativeActionContractVersion, task.Config.ComputerScope = agents.NativeActionContractVersion, "desktop"
	task.Workflow.Checks = nil
	if mode != "new_document" {
		read.Page = &computer.DocumentTextPage{Snapshot: read.Snapshot, Binding: read.Binding, SourceComplete: true, Text: "prefix suffix", Start: 0, End: 13, TotalScalars: 13}
		read.Selection = &computer.DocumentTextRange{}
		if mode == "replace_selection" {
			read.Selection.End = 1
		}
		task.Steps[1].ToolResult = documentJSON(read)
	}
	return task, args
}

func actionProposal(name, args string) api.ToolCall {
	var call api.ToolCall
	call.ID, call.Type, call.Function.Name, call.Function.Arguments = "typed-action", "function", name, args
	return call
}

func TestNativeActionDerivesDocumentCheckWithoutMutatingTask(t *testing.T) {
	tools := &browserRunTools{}
	for _, mode := range []string{"new_document", "insert_text", "replace_selection", "replace_value"} {
		t.Run(mode, func(t *testing.T) {
			task, args := actionContractFixture(mode)
			before := documentJSON(task)
			e, err := tools.DeriveNativeActionExpectation(task, actionProposal(documentOperationTool(mode), documentJSON(args)))
			if err != nil || e == nil {
				t.Fatalf("check=%+v err=%v", e, err)
			}
			if before != documentJSON(task) {
				t.Fatal("derivation mutated canonical task")
			}
			if mode == "new_document" {
				if e.Kind != "document_blank" || e.Command != args.Command || e.Document != "" {
					t.Fatal(e)
				}
			} else if e.Kind != "document_edit_matches" || e.Document != args.Document || e.Snapshot != args.Snapshot || e.EditMode != mode || e.Text == nil || *e.Text != *args.Text {
				t.Fatal(e)
			}
			task.Workflow.Checks = []agents.OutcomeCheck{{Expectation: *e}}
			if err := tools.BindWorkflowExpectations(task); err != nil {
				t.Fatal(err)
			}
			if err := tools.ValidateWorkflowEdit(task, documentOperationTool(mode), json.RawMessage(documentJSON(args))); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNativeActionDerivationRejectsInvalidDocumentProposals(t *testing.T) {
	tools := &browserRunTools{}
	for _, fault := range []string{"stale", "null", "duplicate", "unknown-field", "wrong-case", "missing", "wrong-document", "expired", "later-input", "unsupported", "composition"} {
		t.Run(fault, func(t *testing.T) {
			task, args := actionContractFixture("insert_text")
			raw := documentJSON(args)
			switch fault {
			case "stale":
				raw = strings.ReplaceAll(raw, "snapshot", "stale")
			case "null":
				raw = strings.Replace(raw, `"text":`+documentJSON(*args.Text), `"text":null`, 1)
			case "duplicate":
				raw = raw[:len(raw)-1] + `,"text":"other"}`
			case "unknown-field":
				raw = raw[:len(raw)-1] + `,"element":"other"}`
			case "wrong-case":
				raw = strings.Replace(raw, "observation_id", "Observation_id", 1)
			case "missing":
				raw = `{}`
			case "wrong-document":
				raw = strings.Replace(raw, `"document":"doc"`, `"document":"other"`, 1)
			case "expired":
				task.ComputerSessionExpired = true
			case "later-input":
				task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "action", ToolName: "computer_shortcut"})
			case "unsupported":
				var inv computer.DocumentInventory
				_ = json.Unmarshal([]byte(task.Steps[0].ToolResult), &inv)
				inv.Documents[0].Capabilities.InsertText = false
				task.Steps[0].ToolResult = documentJSON(inv)
			case "composition":
				var read computer.DocumentReadback
				_ = json.Unmarshal([]byte(task.Steps[1].ToolResult), &read)
				read.Composition = "active"
				task.Steps[1].ToolResult = documentJSON(read)
			}
			if e, err := tools.DeriveNativeActionExpectation(task, actionProposal("computer_insert_text", raw)); err == nil || e != nil {
				t.Fatalf("accepted %s: %+v %v", fault, e, err)
			}
		})
	}
}

func TestNativeActionLaunchUsesCanonicalLookupAndResolvedAlias(t *testing.T) {
	task, _ := actionContractFixture("new_document")
	task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"installed-app","name":"Editor"}]}`}}
	appendApplicationLookup(t, task, "Editor", 1)
	tools := &browserRunTools{}
	alias := `{"application_id":"a1_0"}`
	if _, err := tools.DeriveNativeActionExpectation(task, actionProposal("computer_open_application", alias)); err == nil {
		t.Fatal("unresolved alias accepted")
	}
	raw := tools.ResolveModelArguments(task, "computer_open_application", alias)
	e, err := tools.DeriveNativeActionExpectation(task, actionProposal("computer_open_application", raw))
	if err != nil || e == nil || e.ApplicationID != "installed-app" || e.Kind != "application_open" {
		t.Fatalf("%+v %v", e, err)
	}
	for _, malformed := range []string{`{"application_id":null}`, `{"Application_id":"installed-app"}`, `{"application_id":"installed-app","application_id":"installed-app"}`, `{"application_id":"other"}`} {
		if _, err := tools.DeriveNativeActionExpectation(task, actionProposal("computer_open_application", malformed)); err == nil {
			t.Fatal("accepted", malformed)
		}
	}
}

func TestNativeActionControlChecksUseCurrentObservedIdentity(t *testing.T) {
	for _, kind := range []string{"text", "checked"} {
		t.Run(kind, func(t *testing.T) {
			task, _ := actionContractFixture("new_document")
			target := computer.TargetIdentity{ID: "window", Driver: "windows-uia", OSSession: "os", ProcessGeneration: "process", Surface: "surface"}
			view := computer.NativeView{Observation: computer.ControlObservation{ID: "view", Target: target}, Elements: []computer.NativeElement{{ID: "field", Identity: "stable-field", Writable: true, Checkable: true}}}
			task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: documentJSON(view)}}
			name, value := "computer_replace_text", any("tourism in Zimbabwe")
			if kind == "checked" {
				name, value = "computer_set_checked", false
			}
			args := map[string]any{"observation_id": "view", "element": "field", kind: value}
			tools := &browserRunTools{}
			e, err := tools.DeriveNativeActionExpectation(task, actionProposal(name, documentJSON(args)))
			if err != nil || e == nil || e.TargetID != target.ID || e.Element != "field" {
				t.Fatalf("%+v %v", e, err)
			}
			if kind == "text" && (e.Text == nil || *e.Text != value) {
				t.Fatal(e)
			}
			if kind == "checked" && (e.Checked == nil || *e.Checked) {
				t.Fatal(e)
			}
			for _, fault := range []string{"stale", "duplicate-identity", "wrong-target", "unsupported", "null", "wrong-case"} {
				t.Run(fault, func(t *testing.T) {
					copyTask, copyView := *task, view
					copyTask.Steps = append([]agents.Step(nil), task.Steps...)
					copyView.Elements = append([]computer.NativeElement(nil), view.Elements...)
					raw := documentJSON(args)
					switch fault {
					case "stale":
						raw = strings.Replace(raw, `"view"`, `"old-view"`, 1)
					case "duplicate-identity":
						copyView.Elements = append(copyView.Elements, computer.NativeElement{ID: "second", Identity: "stable-field"})
					case "wrong-target":
						copyView.Observation.Target.Driver = "other"
					case "unsupported":
						copyView.Elements[0].Writable, copyView.Elements[0].Checkable = false, false
					case "null":
						raw = strings.Replace(raw, `"`+kind+`":`+documentJSON(value), `"`+kind+`":null`, 1)
					case "wrong-case":
						raw = strings.Replace(raw, "observation_id", "Observation_id", 1)
					}
					copyTask.Steps[0].ToolResult = documentJSON(copyView)
					if _, err := tools.DeriveNativeActionExpectation(&copyTask, actionProposal(name, raw)); err == nil {
						t.Fatal("accepted", fault)
					}
				})
			}
		})
	}
}

func TestActionCoupledProjectionPreservesLegacyAndPendingReadback(t *testing.T) {
	task, args := actionContractFixture("insert_text")
	tools := append(stagedDocumentTools(task), api.Tool{Function: api.FunctionDef{Name: "task_expectations"}}, api.Tool{Function: api.FunctionDef{Name: "mcp__docs__read"}})
	projector := &browserRunTools{}
	names := availableNames(projector.ProjectModelTools(task, tools))
	if !names["computer_insert_text"] || !names["computer_create_document"] || !names["mcp__docs__read"] {
		t.Fatal(names)
	}
	for _, tool := range projector.ProjectModelTools(task, tools) {
		if tool.Function.Name != "task_expectations" {
			continue
		}
		raw := documentJSON(tool)
		if strings.Contains(raw, `"const":"document_edit_matches"`) || strings.Contains(raw, `"const":"document_blank"`) || !strings.Contains(raw, `"const":"document_text_equals"`) {
			t.Fatal("mechanical declarations remain", raw)
		}
	}
	task.Config.NativeActionContractVersion = 0
	legacy := availableNames(projector.ProjectModelTools(task, tools))
	if legacy["computer_insert_text"] || legacy["computer_create_document"] || !legacy["task_expectations"] {
		t.Fatal("legacy declaration contract changed", legacy)
	}
	task.Config.NativeActionContractVersion = agents.NativeActionContractVersion
	e, err := projector.DeriveNativeActionExpectation(task, actionProposal("computer_insert_text", documentJSON(args)))
	if err != nil {
		t.Fatal(err)
	}
	task.Workflow.Checks = []agents.OutcomeCheck{{Expectation: *e}}
	if err := projector.BindWorkflowExpectations(task); err != nil {
		t.Fatal(err)
	}
	var before computer.DocumentReadback
	_ = json.Unmarshal([]byte(task.Steps[1].ToolResult), &before)
	after := before.Binding
	after.Revision, after.Observation = "after", "after-view"
	evidence := computer.DocumentActionEvidence{Version: 1, ActionID: "dispatch", Operation: "insert_text", Before: &before.Binding, BeforeSnapshot: before.Snapshot, After: after, AfterSnapshot: "after-snapshot", PreimageSHA256: before.TextSHA256, InputSHA256: computer.DocumentTextDigest(*args.Text), ExpectedSHA256: computer.DocumentTextDigest(*args.Text)}
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
	task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "action", ToolName: "computer_insert_text", ToolArgs: documentJSON(args), ToolResult: documentJSON(evidence)})
	if _, err := projector.DeriveNativeActionExpectation(task, actionProposal("computer_insert_text", documentJSON(args))); err == nil {
		t.Fatal("pending input allowed another edit")
	}
	// Refreshed inventory is not read-back and cannot authorize a new creation.
	var inv computer.DocumentInventory
	_ = json.Unmarshal([]byte(task.Steps[0].ToolResult), &inv)
	inv.Active, inv.Observation.ID = &after, after.Observation
	task.Steps = append(task.Steps, agents.Step{ID: 4, Type: "action", ToolName: "computer_documents", ToolResult: documentJSON(inv)})
	if _, err := projector.DeriveNativeActionExpectation(task, actionProposal("computer_create_document", documentJSON(documentEditArguments{Observation: after.Observation, Command: "new"}))); err == nil {
		t.Fatal("pending readback allowed creation")
	}
	names = availableNames(projector.ProjectModelTools(task, tools))
	if names["computer_insert_text"] || names["computer_create_document"] || !names["computer_documents"] || !names["mcp__docs__read"] {
		t.Fatal("readback boundary lost", names)
	}
}

func TestActionContractPromptDoesNotRequireMetadataRoundTrip(t *testing.T) {
	task, _ := actionContractFixture("insert_text")
	prompt := desktopTaskPrompt(task.Config)
	if strings.Contains(prompt, "task_expectations(") || strings.Contains(prompt, "declare document_blank") || strings.Contains(prompt, "declare document_edit_matches") {
		t.Fatal("legacy declaration instructions", prompt)
	}
	if !strings.Contains(prompt, actionDocumentTaskProtocol) {
		t.Fatal("action document contract missing")
	}
	guidance := actionCoupledStageGuidance(task, nativeModelReferences(task))
	if strings.Contains(guidance, "first declare") || !strings.Contains(guidance, "perform the supported typed edit") {
		t.Fatal(guidance)
	}
	target := computer.TargetIdentity{ID: "window", Driver: "windows-uia", OSSession: "os", ProcessGeneration: "process", Surface: "surface"}
	task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_observe", ToolResult: documentJSON(computer.NativeView{Observation: computer.ControlObservation{ID: "before", Target: target}})}, {ID: 2, Type: "action", ToolName: "computer_documents", ToolResult: unavailableDocumentCapabilityResult}}
	guidance = actionCoupledStageGuidance(task, nil)
	if !strings.Contains(guidance, "Do not retry document discovery") || !strings.Contains(guidance, "fresh computer_observe") {
		t.Fatal(guidance)
	}
}

func TestActionControlReadbackBlocksEnterAndOtherMutations(t *testing.T) {
	for _, kind := range []string{"control_text_equals", "control_checked_equals"} {
		t.Run(kind, func(t *testing.T) {
			task, _ := actionEvidenceControlFixture(kind)
			task.Config.ComputerScope = "desktop"
			verifiedStep := task.Steps[len(task.Steps)-1]
			task.Steps = task.Steps[:len(task.Steps)-1]
			task.Workflow.Checks[0].State = "verified" // A stored claim cannot bypass actual verification.
			tools := &browserRunTools{}
			for _, name := range []string{"computer_shortcut", "computer_activate", "computer_replace_text", "computer_set_checked", "computer_open_application", "computer_focus_window", "computer_create_document", "computer_select_document", "computer_select_text"} {
				if err := tools.ValidateWorkflowEdit(task, name, json.RawMessage(`{}`)); err == nil {
					t.Fatal("input before independent verification", name)
				}
				if _, err := tools.DeriveNativeActionExpectation(task, actionProposal(name, `{}`)); err == nil {
					t.Fatal("proposal bypassed verification", name)
				}
			}
			if err := tools.ValidateWorkflowEdit(task, "computer_observe", json.RawMessage(`{}`)); err != nil {
				t.Fatal("fresh observation blocked", err)
			}
			menu := []api.Tool{{Function: api.FunctionDef{Name: "computer_shortcut"}}, {Function: api.FunctionDef{Name: "computer_observe"}}, {Function: api.FunctionDef{Name: verifiedStep.ToolName}}, {Function: api.FunctionDef{Name: "mcp__research__read"}}}
			names := availableNames(tools.ProjectModelTools(task, menu))
			if names["computer_shortcut"] || !names["computer_observe"] || !names[verifiedStep.ToolName] || !names["mcp__research__read"] {
				t.Fatal(names)
			}
			task.Steps = append(task.Steps, verifiedStep)
			if err := tools.ValidateWorkflowEdit(task, "computer_shortcut", json.RawMessage(`{}`)); err != nil {
				t.Fatal("verified action still blocks next transition", err)
			}
		})
	}
}
