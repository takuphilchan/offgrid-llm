package agents

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func documentExpectationFixture() OutcomeExpectation {
	text := "café 日本語 😀"
	return OutcomeExpectation{ID: "write", Title: "Exact inserted text", Kind: "document_edit_matches", TargetID: "window", Document: "document", Snapshot: "snapshot", EditMode: "insert_text", Text: &text}
}

func TestDocumentWorkflowVersionsPersistWithoutUpgradingHistory(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithPersistence(nil, nil, nil, dir)
			task, err := m.CreateTask("versioned", "write a document", nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.updateTask(task.ID, func(task *Task) error {
				task.Actor = "owner"
				task.Config = workflowConfig()
				task.Config.NativeWorkflowVersion = version
				task.Workflow = newWorkflowEvidence(task)
				if version == 2 {
					return declareExpectations(task, []OutcomeExpectation{documentExpectationFixture()}, "caller_explicit")
				}
				return declareExpectations(task, []OutcomeExpectation{{ID: "open", Title: "Application open", Kind: "application_open", ApplicationID: "app"}}, "caller_explicit")
			})
			if err != nil {
				t.Fatal(err)
			}
			m = NewManagerWithPersistence(nil, nil, nil, dir)
			if m.StorageError() != nil {
				t.Fatal(m.StorageError())
			}
			saved, ok := m.GetTask(task.ID)
			if !ok || saved.Workflow.Version != version || saved.Config.NativeWorkflowVersion != version || !WorkflowVersionSupported(saved) {
				t.Fatal("version or checks lost on reload")
			}
			data, err := json.Marshal(saved.Workflow)
			if err != nil {
				t.Fatal(err)
			}
			if version == 1 && bytes.Contains(data, []byte(`"document"`)) {
				t.Fatal("legacy projection changed")
			}
			if version == 2 && saved.Workflow.Checks[0].Expectation.Document != "document" {
				t.Fatal("document identity dropped")
			}
			saved.Config.NativeWorkflowVersion = 3 - version
			if WorkflowVersionSupported(saved) {
				t.Fatal("mismatched versions accepted")
			}
		})
	}
}

func TestDocumentExpectationsRequireVersionAndSeparateModes(t *testing.T) {
	valid := documentExpectationFixture()
	if ValidateExpectations([]OutcomeExpectation{valid}) == nil {
		t.Fatal("v1 accepted document predicate")
	}
	if err := ValidateExpectationsVersion([]OutcomeExpectation{valid}, 2); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*OutcomeExpectation){
		func(e *OutcomeExpectation) { e.EditMode = "shortcut" }, func(e *OutcomeExpectation) { e.Snapshot = "" },
		func(e *OutcomeExpectation) { e.Document = "" }, func(e *OutcomeExpectation) { e.Command = "invented" },
		func(e *OutcomeExpectation) { e.Element = "field" }, func(e *OutcomeExpectation) { e.Text = nil },
		func(e *OutcomeExpectation) { value := "bad\x00"; e.Text = &value }, func(e *OutcomeExpectation) { value := strings.Repeat("a", 16001); e.Text = &value },
	} {
		e := valid
		change(&e)
		if ValidateExpectationsVersion([]OutcomeExpectation{e}, 2) == nil {
			t.Fatal("invalid predicate accepted")
		}
	}
	blank := OutcomeExpectation{ID: "new", Title: "New blank document", Kind: "document_blank", TargetID: "window", Command: "new-command"}
	if err := ValidateExpectationsVersion([]OutcomeExpectation{blank}, 2); err != nil {
		t.Fatal(err)
	}
	blank.Document = "guessed-future-tab"
	if ValidateExpectationsVersion([]OutcomeExpectation{blank}, 2) == nil {
		t.Fatal("model bound future creation identity")
	}
	valid.Kind = "document_text_equals"
	valid.EditMode = ""
	if err := ValidateExpectationsVersion([]OutcomeExpectation{valid}, 2); err != nil {
		t.Fatal(err)
	}
	valid.Kind = "research"
	if ValidateExpectationsVersion([]OutcomeExpectation{valid}, 2) == nil {
		t.Fatal("document fields on unrelated kind accepted")
	}
}

func TestLegacyWorkflowRejectsDocumentFieldsEvenNullAndHistorical(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"checks":[{"expectation":{"document":null}}]}`,
		`{"version":1,"checks":[{"expectation":{"snapshot":""}}]}`,
		`{"version":1,"checks":[{"binding":{"document":null}}]}`,
		`{"version":1,"history":[{"checks":[{"expectation":{"edit_mode":null}}]}]}`,
	} {
		var w WorkflowEvidence
		if json.Unmarshal([]byte(raw), &w) == nil {
			t.Fatal("v2 fields silently downgraded", raw)
		}
	}
	raw := []byte(`{"version":700,"checks":[{"new_private_field":true}]}`)
	var w WorkflowEvidence
	if json.Unmarshal(raw, &w) != nil {
		t.Fatal("unknown record not inspectable")
	}
	out, err := json.Marshal(w)
	if err != nil || !bytes.Equal(raw, out) {
		t.Fatal("unknown fields lost")
	}
	if WorkflowVersionSupported(&Task{Workflow: &w}) {
		t.Fatal("future version executable")
	}
}
