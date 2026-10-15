package agents

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDocumentModelCheckProjectionPreservesCanonicalEvidence(t *testing.T) {
	text := "OffGrid test — café 日本語 12345"
	task := &Task{Prompt: "Do not change another document", Workflow: &WorkflowEvidence{
		Version: DocumentWorkflowEvidenceVersion,
		Checks: []OutcomeCheck{{
			Expectation: OutcomeExpectation{ID: "write", Title: "Exact text", Kind: "document_edit_matches", Document: "doc", Snapshot: "snapshot", EditMode: "insert_text", Text: &text},
			State:       "verified", EvidenceSteps: []int{1, 2, 3}, Provenance: "model_proposed", Revision: 1,
			Binding: &OutcomeBinding{Session: "canonical-session", TargetSHA256: strings.Repeat("a", 64), Observation: "canonical-observation", Step: 1},
		}},
	}}
	before, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	view, err := json.Marshal(modelWorkflowChecks(task))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"verified", "evidence_steps", "insert_text", "snapshot", text} {
		if !strings.Contains(string(view), want) {
			t.Fatal("projection lost intent or progress", want)
		}
	}
	for _, omitted := range []string{"canonical-session", "canonical-observation", "target_sha256", "provenance"} {
		if strings.Contains(string(view), omitted) {
			t.Fatal("redundant canonical metadata in model projection", omitted)
		}
	}
	after, err := json.Marshal(task)
	if err != nil || string(before) != string(after) {
		t.Fatal("projection changed saved history")
	}
	task.Workflow.Version = WorkflowEvidenceVersion
	legacy, err := json.Marshal(modelWorkflowChecks(task))
	if err != nil || !strings.Contains(string(legacy), "canonical-session") {
		t.Fatal("legacy projection changed")
	}
}

func TestDocumentVersionKeepsLegacyControlRepairShape(t *testing.T) {
	cfg := workflowConfig()
	cfg.NativeWorkflowVersion = DocumentWorkflowEvidenceVersion
	task := &Task{Config: cfg, Checkpoint: &Checkpoint{WorkflowEditPending: true}, Steps: []Step{{Type: "rejected", ToolName: "computer_replace_text", ToolArgs: `{"element":"control","text":"hello"}`, ToolResult: `{"error":"workflow_control_expectation_required"}`}}}
	for _, tool := range runtimeTools(task) {
		if tool.Function.Name != "task_expectations" {
			continue
		}
		b, err := json.Marshal(tool)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "control_text_equals") || strings.Contains(string(b), `"document"`) || strings.Contains(string(b), `"snapshot"`) {
			t.Fatal("legacy edit requires unavailable document fields", string(b))
		}
		return
	}
	t.Fatal("missing legacy expectation repair")
}
