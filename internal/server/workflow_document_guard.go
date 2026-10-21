package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

// Arguments contain references and requested text only. Document identity,
// preimages, action identities and result digests always come from host records.
type documentEditArguments struct {
	Observation string  `json:"observation_id"`
	Command     string  `json:"command,omitempty"`
	Document    string  `json:"document,omitempty"`
	Snapshot    string  `json:"snapshot,omitempty"`
	Text        *string `json:"text,omitempty"`
}

// Match the closed public argument shape before interpreting it. In particular,
// null, repeated keys, case-folded aliases and irrelevant empty fields must not
// acquire a different meaning at another layer of the dispatch path.
func decodeDocumentEditArguments(name string, raw []byte) (documentEditArguments, bool) {
	var out documentEditArguments
	if len(raw) == 0 || len(raw) > 128<<10 || jsonwire.Validate(raw) != nil {
		return out, false
	}
	fields := map[string]*string{"observation_id": &out.Observation}
	text := ""
	if name == "computer_create_document" {
		fields["command"] = &out.Command
	} else if documentEditModeForTool(name) != "" {
		fields["document"], fields["snapshot"], fields["text"] = &out.Document, &out.Snapshot, &text
	} else {
		return out, false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return out, false
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		name, ok := key.(string)
		field := fields[name]
		if err != nil || !ok || field == nil || seen[name] {
			return out, false
		}
		value, err := d.Token()
		str, ok := value.(string)
		if err != nil || !ok {
			return out, false
		}
		*field, seen[name] = str, true
	}
	if end, err := d.Token(); err != nil || end != json.Delim('}') || len(seen) != len(fields) || d.Decode(new(any)) != io.EOF {
		return out, false
	}
	if name != "computer_create_document" {
		out.Text = &text
	}
	return out, true
}

type savedDocumentState struct {
	inventory computer.DocumentInventory
	read      computer.DocumentReadback
	step      int
	readStep  int
}

func documentState(t *agents.Task) (savedDocumentState, error) {
	var state savedDocumentState
	if t == nil || !agents.WorkflowVersionSupported(t) || t.Workflow == nil || t.Workflow.Version != agents.DocumentWorkflowEvidenceVersion || t.ComputerSessionExpired || t.Config.ComputerSession == "" || !nativeComputerDriver(t.Config.ComputerDriver) || t.Config.ComputerStepOffset < 0 || t.Config.ComputerStepOffset > len(t.Steps) {
		return state, computer.ErrSession
	}
	for i := t.Config.ComputerStepOffset; i < len(t.Steps); i++ {
		s := t.Steps[i]
		if s.ID != i+1 {
			return savedDocumentState{}, computer.ErrInvalidControl
		}
		if s.Type == "task_metadata" || s.Type == "rejected" || s.Type == "completion_rejected" {
			continue
		}
		switch s.ToolName {
		case "computer_documents":
			state = savedDocumentState{}
			if s.Type != "action" || !decodeNativeModelResult(s.ToolResult, &state.inventory) || state.inventory.Observation.Target.Driver != t.Config.ComputerDriver || !state.inventory.Complete || state.inventory.Validate(state.inventory.Observation.Target, state.inventory.Observation.CapturedAt) != nil {
				return savedDocumentState{}, computer.ErrDocumentIncomplete
			}
			state.step = s.ID
		case "computer_read_text":
			state.read, state.readStep = computer.DocumentReadback{}, 0
			if s.Type != "action" || !decodeNativeModelResult(s.ToolResult, &state.read) || state.read.Validate() != nil || state.inventory.Active == nil || !computer.SameDocument(*state.inventory.Active, state.read.Binding) || state.inventory.Active.Revision != state.read.Binding.Revision {
				return savedDocumentState{}, computer.ErrDocumentChanged
			}
			state.readStep = s.ID
		default:
			// Unknown native operations are conservative. MCP metadata cannot
			// introduce a document or clear the input boundary.
			if strings.HasPrefix(s.ToolName, "computer_") && s.ToolName != "computer_find_controls" && s.ToolName != "computer_inspect_control" && s.ToolName != "computer_applications" {
				state = savedDocumentState{}
			}
		}
	}
	return state, nil
}

func observedDocumentCommand(state savedDocumentState, command string) bool {
	if state.step == 0 || command == "" {
		return false
	}
	for _, c := range state.inventory.Commands {
		if c.ID == command {
			return true
		}
	}
	return false
}

func documentEditModeForTool(name string) string {
	switch name {
	case "computer_insert_text":
		return "insert_text"
	case "computer_replace_selection":
		return "replace_selection"
	case "computer_replace_text":
		return "replace_value"
	}
	return ""
}

func bindDocumentDeclaration(t *agents.Task, check *agents.OutcomeCheck, state savedDocumentState) error {
	e := check.Expectation
	if check.Binding != nil {
		return nil // Immutable once declared; never silently retarget a check.
	}
	if state.step == 0 || state.inventory.Observation.Target.ID != e.TargetID {
		return fmt.Errorf("declare document checks from computer_documents for the current target")
	}
	b := &agents.OutcomeBinding{Session: t.Config.ComputerSession, TargetSHA256: evidenceTargetDigest(state.inventory.Observation.Target), Observation: state.inventory.Observation.ID, Step: state.step}
	if e.Kind == "document_blank" {
		if !observedDocumentCommand(state, e.Command) {
			return fmt.Errorf("declare blankness using an observed creation command, not a future document")
		}
	} else {
		r := state.read
		if state.readStep == 0 || r.Binding.Document != e.Document || r.Snapshot != e.Snapshot {
			return fmt.Errorf("read the active document before declaring its exact snapshot and edit mode")
		}
		b.Observation, b.Step = r.Binding.Observation, state.readStep
		b.Document = outcomeDocumentIdentity(r.Binding, r.Snapshot)
	}
	check.Binding = b
	return nil
}

func outcomeDocumentIdentity(b computer.DocumentBinding, snapshot string) *agents.OutcomeDocumentBinding {
	return &agents.OutcomeDocumentBinding{ID: b.Document, Generation: b.Generation, Container: b.Container, Editor: b.Editor, Revision: b.Revision, Snapshot: snapshot}
}

func validateDocumentWorkflowEdit(t *agents.Task, name string, raw json.RawMessage) error {
	state, err := documentState(t)
	if err != nil {
		return err
	}
	a, valid := decodeDocumentEditArguments(name, raw)
	if !valid || state.step == 0 {
		return computer.ErrInvalidControl
	}
	creation := name == "computer_create_document"
	mode := documentEditModeForTool(name)
	if creation {
		if a.Document != "" || a.Snapshot != "" || a.Text != nil || a.Observation != state.inventory.Observation.ID || !observedDocumentCommand(state, a.Command) {
			return computer.ErrInvalidControl
		}
	} else if mode == "" || a.Command != "" || a.Text == nil || computer.ValidateDocumentInput(*a.Text) != nil || state.readStep == 0 || a.Document != state.read.Binding.Document || a.Snapshot != state.read.Snapshot || a.Observation != state.read.Binding.Observation {
		return computer.ErrDocumentChanged
	}
	for _, c := range t.Workflow.Checks {
		e, b := c.Expectation, c.Binding
		if b == nil || b.Session != t.Config.ComputerSession || b.TargetSHA256 != evidenceTargetDigest(state.inventory.Observation.Target) || e.TargetID != state.inventory.Observation.Target.ID {
			continue
		}
		if creation && e.Kind == "document_blank" && e.Command == a.Command && b.Document == nil && b.Observation == a.Observation && b.Step == state.step {
			return nil
		}
		if !creation && e.Kind == "document_edit_matches" && e.Text != nil && *e.Text == *a.Text && e.EditMode == mode && e.Document == a.Document && e.Snapshot == a.Snapshot && b.Document != nil && b.Document.ActionID == "" && matchesDocumentOutcome(b, state.read.Binding) && b.Document.Revision == state.read.Binding.Revision && b.Document.Snapshot == a.Snapshot {
			return nil
		}
	}
	return &workflowControlExpectationError{targetID: state.inventory.Observation.Target.ID, document: a.Document, snapshot: a.Snapshot, command: a.Command, editMode: mode}
}

// Fill only result-derived fields after one canonical successful dispatch. This
// runs on a copy during assessment, which the runner persists in the same task
// transaction as the result. A crash before persistence never creates new input.
func bindRecordedDocumentAction(t *agents.Task, check *agents.OutcomeCheck) {
	b := check.Binding
	if b == nil || b.Session != t.Config.ComputerSession || (b.Document != nil && b.Document.ActionID != "") || check.Expectation.Kind == "document_text_equals" {
		return
	}
	for i := b.Step; i < len(t.Steps); i++ {
		s := t.Steps[i]
		if s.ID != i+1 {
			return
		}
		if s.Type == "task_metadata" || s.Type == "rejected" || s.Type == "completion_rejected" {
			continue
		}
		if !documentMutationTool(s.ToolName) {
			if strings.HasPrefix(s.ToolName, "computer_") && s.ToolName != "computer_read_text" && s.ToolName != "computer_documents" {
				return
			}
			continue
		}
		if check.ActionDerivedCallID != "" && s.CallID != check.ActionDerivedCallID {
			return
		}
		var a computer.DocumentActionEvidence
		args, valid := decodeDocumentEditArguments(s.ToolName, []byte(s.ToolArgs))
		if s.Type != "action" || !decodeNativeModelResult(s.ToolResult, &a) || a.Validate() != nil || !valid || documentOperationTool(a.Operation) != s.ToolName || a.After.Target.Driver != t.Config.ComputerDriver || evidenceTargetDigest(a.After.Target) != b.TargetSHA256 {
			return
		}
		e := check.Expectation
		var document *agents.OutcomeDocumentBinding
		if e.Kind == "document_blank" {
			if a.Command != e.Command || args.Command != e.Command || args.Observation != b.Observation || args.Document != "" || args.Snapshot != "" || args.Text != nil || a.Before != nil || b.Document != nil {
				return
			}
			document = outcomeDocumentIdentity(a.After, a.AfterSnapshot)
		} else {
			if a.Before == nil || !matchesDocumentOutcome(b, *a.Before) || b.Document.Revision != a.Before.Revision || b.Document.Snapshot != a.BeforeSnapshot || e.Document != args.Document || e.Snapshot != args.Snapshot || args.Observation != b.Observation || args.Command != "" || e.Text == nil || args.Text == nil || *e.Text != *args.Text || a.InputSHA256 != computer.DocumentTextDigest(*e.Text) || e.EditMode != a.Operation {
				return
			}
			d := *b.Document
			document = &d
		}
		document.ActionID, document.Operation = a.ActionID, a.Operation
		document.PreimageSHA256, document.ExpectedSHA256 = a.PreimageSHA256, a.ExpectedSHA256
		copyBinding := *b
		copyBinding.Document = document
		check.Binding = &copyBinding
		return
	}
}
