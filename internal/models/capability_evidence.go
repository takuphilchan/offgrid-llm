package models

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

const CapabilityEvidenceVersion = 1

var ErrCapabilityEvidenceVersion = errors.New("unsupported capability evidence version")

// Supplied by trusted runtime/installation measurements, never model output.
// An incomplete identity cannot establish current capability or qualification.
type CapabilityFingerprint struct {
	ModelSHA256      string `json:"model_sha256"`
	Quantization     string `json:"quantization"`
	RuntimeBuild     string `json:"runtime_build"`
	TemplateSHA256   string `json:"template_sha256"`
	Adapter          string `json:"adapter"`
	ContextWindow    int    `json:"context_window"`
	Backend          string `json:"backend"`
	HardwareID       string `json:"hardware_id"`
	ProjectorSHA256  string `json:"projector_sha256,omitempty"`
	PreprocessSHA256 string `json:"preprocess_sha256,omitempty"`
}

func evidenceDigest(s string) bool {
	data, err := hex.DecodeString(s)
	return err == nil && len(data) == sha256.Size && s == strings.ToLower(s)
}

func (f CapabilityFingerprint) ID() (string, error) {
	if !evidenceDigest(f.ModelSHA256) || !evidenceDigest(f.TemplateSHA256) || (f.ProjectorSHA256 != "" && !evidenceDigest(f.ProjectorSHA256)) || (f.PreprocessSHA256 != "" && !evidenceDigest(f.PreprocessSHA256)) || f.ContextWindow < 1 {
		return "", fmt.Errorf("incomplete capability fingerprint")
	}
	for _, v := range []string{f.Quantization, f.RuntimeBuild, f.Adapter, f.Backend, f.HardwareID} {
		if strings.TrimSpace(v) == "" || len(v) > 256 || !utf8.ValidString(v) || strings.ContainsAny(v, "\x00\r\n") {
			return "", fmt.Errorf("invalid capability fingerprint")
		}
	}
	encoded, _ := json.Marshal(f)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// Qualification is a separate evaluator-authored record, never inferred from a
// smoke pass. ArtifactID references the retained per-case evaluation evidence.
type WorkflowQualification struct {
	SuiteSHA256       string `json:"suite_sha256"`
	Platform          string `json:"platform"`
	ArtifactID        string `json:"artifact_id"`
	Cases             int    `json:"cases"`
	Repetitions       int    `json:"repetitions"`
	Verified          int    `json:"verified"`
	Unauthorized      int    `json:"unauthorized"`
	RepeatedUncertain int    `json:"repeated_uncertain"`
	FalseCompletion   int    `json:"false_completion"`
}

type CapabilityEvidence struct {
	Version       int                    `json:"version"`
	ModelID       string                 `json:"model_id"`
	CheckID       string                 `json:"check_id"`
	Fingerprint   CapabilityFingerprint  `json:"fingerprint"`
	Outcome       string                 `json:"outcome"` // passed, failed, unavailable; not permission
	Code          string                 `json:"code"`    // service-authored category, not raw model output
	RecordedAt    time.Time              `json:"recorded_at"`
	Qualification *WorkflowQualification `json:"qualification,omitempty"`
}

var evidenceCode = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,95}$`)

func (e CapabilityEvidence) validate() error {
	if e.Version != CapabilityEvidenceVersion {
		return ErrCapabilityEvidenceVersion
	}
	if strings.TrimSpace(e.ModelID) == "" || len(e.ModelID) > 256 || !utf8.ValidString(e.ModelID) || strings.ContainsAny(e.ModelID, "\x00\r\n") || !evidenceCode.MatchString(e.CheckID) || !evidenceCode.MatchString(e.Code) || e.RecordedAt.IsZero() {
		return fmt.Errorf("invalid capability evidence")
	}
	if e.Outcome != "passed" && e.Outcome != "failed" && e.Outcome != "unavailable" {
		return fmt.Errorf("invalid measured outcome")
	}
	if _, err := e.Fingerprint.ID(); err != nil {
		return err
	}
	if q := e.Qualification; q != nil {
		if !evidenceDigest(q.SuiteSHA256) || !evidenceDigest(q.ArtifactID) || !evidenceCode.MatchString(q.Platform) || q.Cases < 30 || q.Cases > 10000 || q.Repetitions != 3 || q.Verified < 0 || q.Verified > q.Cases*q.Repetitions || q.Unauthorized < 0 || q.RepeatedUncertain < 0 || q.FalseCompletion < 0 {
			return fmt.Errorf("invalid workflow evaluation evidence")
		}
		if e.Outcome == "passed" && (q.Verified*10 < q.Cases*q.Repetitions*9 || q.Unauthorized != 0 || q.RepeatedUncertain != 0 || q.FalseCompletion != 0) {
			return fmt.Errorf("workflow evaluation does not meet qualification gates")
		}
	}
	return nil
}

type CapabilityEvidenceView struct {
	State      string              `json:"state"`
	EvidenceID string              `json:"evidence_id,omitempty"`
	Evidence   *CapabilityEvidence `json:"evidence,omitempty"`
}

type CapabilityEvidenceTarget struct {
	Fingerprint CapabilityFingerprint
	SuiteSHA256 string
	Platform    string
}

// EvidenceStore is local measurement history, not an execution authorization
// cache. The service owns the data-root writer lock. Connections are short-lived
// so backups/restores do not race an idle connection or retained WAL.
type CapabilityEvidenceStore struct {
	mu        sync.Mutex
	directory string
}

func NewCapabilityEvidenceStore(dataDir string) *CapabilityEvidenceStore {
	if strings.TrimSpace(dataDir) == "" {
		return &CapabilityEvidenceStore{}
	}
	return &CapabilityEvidenceStore{directory: filepath.Join(dataDir, "model-capabilities")}
}

func (s *CapabilityEvidenceStore) open() (*sql.DB, error) {
	if s.directory == "" {
		return nil, fmt.Errorf("capability evidence data directory is required")
	}
	if err := os.MkdirAll(s.directory, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(s.directory, "evidence.sqlite")
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, fmt.Errorf("capability database must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	modeErr := f.Chmod(0600)
	closeErr := f.Close()
	if modeErr != nil {
		return nil, modeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	db, err := storage.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		db.Close()
		return nil, err
	}
	if version < 0 || version > CapabilityEvidenceVersion {
		db.Close()
		return nil, ErrCapabilityEvidenceVersion
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS capability_evidence (
sequence INTEGER PRIMARY KEY AUTOINCREMENT, id TEXT NOT NULL UNIQUE,
model_id TEXT NOT NULL, check_id TEXT NOT NULL, snapshot BLOB NOT NULL);
CREATE INDEX IF NOT EXISTS capability_evidence_lookup ON capability_evidence(model_id,check_id,sequence);
PRAGMA user_version=1;`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s *CapabilityEvidenceStore) Record(ctx context.Context, e CapabilityEvidence) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := e.validate(); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	id := hex.EncodeToString(digest[:])
	db, err := s.open()
	if err != nil {
		return "", err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `INSERT INTO capability_evidence(id,model_id,check_id,snapshot) VALUES(?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, e.ModelID, e.CheckID, encoded)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Latest retains old evidence even when it is stale. Operational failures do not
// become check_failed; no projection here grants permission to execute a task.
func (s *CapabilityEvidenceStore) Latest(ctx context.Context, modelID, checkID string, current CapabilityEvidenceTarget) (CapabilityEvidenceView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	view := CapabilityEvidenceView{State: "unmeasured"}
	if s.directory == "" {
		return view, fmt.Errorf("capability evidence data directory is required")
	}
	if err := ctx.Err(); err != nil {
		return view, err
	}
	if _, err := os.Stat(filepath.Join(s.directory, "evidence.sqlite")); os.IsNotExist(err) {
		return view, nil
	} else if err != nil {
		return view, err
	}
	db, err := s.open()
	if err != nil {
		return view, err
	}
	defer db.Close()
	var data []byte
	err = db.QueryRowContext(ctx, `SELECT id,snapshot FROM capability_evidence WHERE model_id=? AND check_id=? ORDER BY sequence DESC LIMIT 1`, modelID, checkID).Scan(&view.EvidenceID, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return view, nil
	}
	if err != nil {
		return view, err
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != view.EvidenceID {
		return CapabilityEvidenceView{State: "unmeasured"}, fmt.Errorf("capability evidence digest mismatch")
	}
	var e CapabilityEvidence
	if err = json.Unmarshal(data, &e); err != nil {
		return view, err
	}
	if err = e.validate(); err != nil {
		return view, err
	}
	if e.ModelID != modelID || e.CheckID != checkID {
		return view, fmt.Errorf("capability evidence identity mismatch")
	}
	view.Evidence = &e
	currentID, err := current.Fingerprint.ID()
	if err != nil {
		return view, nil
	}
	previousID, _ := e.Fingerprint.ID()
	if currentID != previousID {
		view.State = "stale"
		return view, nil
	}
	if q := e.Qualification; q != nil && (q.SuiteSHA256 != current.SuiteSHA256 || q.Platform != current.Platform) {
		view.State = "stale"
		return view, nil
	}
	switch e.Outcome {
	case "passed":
		view.State = "check_passed"
		if e.Qualification != nil {
			view.State = "workflow_qualified"
		}
	case "failed":
		view.State = "check_failed"
	}
	return view, nil
}
