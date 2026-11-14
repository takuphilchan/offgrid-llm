package execution

import (
	"context"
	"encoding/json"
	"time"
)

const changeSetSchema = `CREATE TABLE IF NOT EXISTS worker_change_sets(
 id TEXT PRIMARY KEY, copy_id TEXT NOT NULL REFERENCES worker_copies(id), request_digest TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('draft','sealed','validating','ready','applying','integrated','conflict','validation_failed','partial','uncertain','abandoned')),
 header BLOB NOT NULL, operations BLOB NOT NULL, result_manifest BLOB NOT NULL);
 CREATE INDEX IF NOT EXISTS worker_change_sets_copy ON worker_change_sets(copy_id);
 CREATE TABLE IF NOT EXISTS worker_change_blobs(set_id TEXT NOT NULL REFERENCES worker_change_sets(id),hash TEXT NOT NULL,chunks BLOB NOT NULL,PRIMARY KEY(set_id,hash));
 CREATE TABLE IF NOT EXISTS worker_change_bases(set_id TEXT PRIMARY KEY REFERENCES worker_change_sets(id),manifest BLOB NOT NULL);`

const maxChangeSetMetadataBytes = 16 << 20

type SealChangeSetRequest struct {
	MaxReservedBytes   int64   `json:"max_reserved_bytes,omitempty"`
	RequestID          string  `json:"request_id"`
	CopyID             string  `json:"copy_id"`
	Source             Binding `json:"source"`
	InstructionVersion uint64  `json:"instruction_version"`
	ControlEpoch       uint64  `json:"control_epoch"`
}

type ChangeSet struct {
	Version              int               `json:"version"`
	ID                   string            `json:"id"`
	CopyID               string            `json:"copy_id"`
	Source               Binding           `json:"source"`
	WorkerRunID          string            `json:"worker_run_id"`
	InstructionVersion   uint64            `json:"instruction_version"`
	ControlEpoch         uint64            `json:"control_epoch"`
	State                string            `json:"state"`
	BaseManifestDigest   string            `json:"base_manifest_digest"`
	ResultManifestDigest string            `json:"result_manifest_digest"`
	OperationsDigest     string            `json:"operations_digest"`
	OperationCount       int               `json:"operation_count"`
	BlobBytes            int64             `json:"blob_bytes"`
	CreatedAt            time.Time         `json:"created_at"`
	IntegrationStarted   bool              `json:"integration_started,omitempty"`
	Validation           ValidationSummary `json:"validation,omitzero"`
}

type FileChange struct {
	Path         string            `json:"path"`
	Kind         string            `json:"kind"`
	Base         *WorkingCopyEntry `json:"base,omitempty"`
	Result       *WorkingCopyEntry `json:"result,omitempty"`
	ChunksDigest string            `json:"chunks_digest,omitempty"`
}

type ChangeSetPage struct {
	ChangeSet  ChangeSet    `json:"change_set"`
	Operations []FileChange `json:"operations"`
	NextOffset *int         `json:"next_offset,omitempty"`
}

func changeSetIdentity(req SealChangeSetRequest) (string, string) {
	identity, _ := json.Marshal(struct {
		Source            Binding
		CopyID, RequestID string
	}{req.Source, req.CopyID, req.RequestID})
	payload, _ := json.Marshal(req)
	return digest(identity), digest(payload)
}

func (s *WorkingCopies) changeSet(ctx context.Context, id string, owner Binding) (ChangeSet, []FileChange, error) {
	var header []byte
	var state string
	if !validCopyID(id) {
		return ChangeSet{}, nil, ErrAuthority
	}
	if err := s.db.QueryRowContext(ctx, `SELECT state,header FROM worker_change_sets WHERE id=?`, id).Scan(&state, &header); err != nil {
		return ChangeSet{}, nil, err
	}
	var set ChangeSet
	if err := json.Unmarshal(header, &set); err != nil {
		return ChangeSet{}, nil, err
	}
	if set.Source != owner {
		return ChangeSet{}, nil, ErrAuthority
	}
	if set.ID != id || !validCopyID(set.CopyID) {
		return ChangeSet{}, nil, ErrRevision
	}
	set.State = state
	if cached := s.changeCache; cached != nil && cached.id == id && cached.digest == set.OperationsDigest {
		return set, cached.operations, nil
	}
	var operations []byte
	if err := s.db.QueryRowContext(ctx, `SELECT operations FROM worker_change_sets WHERE id=?`, id).Scan(&operations); err != nil {
		return ChangeSet{}, nil, err
	}
	if len(operations) > maxChangeSetMetadataBytes || digest(operations) != set.OperationsDigest {
		return ChangeSet{}, nil, ErrRevision
	}
	var changes []FileChange
	if err := json.Unmarshal(operations, &changes); err != nil || len(changes) != set.OperationCount {
		return ChangeSet{}, nil, ErrRevision
	}
	s.changeCache = &changeSetCache{id: id, digest: set.OperationsDigest, operations: changes}
	return set, changes, nil
}

// Inspection is bounded and cannot seal, validate or apply a change set.
func (s *WorkingCopies) InspectChangeSet(ctx context.Context, id string, owner Binding, offset, limit int) (ChangeSetPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inspectChangeSetLocked(ctx, id, owner, offset, limit)
}

func (s *WorkingCopies) inspectChangeSetLocked(ctx context.Context, id string, owner Binding, offset, limit int) (ChangeSetPage, error) {
	if offset < 0 || limit < 1 || limit > 64 {
		return ChangeSetPage{}, ErrLimit
	}
	set, changes, err := s.changeSet(ctx, id, owner)
	if err != nil {
		return ChangeSetPage{}, err
	}
	if offset > len(changes) {
		return ChangeSetPage{}, ErrLimit
	}
	end := offset + min(limit, len(changes)-offset)
	page := ChangeSetPage{ChangeSet: set, Operations: append([]FileChange{}, changes[offset:end]...)}
	for i := range page.Operations {
		if base := page.Operations[i].Base; base != nil {
			value := *base
			page.Operations[i].Base = &value
		}
		if result := page.Operations[i].Result; result != nil {
			value := *result
			page.Operations[i].Result = &value
		}
	}
	if end < len(changes) {
		page.NextOffset = &end
	}
	return page, nil
}

// Abandon preserves evidence until an explicit working-copy release. It cannot
// undo effects, and therefore cannot abandon applying/partially applied sets.
func (s *WorkingCopies) AbandonChangeSet(ctx context.Context, id string, owner Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, _, err := s.changeSet(ctx, id, owner)
	if err != nil {
		return err
	}
	if set.IntegrationStarted {
		return ErrConflict
	}
	switch set.State {
	case "draft", "sealed", "ready", "conflict", "validation_failed", "uncertain", "abandoned":
	default:
		return ErrConflict
	}
	_, err = s.db.ExecContext(ctx, `UPDATE worker_change_sets SET state='abandoned' WHERE id=?`, id)
	return err
}

type changeSetCache struct {
	id, digest string
	operations []FileChange
}
