package agents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

const WorkflowEvidenceVersion = 1

// Explicit opt-in for document-aware tasks. Existing v1 tasks and producers
// keep their wire contract until the document provider is negotiated.
const DocumentWorkflowEvidenceVersion = 2

func knownWorkflowVersion(version int) bool {
	return version == WorkflowEvidenceVersion || version == DocumentWorkflowEvidenceVersion
}

var ErrWorkflowVersion = errors.New("unsupported task workflow evidence version")

// OutcomeExpectation describes a check, not authority or semantic proof that
// all parts of a user's request were interpreted correctly. Only the service
// supplies provenance, revisions, state and evidence below.
type OutcomeExpectation struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Kind          string  `json:"kind"`
	ApplicationID string  `json:"application_id,omitempty"`
	TargetID      string  `json:"target_id,omitempty"`
	Element       string  `json:"element,omitempty"`
	Text          *string `json:"text,omitempty"`
	Checked       *bool   `json:"checked,omitempty"`
	Document      string  `json:"document,omitempty"`
	Snapshot      string  `json:"snapshot,omitempty"`
	Command       string  `json:"command,omitempty"`
	EditMode      string  `json:"edit_mode,omitempty"`
}

type OutcomeCheck struct {
	Binding       *OutcomeBinding    `json:"binding,omitempty"`
	Expectation   OutcomeExpectation `json:"expectation"`
	Provenance    string             `json:"provenance"`
	Revision      int                `json:"revision"`
	State         string             `json:"state"`
	EvidenceSteps []int              `json:"evidence_steps,omitempty"`
	Reason        string             `json:"reason,omitempty"`
	CheckedAt     *time.Time         `json:"checked_at,omitempty"`
}

// Service authored from a canonical host observation; never a model argument.
type OutcomeBinding struct {
	Session      string                  `json:"session"`
	TargetSHA256 string                  `json:"target_sha256"`
	Identity     string                  `json:"identity,omitempty"`
	Observation  string                  `json:"observation"`
	Step         int                     `json:"step"`
	Document     *OutcomeDocumentBinding `json:"document,omitempty"`
}

type WorkflowRevision struct {
	SupersededBy      string         `json:"superseded_by"`
	Revision          int            `json:"revision"`
	InstructionID     string         `json:"instruction_id"`
	InstructionSHA256 string         `json:"instruction_sha256"`
	Checks            []OutcomeCheck `json:"checks"`
}

// WorkflowEvidence lives in the authoritative task snapshot, with the same
// transaction and ownership as results/events. Unknown versions remain opaque
// so inspection/copying cannot strip future security-relevant fields.
type WorkflowEvidence struct {
	Version           int                `json:"version"`
	Revision          int                `json:"revision"`
	InstructionID     string             `json:"instruction_id"`
	InstructionSHA256 string             `json:"instruction_sha256"`
	Checks            []OutcomeCheck     `json:"checks"`
	History           []WorkflowRevision `json:"history,omitempty"`
	State             string             `json:"state"`
	unknown           json.RawMessage
}

func (w *WorkflowEvidence) UnmarshalJSON(data []byte) error {
	if len(data) > 256<<10 {
		return fmt.Errorf("workflow evidence exceeds snapshot limit")
	}
	var envelope struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if !knownWorkflowVersion(envelope.Version) {
		*w = WorkflowEvidence{Version: envelope.Version, unknown: append(json.RawMessage(nil), data...)}
		return nil
	}
	if envelope.Version == WorkflowEvidenceVersion && legacyDocumentFields(data) {
		return fmt.Errorf("document evidence requires workflow version 2")
	}
	type plain WorkflowEvidence
	var value plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return err
	}
	*w = WorkflowEvidence(value)
	return nil
}

func (w WorkflowEvidence) MarshalJSON() ([]byte, error) {
	if !knownWorkflowVersion(w.Version) && len(w.unknown) != 0 {
		return append([]byte(nil), w.unknown...), nil
	}
	type plain WorkflowEvidence
	data, err := json.Marshal(plain(w))
	if err == nil && w.Version == WorkflowEvidenceVersion && legacyDocumentFields(data) {
		return nil, fmt.Errorf("document evidence requires workflow version 2")
	}
	if err == nil && len(data) > 256<<10 {
		return nil, fmt.Errorf("workflow evidence exceeds snapshot limit")
	}
	return data, err
}

func WorkflowVersionSupported(t *Task) bool {
	return t != nil && (t.Config.NativeWorkflowVersion == 0 || knownWorkflowVersion(t.Config.NativeWorkflowVersion)) &&
		(t.Workflow == nil || (knownWorkflowVersion(t.Workflow.Version) &&
			(t.Config.NativeWorkflowVersion == 0 || t.Config.NativeWorkflowVersion == t.Workflow.Version)))
}

func newWorkflowEvidence(t *Task) *WorkflowEvidence {
	sum := sha256.Sum256([]byte(t.Prompt))
	version := WorkflowEvidenceVersion
	if t.Config.NativeWorkflowVersion == DocumentWorkflowEvidenceVersion {
		version = DocumentWorkflowEvidenceVersion
	}
	return &WorkflowEvidence{Version: version, Revision: 1,
		InstructionID: "initial", InstructionSHA256: hex.EncodeToString(sum[:]),
		Checks: []OutcomeCheck{}, State: "pending"}
}

func validateWorkflowEvidence(w *WorkflowEvidence) error {
	if w == nil || !knownWorkflowVersion(w.Version) {
		return nil // unknown records are readable, but Continue refuses execution
	}
	if w.Revision < 1 || w.Revision > 33 || len(w.History) > 32 || len(w.Checks) > 12 || w.InstructionID == "" || len(w.InstructionID) > 128 {
		return fmt.Errorf("invalid workflow evidence envelope")
	}
	if digest, err := hex.DecodeString(w.InstructionSHA256); err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("invalid workflow instruction digest")
	}
	switch w.State {
	case "pending", "partial", "unverified", "checks_passed":
	default:
		return fmt.Errorf("invalid workflow assessment")
	}
	seen := map[string]bool{}
	for _, check := range w.Checks {
		e := check.Expectation
		if validateExpectationVersion(e, w.Version) != nil || validateOutcomeDocumentBinding(check.Binding, w.Version) != nil || seen[e.ID] || check.Revision < 1 || check.Revision > w.Revision || (check.Provenance != "model_proposed" && check.Provenance != "caller_explicit") {
			return fmt.Errorf("invalid workflow expectation")
		}
		seen[e.ID] = true
		switch check.State {
		case "pending", "verified", "unavailable":
		default:
			return fmt.Errorf("invalid workflow check state")
		}
	}
	for _, revision := range w.History {
		if len(revision.Checks) > 12 {
			return fmt.Errorf("invalid historical workflow checks")
		}
		for _, check := range revision.Checks {
			if validateExpectationVersion(check.Expectation, w.Version) != nil || validateOutcomeDocumentBinding(check.Binding, w.Version) != nil {
				return fmt.Errorf("invalid historical workflow evidence")
			}
		}
	}
	return nil
}

func validExpectation(e OutcomeExpectation) bool {
	return validateExpectation(e) == nil
}

func validateExpectation(e OutcomeExpectation) error {
	if e.ID == "" || len(e.ID) > 64 || strings.ContainsAny(e.ID, " \t\r\n\x00") || strings.TrimSpace(e.Title) == "" || len(e.Title) > 512 || !utf8.ValidString(e.Title) {
		return fmt.Errorf("id must be 1-64 bytes without whitespace; title must be nonempty UTF-8 up to 512 bytes")
	}
	if e.Kind == "" || len(e.Kind) > 64 || len(e.ApplicationID) > 256 || len(e.TargetID) > 256 || len(e.Element) > 256 || (e.Text != nil && (len(*e.Text) > 4000 || !utf8.ValidString(*e.Text))) {
		return fmt.Errorf("kind must be 1-64 bytes, host references at most 256 bytes, and text valid UTF-8 up to 4000 bytes")
	}
	switch e.Kind {
	case "application_open":
		if e.ApplicationID == "" || e.Text != nil || e.Checked != nil || e.Element != "" {
			return fmt.Errorf("application_open requires application_id from computer_applications; omit element, text and checked")
		}
	case "control_text_equals":
		if e.Element == "" {
			return fmt.Errorf("control_text_equals is missing required field element. Keep the other fields and add element with the observed CONTROL id. target_id is the WINDOW id and cannot replace element")
		}
		if e.TargetID == "" || e.Element == "" || e.Text == nil || e.Checked != nil {
			return fmt.Errorf("control_text_equals requires target_id and element from the latest observed application, plus text; omit checked. Do not invent target or element references")
		}
	case "control_checked_equals":
		if e.Element == "" {
			return fmt.Errorf("control_checked_equals is missing required field element. Keep the other fields and add element with the observed CONTROL id. target_id is the WINDOW id and cannot replace element")
		}
		if e.TargetID == "" || e.Element == "" || e.Checked == nil || e.Text != nil {
			return fmt.Errorf("control_checked_equals requires observed target_id, element and checked; omit text. Do not invent target or element references")
		}
	default:
		if e.Text != nil || e.Checked != nil {
			return fmt.Errorf("unsupported outcome kinds cannot take text or checked predicates; use a supported control kind after observing the application")
		}
	}
	return nil
}

func ValidateExpectations(items []OutcomeExpectation) error {
	return ValidateExpectationsVersion(items, WorkflowEvidenceVersion)
}

func ValidateExpectationsVersion(items []OutcomeExpectation, version int) error {
	if !knownWorkflowVersion(version) {
		return ErrWorkflowVersion
	}
	if len(items) > 12 {
		return fmt.Errorf("use at most 12 expectations")
	}
	seen := map[string]bool{}
	for i, e := range items {
		if err := validateExpectationVersion(e, version); err != nil {
			return fmt.Errorf("expectations[%d]: %w", i, err)
		}
		if seen[e.ID] {
			return fmt.Errorf("expectations[%d]: duplicate id; use a distinct ID for each outcome", i)
		}
		seen[e.ID] = true
	}
	return nil
}

func usesWorkflow(t *Task) bool {
	return t.Config.TaskFirst && knownWorkflowVersion(t.Config.NativeWorkflowVersion) && t.Config.ComputerDriver != "" && t.Config.ComputerDriver != "browser" && t.Config.ComputerSession != ""
}

func initializeWorkflow(t *Task) error {
	if !usesWorkflow(t) || t.Workflow != nil {
		return nil
	}
	t.Workflow = newWorkflowEvidence(t)
	return declareExpectations(t, t.Config.NativeExpectations, "caller_explicit")
}

// Declarations are additive and immutable within a user revision. Only the
// authenticated steering path revises intent, never model or source text.
func declareExpectations(t *Task, items []OutcomeExpectation, provenance string) error {
	if !usesWorkflow(t) || t.Workflow == nil || !WorkflowVersionSupported(t) {
		return ErrWorkflowVersion
	}
	if err := ValidateExpectationsVersion(items, t.Workflow.Version); err != nil {
		return err
	}
	next := append([]OutcomeCheck(nil), t.Workflow.Checks...)
	for _, e := range items {
		found := false
		for _, old := range next {
			if old.Expectation.ID == e.ID {
				if !reflect.DeepEqual(old.Expectation, e) {
					return fmt.Errorf("expectation IDs and values cannot be changed by the model; user steering is required")
				}
				found = true
			}
		}
		if !found {
			next = append(next, OutcomeCheck{Expectation: e, Provenance: provenance, Revision: t.Workflow.Revision, State: "pending"})
		}
	}
	if len(next) > 12 {
		return fmt.Errorf("use at most 12 expectations per revision")
	}
	t.Workflow.Checks = next
	return nil
}

func reviseWorkflow(t *Task, instructionID, digest string) error {
	if t.Workflow == nil {
		return nil
	}
	if !WorkflowVersionSupported(t) {
		return ErrWorkflowVersion
	}
	w := t.Workflow
	w.History = append(w.History, WorkflowRevision{Revision: w.Revision, InstructionID: w.InstructionID, InstructionSHA256: w.InstructionSHA256, Checks: w.Checks, SupersededBy: instructionID})
	w.Revision++
	w.InstructionID, w.InstructionSHA256 = instructionID, digest
	w.Checks, w.State = []OutcomeCheck{}, "pending"
	t.Checkpoint.CompletionRepairs, t.Checkpoint.CompletionProgress = 0, ""
	return nil
}

func workflowMutation(name string, d capabilities.Descriptor) bool {
	// Explicitly reviewed native reads only; MCP annotations are not authority.
	if d.Kind == capabilities.Computer {
		switch name {
		case "computer_applications", "computer_windows", "computer_observe", "computer_verify", "computer_verify_checked", "computer_verify_application", "computer_find_controls", "computer_inspect_control":
			return false
		}
	}
	return d.Kind != capabilities.Read
}

// Used in both initial model context and rejection feedback, including old
// saved tasks whose persisted system prompt predates this workflow contract.
func workflowDeclarationGuidance(t *Task) string {
	if t.Config.NativeWorkflowVersion == DocumentWorkflowEvidenceVersion {
		return "Preserve saved checks. After application readiness call computer_documents. Before creating, declare document_blank with its observed target_id and command. Then read the resulting document. Before editing declare document_edit_matches with target_id, document, snapshot, edit_mode and exact requested text. Use only observed references; never guess a future document. Every check requires id, title, kind. Declarations do not grant permission or verify input."
	}
	if t.Workflow != nil && len(t.Workflow.Checks) > 0 {
		return "Keep the saved checks and continue the current stage; do not rediscover or reopen an already observed application. For the pending edit call task_expectations with a NEW id, title, kind=control_text_equals, target_id=observation.target.id, element=the control's id, text=the exact requested text. target_id and element are different required fields. For a checkbox use control_checked_equals and checked instead of text. Observe again only if needed for current IDs."
	}
	if t.Config.ComputerScope != "desktop" {
		return "Call computer_observe for the selected window, then task_expectations with control_text_equals (target_id, element, text) or control_checked_equals (target_id, element, checked) BEFORE editing. Each item requires id, title, kind. Use observed IDs, never guesses. Declarations add checks, not permission; do not request unavailable application-launch tools."
	}
	return "Declare checks in stages: computer_applications -> task_expectations with application_open and its application_id -> computer_open_application -> computer_verify_application. Then add control_text_equals or control_checked_equals using the observed target_id and element BEFORE editing. Never invent window/control IDs or declare control checks before observing them. Each item requires id, title, kind; calls add checks, never replace them."
}

func workflowTool() api.Tool {
	str := map[string]any{"type": "string", "maxLength": 256}
	target := map[string]any{"type": "string", "maxLength": 256, "description": "WINDOW ID: observation.target.id"}
	element := map[string]any{"type": "string", "maxLength": 256, "description": "CONTROL ID: required separately from target_id"}
	item := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"id", "title", "kind"}, "properties": map[string]any{
		"id": map[string]any{"type": "string", "maxLength": 64}, "title": map[string]any{"type": "string", "maxLength": 512},
		"kind":           map[string]any{"type": "string", "maxLength": 64, "description": "application_open requires application_id; control_text_equals requires target_id, element, text; control_checked_equals requires target_id, element, checked; other kinds remain unverified"},
		"application_id": str, "target_id": target, "element": element, "text": map[string]any{"type": "string", "maxLength": 4000}, "checked": map[string]any{"type": "boolean"},
	}}
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: "task_expectations", Description: "Record 1-12 immutable outcome checks, not permission. Start with {\"expectations\":[{\"id\":\"open\",\"title\":\"Application ready\",\"kind\":\"application_open\",\"application_id\":\"<ID from computer_applications>\"}]}. After observing controls, add control checks before editing. Only user steering can replace checks. Other kinds (research, saved_file) remain unverified.", Parameters: map[string]interface{}{"type": "object", "additionalProperties": false, "required": []string{"expectations"}, "properties": map[string]interface{}{"expectations": map[string]any{"type": "array", "minItems": 1, "maxItems": 12, "items": item}}}}}
}

// Expose the exact declaration shape during a service-authored edit repair.
// A simple flat required list works with small-model tool templates without
// relying on conditional/union schema support. Canonical validation remains
// unchanged; this neither fills missing arguments nor dispatches anything.
func constrainWorkflowRepairTool(tool *api.Tool, task *Task) {
	if task.Checkpoint == nil || !task.Checkpoint.WorkflowEditPending {
		return
	}
	for i := len(task.Steps) - 1; i >= 0; i-- {
		s := task.Steps[i]
		var failure struct {
			Code string `json:"error"`
		}
		if s.Type != "rejected" || json.Unmarshal([]byte(s.ToolResult), &failure) != nil || failure.Code != "workflow_control_expectation_required" {
			continue
		}
		if task.Config.NativeWorkflowVersion == DocumentWorkflowEvidenceVersion && constrainDocumentRepairTool(tool, s.ToolName) {
			return
		}
		kind, value := "control_text_equals", "text"
		if s.ToolName == "computer_set_checked" {
			kind, value = "control_checked_equals", "checked"
		} else if s.ToolName != "computer_replace_text" {
			return
		}
		item := tool.Function.Parameters["properties"].(map[string]interface{})["expectations"].(map[string]any)["items"].(map[string]any)
		item["required"] = []string{"id", "title", "kind", "target_id", "element", value}
		properties := item["properties"].(map[string]any)
		properties["kind"] = map[string]any{"type": "string", "enum": []string{kind}}
		delete(properties, "application_id")
		if value == "text" {
			delete(properties, "checked")
		} else {
			delete(properties, "text")
		}
		tool.Function.Description = "Repair the rejected edit by declaring its matching check. All six fields are required: id, title, kind, target_id (window), element (control), and " + value + " (exact requested value). Existing checks are preserved. This only records metadata; it grants no permission and performs no edit."
		return
	}
}
