package execution

import (
	"context"
	"encoding/json"
	"fmt"
)

// These are execution facts, not a claim that the commands are good tests or
// that they establish semantic task completion. Full commands remain private.
type ValidationCheck struct {
	OperationID   string      `json:"operation_id"`
	Command       ProcessSpec `json:"command"`
	RequestDigest string      `json:"request_digest"`
	ReceiptDigest string      `json:"receipt_digest"`
	State         string      `json:"state"`
	ExitCode      *int        `json:"exit_code,omitempty"`
}

type ValidationSummary struct {
	Version             int    `json:"version"`
	ID                  string `json:"id"`
	State               string `json:"state"`
	InputManifestDigest string `json:"input_manifest_digest"`
	CheckCount          int    `json:"check_count"`
}

func (s ValidationSummary) ValidFor(record WorkingCopyRecord) bool {
	if s == (ValidationSummary{}) {
		return true
	}
	if s.Version != 1 || record.Purpose != "test" || !validCopyID(s.ID) || !validCopyID(s.InputManifestDigest) || s.InputManifestDigest != record.ManifestDigest || s.CheckCount < 0 || s.CheckCount > 128 {
		return false
	}
	switch s.State {
	case "passed":
		return s.CheckCount > 0
	case "not_run":
		return s.CheckCount == 0
	case "failed", "uncertain", "inputs_changed":
		return true
	default:
		return false
	}
}

type ValidationPage struct {
	Summary    ValidationSummary `json:"summary"`
	Checks     []ValidationCheck `json:"checks"`
	NextOffset *int              `json:"next_offset,omitempty"`
}

func (s *WorkingCopies) inspectValidationLocked(ctx context.Context, id string, owner Binding, offset, limit int) (ValidationPage, error) {
	if offset < 0 || limit < 1 || limit > 64 {
		return ValidationPage{}, ErrLimit
	}
	set, _, err := s.changeSet(ctx, id, owner)
	if err != nil {
		return ValidationPage{}, err
	}
	if set.Validation.Version != 1 {
		return ValidationPage{}, ErrUnsupported
	}
	record, err := s.inspect(ctx, set.CopyID)
	if err != nil {
		return ValidationPage{}, err
	}
	evidence, digest, err := s.validationForCopy(ctx, record)
	if err != nil {
		return ValidationPage{}, err
	}
	if digest != set.Validation.ID || len(evidence.Checks) != set.Validation.CheckCount {
		return ValidationPage{}, ErrRevision
	}
	if offset > len(evidence.Checks) {
		return ValidationPage{}, ErrLimit
	}
	end := offset + min(limit, len(evidence.Checks)-offset)
	page := ValidationPage{Summary: set.Validation, Checks: append([]ValidationCheck{}, evidence.Checks[offset:end]...)}
	if end < len(evidence.Checks) {
		page.NextOffset = &end
	}
	return page, nil
}

type validationEvidence struct {
	Version             int               `json:"version"`
	Binding             Binding           `json:"binding"`
	InputManifestDigest string            `json:"input_manifest_digest"`
	Checks              []ValidationCheck `json:"checks"`
}

func (j *Journal) recordValidationProcess(ctx context.Context, request Request, input string) error {
	if request.Operation != "process_start" || !validCopyID(input) {
		return ErrProtocol
	}
	request.Token = ""
	data, _ := json.Marshal(request)
	tx, err := j.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_process_requests`).Scan(&count); err != nil {
		return err
	}
	if count >= 128 {
		return ErrLimit
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO execution_process_requests(id,request,input_manifest) SELECT id,?,? FROM execution_operations WHERE id=? AND digest=? AND state='claimed'`, data, input, request.OperationID, digest(request.Payload()))
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return tx.Commit()
}

// Called after owned processes and their journal writers have settled, before
// closing the private journal. No lost request/result is reconstructed by replay.
func (j *Journal) validationEvidence(ctx context.Context, binding Binding, input string) (validationEvidence, error) {
	evidence := validationEvidence{Version: 1, Binding: binding, InputManifestDigest: input, Checks: []ValidationCheck{}}
	rows, err := j.db.QueryContext(ctx, `SELECT r.request,r.input_manifest,o.digest,o.state,o.result FROM execution_process_requests r JOIN execution_operations o ON o.id=r.id ORDER BY r.id`)
	if err != nil {
		return evidence, err
	}
	defer rows.Close()
	for rows.Next() {
		var data, result []byte
		var manifest, requestDigest, state string
		if err = rows.Scan(&data, &manifest, &requestDigest, &state, &result); err != nil {
			return evidence, err
		}
		var request Request
		if len(evidence.Checks) >= 128 || decodeStrict(data, &request) != nil || request.Token != "" || request.Binding != binding || request.Operation != "process_start" || manifest != input || digest(request.Payload()) != requestDigest {
			return evidence, ErrRevision
		}
		spec, err := ParseProcessSpec(request.Arguments)
		if err != nil {
			return evidence, err
		}
		check := ValidationCheck{OperationID: request.OperationID, Command: spec, RequestDigest: requestDigest, ReceiptDigest: digest(result), State: "uncertain"}
		var process ProcessResult
		if state == "complete" && json.Unmarshal(result, &process) == nil && process.Handle != "" && process.Runtime != nil && process.Runtime.Settled && ProcessDescendantsSettled(process) {
			check.State, check.ExitCode = process.State, process.ExitCode
		} else if state == "rejected" {
			check.State = "rejected"
		}
		evidence.Checks = append(evidence.Checks, check)
	}
	return evidence, rows.Err()
}

func (s *WorkingCopies) recordValidation(ctx context.Context, record WorkingCopyRecord, evidence validationEvidence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, err := s.inspect(ctx, record.ID)
	if err != nil {
		return err
	}
	if stored.Source != record.Source || stored.WorkerRunID != evidence.Binding.Job || stored.ManifestDigest != evidence.InputManifestDigest || stored.Purpose != "test" {
		return ErrAuthority
	}
	data, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	if len(data) > maxChangeSetMetadataBytes {
		return ErrLimit
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO worker_validations(copy_id,digest,evidence) VALUES(?,?,?) ON CONFLICT(copy_id) DO NOTHING`, record.ID, digest(data), data)
	if err != nil {
		return err
	}
	var recorded string
	if err = s.db.QueryRowContext(ctx, `SELECT digest FROM worker_validations WHERE copy_id=?`, record.ID).Scan(&recorded); err != nil {
		return err
	}
	if recorded != digest(data) {
		return ErrConflict
	}
	return nil
}

func (s *WorkingCopies) validationSummary(ctx context.Context, record WorkingCopyRecord, base, result *WorkingCopyManifest) (ValidationSummary, error) {
	if record.Purpose != "test" {
		return ValidationSummary{}, nil
	}
	evidence, id, err := s.validationForCopy(ctx, record)
	if err != nil {
		return ValidationSummary{}, err
	}
	summary := ValidationSummary{Version: 1, ID: id, State: "passed", InputManifestDigest: evidence.InputManifestDigest, CheckCount: len(evidence.Checks)}
	if len(evidence.Checks) == 0 {
		summary.State = "not_run"
	}
	for _, check := range evidence.Checks {
		if check.State == "uncertain" {
			summary.State = "uncertain"
			break
		}
		if check.State != "exited" || check.ExitCode == nil || *check.ExitCode != 0 {
			summary.State = "failed"
		}
	}
	if !validationInputsMatch(base, result) {
		summary.State = "inputs_changed"
	}
	return summary, nil
}

func (s *WorkingCopies) validationForCopy(ctx context.Context, record WorkingCopyRecord) (validationEvidence, string, error) {
	var data []byte
	var id string
	if err := s.db.QueryRowContext(ctx, `SELECT digest,evidence FROM worker_validations WHERE copy_id=?`, record.ID).Scan(&id, &data); err != nil {
		return validationEvidence{}, "", err
	}
	var evidence validationEvidence
	if len(data) > maxChangeSetMetadataBytes || digest(data) != id || json.Unmarshal(data, &evidence) != nil || evidence.Version != 1 || evidence.Binding.Job != record.WorkerRunID || evidence.Binding.Actor != record.Source.Actor || evidence.Binding.Workspace != record.Source.Workspace || evidence.InputManifestDigest != record.ManifestDigest || len(evidence.Checks) > 128 {
		return evidence, "", ErrRevision
	}
	return evidence, id, nil
}

// Test commands may produce additional logs/cache files, but may not replace or
// remove any admitted input and still qualify the original candidate as tested.
func validationInputsMatch(base, result *WorkingCopyManifest) bool {
	actual := map[string]WorkingCopyEntry{}
	for _, entry := range result.Entries {
		actual[entry.Path] = entry
	}
	for _, entry := range base.Entries {
		want := entry
		want.Mode = workingCopyPrivateMode(want)
		if actual[entry.Path] != want {
			return false
		}
	}
	return true
}

func (s *WorkingCopies) checkValidationInputs(ctx context.Context, record WorkingCopyRecord) error {
	base, err := s.Manifest(ctx, record.ID, record.Source)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrHostUnavailable
	}
	root, err := s.root.OpenRoot(record.ID + "/content")
	if err != nil {
		return err
	}
	defer root.Close()
	actual, err := inventoryWorkingCopy(ctx, root, WorkingCopyRequest{Parent: record.Source, MaxBytes: s.policy.MaxCopyBytes}, s.policy)
	if err != nil {
		return err
	}
	if !validationInputsMatch(base, actual) {
		return fmt.Errorf("test candidate inputs changed before command dispatch: %w", ErrRevision)
	}
	return nil
}
