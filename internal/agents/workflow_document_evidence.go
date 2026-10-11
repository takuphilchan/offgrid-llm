package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Canonical host/service fields, never declaration arguments. An editor alone
// cannot identify a document: tabs may reuse the same input control.
type OutcomeDocumentBinding struct {
	ID             string `json:"id"`
	Generation     string `json:"generation"`
	Container      string `json:"container"`
	Editor         string `json:"editor"`
	Revision       string `json:"revision"`
	Snapshot       string `json:"snapshot"`
	ActionID       string `json:"action_id,omitempty"`
	Operation      string `json:"operation,omitempty"`
	PreimageSHA256 string `json:"preimage_sha256,omitempty"`
	ExpectedSHA256 string `json:"expected_sha256,omitempty"`
}

func documentReference(value string) bool {
	return value != "" && len(value) <= 256 && utf8.ValidString(value) && !strings.ContainsAny(value, " \t\r\n\x00")
}
func evidenceDigest(value string) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == sha256.Size && value == strings.ToLower(value)
}
func documentEditMode(mode string) bool {
	return mode == "insert_text" || mode == "replace_selection" || mode == "replace_value"
}

func validateOutcomeDocumentBinding(binding *OutcomeBinding, version int) error {
	if binding == nil || binding.Document == nil {
		return nil
	}
	d := binding.Document
	if version != DocumentWorkflowEvidenceVersion || !documentReference(binding.Session) || !evidenceDigest(binding.TargetSHA256) || !documentReference(binding.Observation) || binding.Step < 1 {
		return fmt.Errorf("invalid document evidence origin")
	}
	for _, value := range []string{d.ID, d.Generation, d.Container, d.Editor, d.Revision, d.Snapshot} {
		if !documentReference(value) {
			return fmt.Errorf("invalid document identity")
		}
	}
	if d.ActionID == "" {
		if d.Operation != "" || d.PreimageSHA256 != "" || d.ExpectedSHA256 != "" {
			return fmt.Errorf("document edit evidence requires an action")
		}
		return nil
	}
	if !documentReference(d.ActionID) || !evidenceDigest(d.ExpectedSHA256) {
		return fmt.Errorf("invalid document action binding")
	}
	if d.Operation == "new_document" || d.Operation == "new_tab" {
		if d.PreimageSHA256 != "" {
			return fmt.Errorf("a new document has no preimage")
		}
	} else if !documentEditMode(d.Operation) || !evidenceDigest(d.PreimageSHA256) {
		return fmt.Errorf("invalid document edit preimage")
	}
	return nil
}

func validateExpectationVersion(e OutcomeExpectation, version int) error {
	documentKind := e.Kind == "document_blank" || e.Kind == "document_text_equals" || e.Kind == "document_edit_matches"
	if !documentKind {
		if e.Document != "" || e.Snapshot != "" || e.Command != "" || e.EditMode != "" {
			return fmt.Errorf("document fields require a document predicate")
		}
		return validateExpectation(e)
	}
	if version != DocumentWorkflowEvidenceVersion {
		return fmt.Errorf("document predicates require workflow version 2")
	}
	base := e
	base.Kind = "research"
	base.Text = nil
	if err := validateExpectation(base); err != nil {
		return err
	}
	if !documentReference(e.TargetID) || e.Element != "" || e.ApplicationID != "" || e.Checked != nil {
		return fmt.Errorf("document predicates require target_id, not application/control references")
	}
	if e.Kind == "document_blank" {
		if !documentReference(e.Command) || e.Document != "" || e.Snapshot != "" || e.EditMode != "" || e.Text != nil {
			return fmt.Errorf("document_blank requires an observed creation command; the service binds the created identity")
		}
		return nil
	}
	if !documentReference(e.Document) || !documentReference(e.Snapshot) || e.Command != "" || e.Text == nil || len(*e.Text) > 16000 || !utf8.ValidString(*e.Text) || strings.ContainsRune(*e.Text, 0) {
		return fmt.Errorf("document text predicates require observed document/snapshot and exact UTF-8 text up to 16000 bytes")
	}
	if e.Kind == "document_edit_matches" {
		if !documentEditMode(e.EditMode) {
			return fmt.Errorf("document_edit_matches requires an explicit edit mode")
		}
	} else if e.EditMode != "" {
		return fmt.Errorf("document_text_equals cannot select an edit mode")
	}
	return nil
}

// V1 strict decoding must still reject even null/empty V2 fields, including in
// historical revisions, rather than quietly accepting a schema downgrade.
func legacyDocumentFields(data []byte) bool {
	var envelope struct {
		Checks  []json.RawMessage `json:"checks"`
		History []struct {
			Checks []json.RawMessage `json:"checks"`
		} `json:"history"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return true
	}
	checks := envelope.Checks
	for _, history := range envelope.History {
		checks = append(checks, history.Checks...)
	}
	for _, raw := range checks {
		var check struct {
			Expectation map[string]json.RawMessage `json:"expectation"`
			Binding     map[string]json.RawMessage `json:"binding"`
		}
		if json.Unmarshal(raw, &check) != nil {
			return true
		}
		for _, key := range []string{"document", "snapshot", "command", "edit_mode"} {
			if _, ok := check.Expectation[key]; ok {
				return true
			}
		}
		if _, ok := check.Binding["document"]; ok {
			return true
		}
	}
	return false
}
