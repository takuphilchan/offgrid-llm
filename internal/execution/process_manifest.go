package execution

import (
	"context"
	"encoding/json"
)

const processManifestSchema = `CREATE TABLE IF NOT EXISTS execution_process_manifests(operation TEXT PRIMARY KEY REFERENCES execution_operations(id),manifest BLOB NOT NULL,digest TEXT NOT NULL); CREATE TABLE IF NOT EXISTS execution_environments(session TEXT PRIMARY KEY,descriptor BLOB NOT NULL);`

type processManifest struct {
	Version       int                   `json:"version"`
	Binding       Binding               `json:"binding"`
	Operation     string                `json:"operation"`
	RequestDigest string                `json:"request_digest"`
	Program       string                `json:"program"`
	Directory     string                `json:"directory"`
	Environment   EnvironmentDescriptor `json:"environment"`
}

// This private journal stores selected paths and hashes, not environment values
// or a restartable command. Publication must succeed before native dispatch.
func (j *Journal) recordProcessManifest(ctx context.Context, request Request, directory, program string, environment EnvironmentDescriptor) error {
	if request.Operation != "process_start" || environment.Version != 1 || environment.Template != "minimal-host-v1" || !validCopyID(environment.Digest) {
		return ErrProtocol
	}
	manifest := processManifest{Version: 1, Binding: request.Binding, Operation: request.OperationID, RequestDigest: digest(request.Payload()), Program: program, Directory: directory, Environment: environment}
	raw, err := json.Marshal(manifest)
	if err != nil || len(raw) > MaxFrameBytes {
		return ErrLimit
	}
	owner := request.Binding
	owner.Job = ""
	descriptor, err := json.Marshal(struct {
		Owner       Binding               `json:"owner"`
		Environment EnvironmentDescriptor `json:"environment"`
	}{owner, environment})
	if err != nil {
		return err
	}
	tx, err := j.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO execution_environments(session,descriptor) VALUES(?,?) ON CONFLICT(session) DO NOTHING`, request.Binding.Session, descriptor); err != nil {
		return err
	}
	var savedDescriptor string
	if err = tx.QueryRowContext(ctx, `SELECT descriptor FROM execution_environments WHERE session=?`, request.Binding.Session).Scan(&savedDescriptor); err != nil {
		return err
	}
	if savedDescriptor != string(descriptor) {
		return ErrRevision
	}
	if err = requireCopyCheckpointWrite(tx.ExecContext(ctx, `INSERT INTO execution_process_manifests(operation,manifest,digest) SELECT id,?,? FROM execution_operations WHERE id=? AND digest=? AND state='claimed'`, raw, digest(raw), request.OperationID, manifest.RequestDigest)); err != nil {
		return err
	}
	var savedManifest, savedDigest string
	if err = tx.QueryRowContext(ctx, `SELECT manifest,digest FROM execution_process_manifests WHERE operation=?`, request.OperationID).Scan(&savedManifest, &savedDigest); err != nil {
		return err
	}
	if savedManifest != string(raw) || savedDigest != digest(raw) {
		return ErrRevision
	}
	// A trigger cannot change the admitted claim while publishing evidence.
	var state, requestDigest, finalDescriptor string
	if err = tx.QueryRowContext(ctx, `SELECT state,digest FROM execution_operations WHERE id=?`, request.OperationID).Scan(&state, &requestDigest); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT descriptor FROM execution_environments WHERE session=?`, request.Binding.Session).Scan(&finalDescriptor); err != nil {
		return err
	}
	if state != "claimed" || requestDigest != manifest.RequestDigest || finalDescriptor != savedDescriptor {
		return ErrRevision
	}
	return tx.Commit()
}
