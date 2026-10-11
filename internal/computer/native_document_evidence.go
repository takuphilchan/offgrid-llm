package computer

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Read-back is a projection of a fresh native snapshot, not request text or a
// model-supplied digest. Complete text remains in the bounded host snapshot.
type DocumentReadback struct {
	Version     int                `json:"version"`
	Snapshot    string             `json:"snapshot"`
	Binding     DocumentBinding    `json:"binding"`
	Complete    bool               `json:"complete"`
	Composition string             `json:"composition"`
	TextSHA256  string             `json:"text_sha256"`
	Bytes       int                `json:"bytes"`
	Page        *DocumentTextPage  `json:"page,omitempty"`
	Selection   *DocumentTextRange `json:"selection,omitempty"`
}

// These fields are emitted from the prepared host action and actual resulting
// snapshot. They are not accepted as preparation arguments or authority.
type DocumentActionEvidence struct {
	Version        int              `json:"version"`
	ActionID       string           `json:"action_id"`
	Operation      string           `json:"operation"`
	Command        string           `json:"command,omitempty"`
	Before         *DocumentBinding `json:"before,omitempty"`
	BeforeSnapshot string           `json:"before_snapshot,omitempty"`
	After          DocumentBinding  `json:"after"`
	AfterSnapshot  string           `json:"after_snapshot"`
	PreimageSHA256 string           `json:"preimage_sha256,omitempty"`
	InputSHA256    string           `json:"input_sha256,omitempty"`
	ExpectedSHA256 string           `json:"expected_sha256"`
}

func DocumentTextDigest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
func documentDigest(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == sha256.Size && value == strings.ToLower(value)
}

// SameDocument excludes content/observation revisions only. Target, tab,
// generation, container and editor must all remain bound.
func SameDocument(a, b DocumentBinding) bool {
	if a.Validate() != nil || b.Validate() != nil {
		return false
	}
	a.Observation, b.Observation = "", ""
	a.Revision, b.Revision = "", ""
	return a == b
}

func (e DocumentReadback) Validate() error {
	if e.Version != NativeDocumentsVersion || !opaqueID(e.Snapshot) || e.Binding.Validate() != nil || !e.Complete || e.Composition != "stable" || !documentDigest(e.TextSHA256) || e.Bytes < 0 || e.Bytes > MaxDocumentSnapshotBytes {
		return ErrDocumentIncomplete
	}
	if (e.Bytes == 0) != (e.TextSHA256 == DocumentTextDigest("")) {
		return ErrDocumentIncomplete
	}
	if e.Page != nil {
		p := e.Page
		if p.Snapshot != e.Snapshot || p.Binding != e.Binding || p.Validate() != nil || p.TotalScalars > e.Bytes || len(p.Text) > e.Bytes {
			return ErrDocumentIncomplete
		}
		if p.Start == 0 && p.Next == nil && (len(p.Text) != e.Bytes || DocumentTextDigest(p.Text) != e.TextSHA256) {
			return ErrDocumentIncomplete
		}
	}
	if e.Selection != nil && (e.Page == nil || e.Selection.Start < 0 || e.Selection.End < e.Selection.Start || e.Selection.End > e.Page.TotalScalars) {
		return ErrDocumentRange
	}
	return nil
}

func NewDocumentReadback(snapshot DocumentSnapshot) (DocumentReadback, error) {
	if err := snapshot.Validate(); err != nil {
		return DocumentReadback{}, err
	}
	result := DocumentReadback{Version: NativeDocumentsVersion, Snapshot: snapshot.ID, Binding: snapshot.Binding, Complete: snapshot.Complete, Composition: snapshot.Composition, TextSHA256: DocumentTextDigest(snapshot.Text), Bytes: len(snapshot.Text)}
	return result, result.Validate()
}

func (e DocumentActionEvidence) Validate() error {
	if e.Version != NativeDocumentsVersion || !opaqueID(e.ActionID) || e.After.Validate() != nil || !opaqueID(e.AfterSnapshot) || !documentDigest(e.ExpectedSHA256) {
		return ErrDocumentUnavailable
	}
	switch e.Operation {
	case "new_document", "new_tab":
		if !opaqueID(e.Command) || e.Before != nil || e.BeforeSnapshot != "" || e.PreimageSHA256 != "" || e.InputSHA256 != "" || e.ExpectedSHA256 != DocumentTextDigest("") {
			return ErrDocumentUnavailable
		}
	case "insert_text", "replace_selection", "replace_value":
		if e.Command != "" || e.Before == nil || !SameDocument(*e.Before, e.After) || !opaqueID(e.BeforeSnapshot) || e.BeforeSnapshot == e.AfterSnapshot || e.Before.Observation == e.After.Observation || !documentDigest(e.PreimageSHA256) || !documentDigest(e.InputSHA256) {
			return ErrDocumentUnavailable
		}
	default:
		return ErrDocumentUnavailable
	}
	return nil
}

func NewDocumentEditEvidence(plan PreparedDocumentText, inventory DocumentInventory, after DocumentSnapshot, now time.Time) (DocumentActionEvidence, error) {
	op := plan.Operation()
	before := plan.Preimage()
	if plan.ID() == "" || op.Text == nil || op.Validate() != nil {
		return DocumentActionEvidence{}, ErrInvalidControl
	}
	if _, err := activeDocument(inventory, after, now); err != nil {
		return DocumentActionEvidence{}, err
	}
	if !SameDocument(before.Binding, after.Binding) || after.Text != plan.ExpectedText() || after.ID == before.ID || after.Binding.Observation == before.Binding.Observation || after.Composition != "stable" {
		return DocumentActionEvidence{}, ErrDocumentChanged
	}
	e := DocumentActionEvidence{Version: NativeDocumentsVersion, ActionID: plan.ID(), Operation: op.Kind, Before: &before.Binding, BeforeSnapshot: before.ID, After: after.Binding, AfterSnapshot: after.ID, PreimageSHA256: DocumentTextDigest(before.Text), InputSHA256: DocumentTextDigest(*op.Text), ExpectedSHA256: DocumentTextDigest(plan.ExpectedText())}
	return e, e.Validate()
}

func NewDocumentCreationEvidence(actionID, command string, before, after DocumentInventory, snapshot DocumentSnapshot, now time.Time) (DocumentActionEvidence, error) {
	if err := ValidateCreatedDocument(before, after, snapshot, now); err != nil {
		return DocumentActionEvidence{}, err
	}
	kind := ""
	for _, c := range before.Commands {
		if c.ID == command {
			kind = c.Kind
		}
	}
	e := DocumentActionEvidence{Version: NativeDocumentsVersion, ActionID: actionID, Operation: kind, Command: command, After: snapshot.Binding, AfterSnapshot: snapshot.ID, ExpectedSHA256: DocumentTextDigest("")}
	return e, e.Validate()
}

// Validate document evidence before both journal persistence and recovery. A
// successful provider call or a digest without a document identity is not proof
// of a text edit. This checks provenance; the backend must perform the read-back.
func validateDocumentResult(binding ControlBinding, action PreparedControlAction, result ControlResult) error {
	if action.Operation.Kind != "document" {
		if result.Document != nil {
			return ErrUncertain
		}
		return nil
	}
	if action.Operation.Validate() != nil || result.Uncertain != (result.Outcome == "uncertain") {
		return ErrUncertain
	}
	if result.Outcome != "verified" {
		if result.Document != nil || result.Outcome == "dispatched" {
			return ErrUncertain
		}
		return nil
	}
	op := action.Operation.Document
	if op.Kind == "select_text" || op.Kind == "select_document" {
		if result.Document != nil || !documentDigest(result.EvidenceID) {
			return ErrUncertain
		}
		return nil
	}
	e := result.Document
	if e == nil || e.Validate() != nil || e.ActionID != action.ID || e.After.Target != binding.Target {
		return ErrUncertain
	}
	if op.Kind == "create_document" {
		if (e.Operation != "new_document" && e.Operation != "new_tab") || e.Command != op.Command {
			return ErrUncertain
		}
	} else if e.Operation != op.Kind || e.Before == nil || e.Before.Document != op.Document || e.BeforeSnapshot != op.Snapshot || e.Before.Editor != action.ControlIdentity || op.Text == nil || e.InputSHA256 != DocumentTextDigest(*op.Text) {
		return ErrUncertain
	}
	return nil
}
