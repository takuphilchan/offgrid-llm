package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

type WorkingCopyPolicy struct {
	Version       int   `json:"version"`
	MaxTotalBytes int64 `json:"max_total_bytes"`
	MaxCopyBytes  int64 `json:"max_copy_bytes"`
	MaxEntries    int   `json:"max_entries"`
	MaxCopies     int   `json:"max_copies"`
}

func DefaultWorkingCopyPolicy() WorkingCopyPolicy {
	return WorkingCopyPolicy{Version: 1, MaxTotalBytes: 2 << 30, MaxCopyBytes: 512 << 20, MaxEntries: 10000, MaxCopies: 32}
}

type WorkingCopyRequest struct {
	RequestID          string                 `json:"request_id"`
	Parent             Binding                `json:"parent"`
	WorkerRunID        string                 `json:"worker_run_id"`
	InstructionVersion uint64                 `json:"instruction_version"`
	ControlEpoch       uint64                 `json:"control_epoch"`
	Permissions        Permissions            `json:"permissions"`
	MaxBytes           int64                  `json:"max_bytes"`
	RequiredPaths      []string               `json:"required_paths,omitempty"`
	Prerequisites      []ChangeSetReference   `json:"prerequisites,omitempty"`
	Integrated         []IntegrationReference `json:"integrated,omitempty"`
	Purpose            string                 `json:"purpose,omitempty"`
}

type WorkingCopyRecord struct {
	rootInfo              os.FileInfo
	ID                    string                 `json:"id"`
	State                 string                 `json:"state"`
	Purpose               string                 `json:"purpose,omitempty"`
	Source                Binding                `json:"source"`
	WorkerRunID           string                 `json:"worker_run_id"`
	ReservedBytes         int64                  `json:"reserved_bytes"`
	InputBytes            int64                  `json:"input_bytes"`
	ManifestDigest        string                 `json:"manifest_digest,omitempty"`
	MeasuredBytes         int64                  `json:"measured_bytes"`
	UsageMeasured         bool                   `json:"usage_measured"`
	ArtifactReservedBytes int64                  `json:"artifact_reserved_bytes,omitempty"`
	ContentBytes          int64                  `json:"content_bytes,omitempty"`
	Environment           *EnvironmentDescriptor `json:"environment,omitempty"`
}

// WorkingCopies owns host-private preparation artifacts and logical-byte
// reservations, not task execution or authority. The reservation is not an OS
// filesystem quota for arbitrary trusted native processes.
type WorkingCopies struct {
	mu            sync.Mutex
	directory     string
	root          *os.Root
	db            *sql.DB
	owner         *storage.Ownership
	policy        WorkingCopyPolicy
	closed        bool
	changeCache   *changeSetCache
	blobCache     *changeBlobCache
	recoveryCache *recoveryBlobIndex
}

func OpenWorkingCopies(directory string, policy WorkingCopyPolicy) (*WorkingCopies, error) {
	if policy.MaxCopies == 0 {
		policy.MaxCopies = 32
	}
	if policy.Version != 1 || policy.MaxCopyBytes < 1 || policy.MaxCopyBytes == math.MaxInt64 || policy.MaxTotalBytes < policy.MaxCopyBytes || policy.MaxTotalBytes > math.MaxInt64/int64(max(policy.MaxCopies, 1)) || policy.MaxEntries < 1 || policy.MaxEntries > 100000 || policy.MaxCopies < 1 || policy.MaxCopies > 128 {
		return nil, ErrLimit
	}
	if err := EnsurePrivateDirectory(directory); err != nil {
		return nil, err
	}
	directory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, err
	}
	owner, err := storage.AcquireOwnership(directory)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*WorkingCopies, error) { owner.Close(); return nil, err }
	dbPath := filepath.Join(directory, "worker-copies.sqlite")
	if info, e := os.Lstat(dbPath); e == nil && !info.Mode().IsRegular() {
		return fail(ErrPath)
	} else if e != nil && !os.IsNotExist(e) {
		return fail(e)
	}
	db, err := storage.OpenSQLite(dbPath)
	if err != nil {
		return fail(err)
	}
	var version int
	if err = db.QueryRow(`PRAGMA user_version`).Scan(&version); err == nil && (version < 0 || version > 12) {
		err = ErrUnsupported
	}
	if err == nil {
		ownershipUpgrade := ""
		if version == 11 {
			ownershipUpgrade = workingCopyOwnershipV12
		}
		_, err = db.Exec(`BEGIN; CREATE TABLE IF NOT EXISTS worker_copies(id TEXT PRIMARY KEY,digest TEXT NOT NULL,state TEXT NOT NULL CHECK(state IN ('preparing','ready','uncertain','releasing','released')),reserved_bytes INTEGER NOT NULL CHECK(reserved_bytes>=0),record BLOB NOT NULL,manifest BLOB); ` + changeSetSchema + integrationSchema + workingCopyOwnershipSchema + ownershipUpgrade + ` CREATE TABLE IF NOT EXISTS worker_validations(copy_id TEXT PRIMARY KEY REFERENCES worker_copies(id),digest TEXT NOT NULL,evidence BLOB NOT NULL); PRAGMA user_version=12; COMMIT;`)
	}
	var integrity string
	if err == nil {
		err = db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity)
		if err == nil && integrity != "ok" {
			err = ErrUnsupported
		}
	}
	if err == nil {
		_, err = db.Exec(`UPDATE worker_copies SET state='uncertain' WHERE state IN ('preparing','releasing')`)
	}
	if err == nil {
		_, err = db.Exec(`UPDATE worker_change_sets SET state='uncertain' WHERE state IN ('draft','validating','applying')`)
	}
	if err == nil {
		_, err = db.Exec(`BEGIN; UPDATE worker_integration_files SET state='uncertain' WHERE state='dispatching'; UPDATE worker_integrations SET state='uncertain' WHERE state='applying'; COMMIT;`)
	}
	if err != nil {
		db.Close()
		return fail(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		db.Close()
		return fail(err)
	}
	return &WorkingCopies{directory: directory, root: root, db: db, owner: owner, policy: policy}, nil
}

func (s *WorkingCopies) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return errors.Join(s.root.Close(), s.db.Close(), s.owner.Close())
}

func (s *WorkingCopies) inspect(ctx context.Context, id string) (WorkingCopyRecord, error) {
	var r WorkingCopyRecord
	var data []byte
	var state string
	var reserved int64
	if err := s.db.QueryRowContext(ctx, `SELECT state,record,reserved_bytes FROM worker_copies WHERE id=?`, id).Scan(&state, &data, &reserved); err != nil {
		return r, err
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return r, err
	}
	r.State = state
	r.ReservedBytes = reserved
	return r, nil
}

func (s *WorkingCopies) Inspect(ctx context.Context, id string, owner Binding) (WorkingCopyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.inspect(ctx, id)
	if err == nil && r.Source != owner {
		return WorkingCopyRecord{}, ErrAuthority
	}
	return r, err
}

func (s *WorkingCopies) Manifest(ctx context.Context, id string, owner Binding) (*WorkingCopyManifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.inspect(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.Source != owner {
		return nil, ErrAuthority
	}
	var data []byte
	if err = s.db.QueryRowContext(ctx, `SELECT manifest FROM worker_copies WHERE id=?`, id).Scan(&data); err != nil {
		return nil, err
	}
	if digest(data) != r.ManifestDigest {
		return nil, ErrRevision
	}
	var m WorkingCopyManifest
	if err = json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func validCopyID(id string) bool {
	if len(id) != 64 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			if c < 'a' || c > 'f' {
				return false
			}
		}
	}
	return true
}
