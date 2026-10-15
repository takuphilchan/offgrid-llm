package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestExpectationSchemasRequireObservedStageAndPreserveOtherTools(t *testing.T) {
	tools := []api.Tool{{Function: api.FunctionDef{Name: "task_expectations", Parameters: map[string]any{"original": true}}}, {Function: api.FunctionDef{Name: "mcp__docs__read"}}}
	projection := &browserRunTools{}
	task, _, inv, read := documentGuardFixture("insert_text")
	task.Config.ComputerScope = "desktop"
	read.Selection = &computer.DocumentTextRange{Start: 0, End: 0}
	read.Page = &computer.DocumentTextPage{Snapshot: read.Snapshot, Binding: read.Binding, SourceComplete: true, Text: "prefix suffix", End: 13, TotalScalars: 13}
	all := append([]agents.Step(nil), task.Steps...)
	task.Steps = nil
	if got := projection.ProjectModelTools(task, tools); len(got) != 1 || got[0].Function.Name != "mcp__docs__read" {
		t.Fatal("unobserved checks or lost MCP", got)
	}
	task.Steps = []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"installed-app","name":"Editor"}]}`}}
	appendApplicationLookup(t, task, "Editor", 1)
	check := func(want ...string) {
		t.Helper()
		got := projection.ProjectModelTools(task, tools)
		if len(got) != 2 {
			t.Fatal(got)
		}
		encoded, _ := json.Marshal(got[0])
		for _, kind := range []string{"application_open", "document_blank", "document_edit_matches", "document_text_equals"} {
			present := false
			for _, w := range want {
				present = present || w == kind
			}
			if strings.Contains(string(encoded), `"const":"`+kind+`"`) != present {
				t.Fatal("wrong stage", kind, string(encoded))
			}
		}
	}
	check("application_open")
	if encoded, _ := json.Marshal(projection.ProjectModelTools(task, tools)); !strings.Contains(string(encoded), "a1_0") || strings.Contains(string(encoded), "installed-app") {
		t.Fatal("canonical application reference leaked into model schema", string(encoded))
	}
	all[0].ID = 3
	all[1].ID = 4
	all[1].ToolResult = documentJSON(read)
	task.Steps = append(task.Steps, all[0])
	if state, err := documentState(task); err != nil || state.step == 0 {
		t.Fatal("invalid fixture stage", err, task.Steps)
	}
	check("document_blank")
	task.Steps = append(task.Steps, all[1])
	check("document_blank", "document_edit_matches", "document_text_equals")
	encoded, _ := json.Marshal(projection.ProjectModelTools(task, tools))
	if !strings.Contains(string(encoded), `"const":"`+inv.Observation.Target.ID+`"`) || !strings.Contains(string(encoded), `"const":"`+read.Snapshot+`"`) {
		t.Fatal("unbound check references", string(encoded))
	}
	task.Steps = append(task.Steps, agents.Step{ID: 5, Type: "action", ToolName: "computer_insert_text"})
	if got := projection.ProjectModelTools(task, tools); len(got) != 1 || got[0].Function.Name != "mcp__docs__read" {
		t.Fatal("stale checks remained", got)
	}
	if tools[0].Function.Parameters["original"] != true {
		t.Fatal("projection mutated canonical schema")
	}
}

func TestProjectedOutcomeIDsCannotReplaceSavedChecks(t *testing.T) {
	task := &agents.Task{Config: agents.AgentConfig{TaskFirst: true, ComputerScope: "desktop", ComputerSession: "session", ComputerDriver: "windows-uia", NativeWorkflowVersion: agents.DocumentWorkflowEvidenceVersion}, Workflow: &agents.WorkflowEvidence{Checks: []agents.OutcomeCheck{{Expectation: agents.OutcomeExpectation{ID: "outcome-1"}}}}, Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_applications", ToolResult: `{"applications":[{"id":"editor","name":"Editor"}]}`}}}
	appendApplicationLookup(t, task, "Editor", 1)
	tool := api.Tool{Function: api.FunctionDef{Name: "task_expectations"}}
	got := (&browserRunTools{}).ProjectModelTools(task, []api.Tool{tool})
	if len(got) != 1 {
		t.Fatal("next check missing")
	}
	encoded, _ := json.Marshal(got[0])
	if !strings.Contains(string(encoded), `"const":"outcome-2"`) || !strings.Contains(string(encoded), `"maxItems":1`) {
		t.Fatal("saved IDs can be reused", string(encoded))
	}
}

func TestCompletedDocumentStageRequiresPlanAdvanceForMoreInput(t *testing.T) {
	task, _, _, read := documentGuardFixture("insert_text")
	task.Config.ComputerScope = "desktop"
	read.Selection = &computer.DocumentTextRange{}
	read.Page = &computer.DocumentTextPage{Snapshot: read.Snapshot, Binding: read.Binding, SourceComplete: true, Text: "prefix suffix", End: 13, TotalScalars: 13}
	task.Steps[1].ToolResult = documentJSON(read)
	projector := &browserRunTools{}
	if err := projector.BindWorkflowExpectations(task); err != nil {
		t.Fatal(err)
	}
	edit := &task.Workflow.Checks[0]
	edit.Expectation.Snapshot = "before-edit"
	edit.State, edit.EvidenceSteps = "verified", []int{1, 2}
	edit.Binding.Document.ExpectedSHA256 = read.TextSHA256
	blankBinding := *edit.Binding
	task.Workflow.Checks = append(task.Workflow.Checks, agents.OutcomeCheck{Expectation: agents.OutcomeExpectation{ID: "blank", Kind: "document_blank", TargetID: read.Binding.Target.ID, Command: "new"}, Binding: &blankBinding, State: "verified", EvidenceSteps: []int{1, 2}})
	tools := []api.Tool{{Function: api.FunctionDef{Name: "task_expectations"}}, {Function: api.FunctionDef{Name: "mcp__docs__read"}}}
	if got := projector.ProjectModelTools(task, tools); len(got) != 1 || got[0].Function.Name != "mcp__docs__read" {
		t.Fatal("completed stage invited duplicate input", got)
	}
	task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "task_metadata", ToolName: "task_plan"})
	encoded, _ := json.Marshal(projector.ProjectModelTools(task, tools))
	if !strings.Contains(string(encoded), `"const":"document_blank"`) || !strings.Contains(string(encoded), `"const":"document_edit_matches"`) {
		t.Fatal("plan advance prevented multi-document work", string(encoded))
	}
}

func TestDocumentMutationProjectionRequiresCurrentBoundCheck(t *testing.T) {
	for _, mode := range []string{"new_document", "insert_text", "replace_selection", "replace_value"} {
		t.Run(mode, func(t *testing.T) {
			task, _, _, _ := documentGuardFixture(mode)
			state, err := documentState(task)
			if err != nil {
				t.Fatal(err)
			}
			var tool api.Tool
			for _, candidate := range nativeDocumentTools() {
				if candidate.Function.Name == documentOperationTool(mode) {
					tool = candidate
				}
			}
			if _, ok := projectCheckedDocumentMutation(task, tool, state); ok {
				t.Fatal("unbound check offered input")
			}
			if err := (&browserRunTools{}).BindWorkflowExpectations(task); err != nil {
				t.Fatal(err)
			}
			got, ok := projectCheckedDocumentMutation(task, tool, state)
			if !ok {
				t.Fatal("bound current check omitted input")
			}
			encoded, _ := json.Marshal(got)
			if !strings.Contains(string(encoded), `"const"`) {
				t.Fatal("unbound schema")
			}
			task.Workflow.Checks[0].Binding.Session = "foreign"
			if _, ok := projectCheckedDocumentMutation(task, tool, state); ok {
				t.Fatal("foreign check offered input")
			}
		})
	}
}

func TestDocumentCheckProjectionDoesNotInviteDuplicateDeclarations(t *testing.T) {
	task, _, _, read := documentGuardFixture("insert_text")
	task.Config.ComputerScope = "desktop"
	projection := &browserRunTools{}
	tools := []api.Tool{{Function: api.FunctionDef{Name: "task_expectations"}}}
	task.Workflow.Checks = append(task.Workflow.Checks, agents.OutcomeCheck{Expectation: agents.OutcomeExpectation{Kind: "document_text_equals", Document: read.Binding.Document, Snapshot: read.Snapshot}})
	b, _ := json.Marshal(projection.ProjectModelTools(task, tools))
	if strings.Contains(string(b), `"const":"document_text_equals"`) {
		t.Fatal("repeat content declaration offered", string(b))
	}
	// A fresh snapshot permits a new check; this is planning relevance only.
	task.Workflow.Checks[len(task.Workflow.Checks)-1].Expectation.Snapshot = "older-snapshot"
	b, _ = json.Marshal(projection.ProjectModelTools(task, tools))
	if !strings.Contains(string(b), `"const":"document_text_equals"`) {
		t.Fatal("new snapshot check removed")
	}
}

func TestModelHistoryOnlyOffersOmittedUnreadPages(t *testing.T) {
	tool := api.Tool{Function: api.FunctionDef{Name: "task_history", Parameters: map[string]any{"original": true}}}
	task := &agents.Task{Steps: []agents.Step{{ID: 1, Type: "action", ToolName: "computer_verify_application", ToolResult: `{"view":{"elements":[{"id":"one"}]}}`}}}
	if _, ok := projectNeededHistory(task, tool, false); ok {
		t.Fatal("complete evidence invited redundant history")
	}
	task.ContextArchive = []agents.ArchivedContext{{ID: "archive-digest"}}
	got, ok := projectNeededHistory(task, tool, false)
	encoded, _ := json.Marshal(got)
	if !ok || !strings.Contains(string(encoded), "archive:1") || !strings.Contains(string(encoded), `"const":0`) {
		t.Fatal("missing exact first page", string(encoded))
	}
	task.Steps = append(task.Steps, agents.Step{ID: 2, Type: "task_metadata", ToolName: "task_history", ToolArgs: `{"reference":"archive:1","offset":0}`, ToolResult: `{"reference":"archive:1","next_offset":4096,"complete":false}`})
	got, ok = projectNeededHistory(task, tool, false)
	encoded, _ = json.Marshal(got)
	if !ok || !strings.Contains(string(encoded), `"const":4096`) || strings.Contains(string(encoded), `"const":0`) {
		t.Fatal("already read page reoffered", string(encoded))
	}
	task.Steps = append(task.Steps, agents.Step{ID: 3, Type: "task_metadata", ToolName: "task_history", ToolArgs: `{"reference":"archive:1","offset":4096}`, ToolResult: `{"reference":"archive:1","next_offset":5000,"complete":true}`})
	if _, ok := projectNeededHistory(task, tool, false); ok {
		t.Fatal("completed archive kept history loop available")
	}
	if tool.Function.Parameters["original"] != true || len(task.ContextArchive) != 1 {
		t.Fatal("canonical history/schema changed")
	}
}

func TestDocumentPlanningCannotSubstituteLegacyVerification(t *testing.T) {
	task, _, _, _ := documentGuardFixture("insert_text")
	task.Config.ComputerScope = "desktop"
	tools := []api.Tool{{Function: api.FunctionDef{Name: "computer_verify"}}, {Function: api.FunctionDef{Name: "computer_verify_checked"}}, {Function: api.FunctionDef{Name: "computer_read_text"}}}
	projector := &browserRunTools{}
	got := availableNames(projector.ProjectModelTools(task, tools))
	if got["computer_verify"] || got["computer_verify_checked"] || !got["computer_read_text"] {
		t.Fatal("document check exposed legacy verification", got)
	}
	task.Workflow.Checks = append(task.Workflow.Checks, agents.OutcomeCheck{Expectation: agents.OutcomeExpectation{Kind: "control_checked_equals"}, Binding: &agents.OutcomeBinding{}})
	got = availableNames(projector.ProjectModelTools(task, tools))
	if got["computer_verify"] || !got["computer_verify_checked"] {
		t.Fatal("explicit non-document check lost", got)
	}
}
