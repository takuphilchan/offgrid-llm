package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const integrationSchema = `CREATE TABLE IF NOT EXISTS worker_integrations(
 id TEXT PRIMARY KEY,set_id TEXT NOT NULL UNIQUE REFERENCES worker_change_sets(id),
 request_digest TEXT NOT NULL,request BLOB NOT NULL,state TEXT NOT NULL,created_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS worker_integration_files(
 integration_id TEXT NOT NULL REFERENCES worker_integrations(id),ordinal INTEGER NOT NULL,
 state TEXT NOT NULL,step BLOB NOT NULL,receipt BLOB,
 PRIMARY KEY(integration_id,ordinal));
 CREATE TABLE IF NOT EXISTS worker_integration_failures(
 integration_id TEXT PRIMARY KEY REFERENCES worker_integrations(id),error TEXT NOT NULL,path TEXT NOT NULL);`

type IntegrateChangeSetRequest struct {
	RequestID          string  `json:"request_id"`
	Source             Binding `json:"source"`
	ChangeSetID        string  `json:"change_set_id"`
	OperationsDigest   string  `json:"operations_digest"`
	InstructionVersion uint64  `json:"instruction_version"`
	ControlEpoch       uint64  `json:"control_epoch"`
	MaxReservedBytes   int64   `json:"max_reserved_bytes"`
}

type IntegrationFileReceipt struct {
	Ordinal         int    `json:"ordinal"`
	Path            string `json:"path"`
	Operation       string `json:"operation"`
	State           string `json:"state"`
	Dispatched      bool   `json:"dispatched"`
	Displaced       string `json:"displaced,omitempty"`
	DisplacedDigest string `json:"displaced_digest,omitempty"`
	ResultDigest    string `json:"result_digest,omitempty"`
	Error           string `json:"error,omitempty"`
}

type IntegrationPage struct {
	Error        string                   `json:"error,omitempty"`
	ConflictPath string                   `json:"conflict_path,omitempty"`
	ID           string                   `json:"id"`
	ChangeSetID  string                   `json:"change_set_id"`
	State        string                   `json:"state"`
	Total        int                      `json:"total"`
	Integrated   int                      `json:"integrated"`
	Files        []IntegrationFileReceipt `json:"files"`
	NextOffset   *int                     `json:"next_offset,omitempty"`
}

type integrationStep struct {
	Ordinal int        `json:"ordinal"`
	Change  FileChange `json:"change"`
}

func integrationIdentity(req IntegrateChangeSetRequest) (string, string) {
	key, _ := json.Marshal(struct {
		Source    Binding
		RequestID string
	}{req.Source, req.RequestID})
	payload, _ := json.Marshal(req)
	return digest(key), digest(payload)
}

const IntegrateTool = "workspace_integrate_changes"

func IntegrationRequestID(req IntegrateChangeSetRequest) string {
	id, _ := integrationIdentity(req)
	return id
}

// The trusted supervisor supplies current versions and live authority. This
// command is never dispatched by GET/reconnect. Once recorded, retries only
// inspect the original attempt, including after process interruption.
func (s *WorkingCopies) IntegrateChangeSet(ctx context.Context, source *Session, req IntegrateChangeSetRequest) (IntegrationPage, error) {
	return s.integrateChangeSet(ctx, source, req, nil)
}

// Hooks only inject failures in native temporary-filesystem tests; they are not
// serialized, exposed to workers, or read from repository configuration.
type integrationHooks struct {
	beforeStep        func(int)
	afterDispatch     func(int) error
	beforeReplacement func() error
}

func (s *WorkingCopies) integrateChangeSet(ctx context.Context, source *Session, req IntegrateChangeSetRequest, hooks *integrationHooks) (page IntegrationPage, resultErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || source == nil {
		return page, ErrHostUnavailable
	}
	if !validIdentity(req.RequestID) || !validCopyID(req.ChangeSetID) || !validCopyID(req.OperationsDigest) || req.InstructionVersion == 0 || req.MaxReservedBytes < 1 {
		return page, ErrProtocol
	}
	config, err := source.Snapshot(ctx, req.Source)
	if err != nil {
		return page, err
	}
	if !config.Permissions.Write || !config.Permissions.Read {
		return page, ErrAuthority
	}
	if config.Backend != "local" {
		return page, ErrUnsupported
	}
	target, err := pinIntegrationSource(source, ctx, req.Source)
	if err != nil {
		return page, err
	}
	defer target.Close()
	id, payloadDigest := integrationIdentity(req)
	var previous string
	if err := s.db.QueryRowContext(ctx, `SELECT request_digest FROM worker_integrations WHERE id=?`, id).Scan(&previous); err == nil {
		if previous != payloadDigest {
			return page, ErrConflict
		}
		return s.inspectIntegrationLocked(ctx, id, req.Source, 0, 64)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return page, err
	}
	set, changes, err := s.changeSet(ctx, req.ChangeSetID, req.Source)
	if err != nil {
		return page, err
	}
	if set.State != "ready" || set.IntegrationStarted || set.OperationsDigest != req.OperationsDigest || set.InstructionVersion != req.InstructionVersion || set.ControlEpoch != req.ControlEpoch || set.Validation.State != "passed" {
		return page, ErrRevision
	}
	if err = PrivateStateOutsideRoot(config.Root, s.directory); err != nil {
		return page, err
	}
	if err = PrivateStateOutsideRoot(s.directory, config.Root); err != nil {
		return page, err
	}
	steps, reserved, err := integrationSteps(changes)
	if err != nil {
		return page, err
	}
	if err = s.reserveIntegration(ctx, req, id, payloadDigest, set, steps, reserved); err != nil {
		return page, err
	}
	// From this point a failure must have a retained native attempt. No handler
	// may retry the loop merely because its result could not be persisted.
	defer func() {
		if resultErr != nil {
			_ = s.finishIntegration(context.Background(), id, req.ChangeSetID, resultErr)
		}
		if saved, e := s.inspectIntegrationLocked(context.Background(), id, req.Source, 0, 64); e == nil {
			page = saved
		}
	}()
	operationCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-source.revokedSignal:
			cancel()
		case <-operationCtx.Done():
		}
	}()
	if err = s.preflightIntegration(operationCtx, target, set); err != nil {
		return page, err
	}
	artifactPath := set.CopyID + "/artifacts/integration-" + id
	if err = s.root.Mkdir(artifactPath, 0700); err != nil {
		return page, err
	}
	private, err := s.root.OpenRoot(artifactPath)
	if err != nil {
		return page, err
	}
	defer private.Close()
	// Reject cross-volume/unsupported directory backends before touching the
	// original tree, even if this candidate's first operation is a creation.
	if hooks != nil && hooks.beforeReplacement != nil {
		if err = hooks.beforeReplacement(); err != nil {
			return page, err
		}
	}
	targetDir, err := pinReplacementDirectory(target, ".")
	if err != nil {
		return page, err
	}
	defer targetDir.close()
	privateDir, err := pinReplacementDirectory(private, ".")
	if err != nil {
		return page, err
	}
	defer privateDir.close()
	if err = qualifyReplacementVolume(targetDir, privateDir); err != nil {
		return page, err
	}
	for _, step := range steps {
		if hooks != nil && hooks.beforeStep != nil {
			hooks.beforeStep(step.Ordinal)
		}
		if _, err = source.Snapshot(operationCtx, req.Source); err != nil {
			return page, err
		}
		directory := fmt.Sprintf("%06d", step.Ordinal)
		if err = private.Mkdir(directory, 0700); err != nil {
			return page, err
		}
		stage, err := private.OpenRoot(directory)
		if err != nil {
			return page, err
		}
		err = s.stageIntegrationStep(operationCtx, set, stage, step)
		if err != nil {
			stage.Close()
			return page, err
		}
		if _, err = source.Snapshot(operationCtx, req.Source); err != nil {
			stage.Close()
			return page, err
		}
		if err = s.transitionIntegrationFile(operationCtx, id, step.Ordinal, "pending", "dispatching", nil); err != nil {
			stage.Close()
			return page, err
		}
		receipt, applyErr := applyIntegrationStep(operationCtx, target, stage, step, s.policy.MaxCopyBytes)
		stage.Close()
		if receipt.Dispatched && hooks != nil && hooks.afterDispatch != nil {
			if err = hooks.afterDispatch(step.Ordinal); err != nil {
				return page, err
			}
		}
		data, _ := json.Marshal(receipt)
		if err = s.transitionIntegrationFile(context.Background(), id, step.Ordinal, "dispatching", receipt.State, data); err != nil {
			return page, err
		}
		if applyErr != nil {
			return page, applyErr
		}
	}
	if _, err = s.measureLocked(context.Background(), set.CopyID, req.Source); err != nil {
		return page, err
	}
	if err = s.finishIntegration(context.Background(), id, req.ChangeSetID, nil); err != nil {
		return page, err
	}
	return s.inspectIntegrationLocked(ctx, id, req.Source, 0, 64)
}

func (s *WorkingCopies) transitionIntegrationFile(ctx context.Context, id string, ordinal int, from, to string, receipt []byte) error {
	result, err := s.db.ExecContext(ctx, `UPDATE worker_integration_files SET state=?,receipt=? WHERE integration_id=? AND ordinal=? AND state=?`, to, receipt, id, ordinal, from)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}

func pinIntegrationSource(source *Session, ctx context.Context, binding Binding) (*os.Root, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	if err := source.authorize(ctx, binding); err != nil {
		return nil, err
	}
	return source.root.OpenRoot(".")
}

func (s *WorkingCopies) InspectIntegration(ctx context.Context, id string, owner Binding, offset, limit int) (IntegrationPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return IntegrationPage{}, ErrHostUnavailable
	}
	return s.inspectIntegrationLocked(ctx, id, owner, offset, limit)
}

func (s *WorkingCopies) inspectIntegrationLocked(ctx context.Context, id string, owner Binding, offset, limit int) (IntegrationPage, error) {
	page := IntegrationPage{ID: id, Files: []IntegrationFileReceipt{}}
	if !validCopyID(id) || offset < 0 || limit < 1 || limit > 64 {
		return page, ErrLimit
	}
	var request []byte
	if err := s.db.QueryRowContext(ctx, `SELECT set_id,state,request FROM worker_integrations WHERE id=?`, id).Scan(&page.ChangeSetID, &page.State, &request); err != nil {
		return page, err
	}
	var req IntegrateChangeSetRequest
	if json.Unmarshal(request, &req) != nil || req.Source != owner {
		return IntegrationPage{}, ErrAuthority
	}
	if err := s.db.QueryRowContext(ctx, `SELECT error,path FROM worker_integration_failures WHERE integration_id=?`, id).Scan(&page.Error, &page.ConflictPath); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return page, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(state='integrated'),0) FROM worker_integration_files WHERE integration_id=?`, id).Scan(&page.Total, &page.Integrated); err != nil {
		return page, err
	}
	if offset > page.Total {
		return page, ErrLimit
	}
	rows, err := s.db.QueryContext(ctx, `SELECT ordinal,state,step,receipt FROM worker_integration_files WHERE integration_id=? ORDER BY ordinal LIMIT ? OFFSET ?`, id, limit, offset)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var file IntegrationFileReceipt
		var stepData, receipt []byte
		if err = rows.Scan(&file.Ordinal, &file.State, &stepData, &receipt); err != nil {
			return page, err
		}
		var step integrationStep
		if json.Unmarshal(stepData, &step) != nil {
			return page, ErrRevision
		}
		file.Path, file.Operation = step.Change.Path, step.Change.Kind
		if len(receipt) > 0 {
			state := file.State
			if json.Unmarshal(receipt, &file) != nil {
				return page, ErrRevision
			}
			file.State = state
		}
		if len(receipt) == 0 && (file.State == "dispatching" || file.State == "uncertain") {
			file.Dispatched = true
		}
		page.Files = append(page.Files, file)
	}
	if offset+len(page.Files) < page.Total {
		next := offset + len(page.Files)
		page.NextOffset = &next
	}
	return page, rows.Err()
}

func (s *WorkingCopies) finishIntegration(ctx context.Context, id, setID string, cause error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing string
	if err = tx.QueryRow(`SELECT state FROM worker_integrations WHERE id=?`, id).Scan(&existing); err != nil {
		return err
	}
	if existing == "integrated" {
		return nil
	}
	var total, done, unknown, pending int
	if err = tx.QueryRow(`SELECT COUNT(*),COALESCE(SUM(state='integrated'),0),COALESCE(SUM(state IN ('dispatching','uncertain')),0),COALESCE(SUM(state='pending'),0) FROM worker_integration_files WHERE integration_id=?`, id).Scan(&total, &done, &unknown, &pending); err != nil {
		return err
	}
	state := "integrated"
	if cause != nil || done != total {
		state = "uncertain"
		if pending == total && total > 0 && errors.Is(cause, ErrUnsupported) {
			// The backend rejected replacement before any file dispatch. Keep
			// the sealed proposal reviewable; do not invent uncertain mutations.
			state = "abandoned"
		}
		if unknown == 0 && (errors.Is(cause, ErrConflict) || errors.Is(cause, ErrRevision)) {
			state = "conflict"
		}
		if done > 0 && unknown == 0 {
			state = "partial"
		}
	}
	if err = integrationRowChanged(tx.Exec(`UPDATE worker_integrations SET state=? WHERE id=?`, state, id)); err != nil {
		return err
	}
	if err = integrationRowChanged(tx.Exec(`UPDATE worker_change_sets SET state=? WHERE id=?`, state, setID)); err != nil {
		return err
	}
	if cause != nil {
		conflictPath := ""
		var conflict *integrationConflict
		if errors.As(cause, &conflict) {
			conflictPath = conflict.path
		}
		if _, err = tx.Exec(`INSERT INTO worker_integration_failures(integration_id,error,path) VALUES(?,?,?) ON CONFLICT(integration_id) DO UPDATE SET error=excluded.error,path=excluded.path`, id, errorCode(cause), conflictPath); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func integrationRowChanged(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}

type integrationConflict struct{ path string }

func (e *integrationConflict) Error() string { return "integration source changed at " + e.path }
func (e *integrationConflict) Unwrap() error { return ErrConflict }
