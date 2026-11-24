package execution

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

var ErrHostProtectionSnapshotUnavailable = errors.New("host_protection_snapshot_unavailable")

type HostProtectionSnapshotUnavailableError struct {
	Reason string `json:"reason"`
}

func (e *HostProtectionSnapshotUnavailableError) Error() string {
	return "host_protection_snapshot_unavailable: " + e.Reason
}
func (e *HostProtectionSnapshotUnavailableError) Unwrap() error {
	return ErrHostProtectionSnapshotUnavailable
}

type HostProtectionInspection struct {
	Version int                              `json:"version"`
	Status  string                           `json:"status"`
	Records []HostProtectionInspectionRecord `json:"records"`
	Next    string                           `json:"next,omitempty"`
}
type HostProtectionBindingSummary struct {
	Actor     string `json:"actor"`
	Workspace string `json:"workspace"`
	Host      string `json:"host"`
	Session   string `json:"session"`
	Job       string `json:"job"`
	Revision  int    `json:"revision"`
}
type HostProtectionPathObservation struct {
	Path  string `json:"path"`
	State string `json:"state"`
}
type HostProtectionInspectionRecord struct {
	ID             string                         `json:"id"`
	Kind           string                         `json:"kind"`
	State          string                         `json:"state"`
	Backend        string                         `json:"backend,omitempty"`
	Binding        *HostProtectionBindingSummary  `json:"binding,omitempty"`
	Root           HostProtectionPathObservation  `json:"root"`
	Journal        *HostProtectionPathObservation `json:"journal,omitempty"`
	AdmissionScope string                         `json:"admission_scope"`
	BlockingReason string                         `json:"blocking_reason"`
}

// Inspect is a trusted local observation, never a grant or recovery action.
// It neither creates storage nor invokes the admission-wide identity fence:
// changed/missing registered paths are the information inspection must expose.
// Pages are deterministic within each read snapshot, not across later mutations.
func (s *HostProtectionRegistry) Inspect(ctx context.Context, after string, limit int) (result HostProtectionInspection, resultErr error) {
	return s.inspectRecords(ctx, after, limit, nil)
}

// The private startup validator shares the same existing-only read snapshot.
// No callback or unredacted record is exposed by the public inspection API.
func (s *HostProtectionRegistry) inspectRecords(ctx context.Context, after string, limit int, visit func(map[string]hostProtectionRecord) error) (result HostProtectionInspection, resultErr error) {
	if s == nil || !protectionPathValid(s.directory) || after != "" && !validCredential(after) || limit < 1 || limit > 64 {
		return result, ErrAuthority
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := s.checkAncestry(s.directory); err != nil {
		return result, err
	}
	parent := filepath.Dir(s.directory)
	marker := filepath.Join(parent, "host-protection.identity")
	_, rootErr := os.Lstat(s.directory)
	_, markerErr := os.Lstat(marker)
	_, lockErr := os.Lstat(filepath.Join(parent, ".offgrid-owner.lock"))
	if os.IsNotExist(rootErr) && os.IsNotExist(markerErr) && os.IsNotExist(lockErr) {
		return HostProtectionInspection{Version: 1, Status: "absent", Records: []HostProtectionInspectionRecord{}}, nil
	}
	if lockErr != nil {
		return result, errors.Join(ErrHostProtectionCorrupt, lockErr)
	}
	if err := CheckPrivatePath(parent); err != nil {
		return result, err
	}
	if err := protectionPrivateFile(filepath.Join(parent, ".offgrid-owner.lock"), false); err != nil {
		return result, err
	}
	owner, err := storage.AcquireExistingOwnership(parent)
	if errors.Is(err, storage.ErrWorkspaceInUse) {
		return result, ErrHostProtectionBusy
	}
	if err != nil {
		return result, errors.Join(ErrHostProtectionStorage, err)
	}
	defer func() { resultErr = errors.Join(resultErr, owner.Close()) }()
	_, rootErr = os.Lstat(s.directory)
	_, markerErr = os.Lstat(marker)
	if os.IsNotExist(rootErr) && os.IsNotExist(markerErr) {
		return HostProtectionInspection{Version: 1, Status: "absent", Records: []HostProtectionInspectionRecord{}}, nil
	}
	if rootErr != nil || markerErr != nil {
		return result, ErrHostProtectionCorrupt
	}
	rootInfo, err := os.Lstat(s.directory)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return result, ErrHostProtectionCorrupt
	}
	if err = CheckPrivatePath(s.directory); err != nil {
		return result, err
	}
	if err = protectionPrivateFile(marker, false); err != nil {
		return result, errors.Join(ErrHostProtectionCorrupt, err)
	}
	file, err := os.Open(marker)
	if err != nil {
		return result, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, 65))
	if err = errors.Join(readErr, file.Close()); err != nil || !validCredential(string(data)) {
		return result, errors.Join(ErrHostProtectionCorrupt, err)
	}
	path := filepath.Join(s.directory, "protection.sqlite")
	if err = protectionPrivateFile(path, false); err != nil {
		return result, errors.Join(ErrHostProtectionCorrupt, err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Size() == 0 {
		return result, ErrHostProtectionCorrupt
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err = protectionPrivateFile(path+suffix, true); err != nil {
			return result, errors.Join(ErrHostProtectionCorrupt, err)
		}
		if sidecar, e := os.Lstat(path + suffix); e == nil && sidecar.Size() != 0 && suffix != "-shm" {
			reason := "retained_wal"
			if suffix == "-journal" {
				reason = "retained_journal"
			}
			return result, &HostProtectionSnapshotUnavailableError{Reason: reason}
		} else if e != nil && !os.IsNotExist(e) {
			return result, errors.Join(ErrHostProtectionCorrupt, e)
		}
	}
	// The account lock excludes cooperating writers. Immutable mode must never
	// ignore retained WAL/journal state; the checks above deliberately refuse it.
	// Unlike OpenSQLite, this connection cannot create sidecars or set WAL mode.
	name := filepath.ToSlash(path)
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	query := url.Values{"mode": {"ro"}, "immutable": {"1"}, "_pragma": {"query_only(1)", "busy_timeout(0)"}}
	dsn := (&url.URL{Scheme: "file", Path: name, RawQuery: query.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return result, errors.Join(ErrHostProtectionCorrupt, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, errors.Join(ErrHostProtectionCorrupt, err)
	}
	defer tx.Rollback()
	var check string
	if err = tx.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		return result, errors.Join(ErrHostProtectionCorrupt, err)
	}
	var version, count int
	var identity string
	if err = tx.QueryRowContext(ctx, "SELECT version,identity,(SELECT COUNT(*) FROM host_protection_meta) FROM host_protection_meta WHERE singleton=1").Scan(&version, &identity, &count); err != nil || count != 1 || version != 1 || identity != string(data) {
		return result, errors.Join(ErrHostProtectionCorrupt, err)
	}
	store := &hostProtectionTransaction{tx: tx, records: map[string]hostProtectionRecord{}}
	if err = store.read(ctx); err != nil {
		return result, err
	}
	if len(store.records) == 0 {
		return result, ErrHostProtectionCorrupt
	}
	if visit != nil {
		if err = visit(store.records); err != nil {
			return result, err
		}
	}
	ids := make([]string, 0, len(store.records))
	for id := range store.records {
		if id > after {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	result = HostProtectionInspection{Version: 1, Status: "ready", Records: []HostProtectionInspectionRecord{}}
	if len(ids) > limit {
		result.Next = ids[limit-1]
		ids = ids[:limit]
	}
	for _, id := range ids {
		if err = ctx.Err(); err != nil {
			return HostProtectionInspection{}, err
		}
		result.Records = append(result.Records, inspectProtectionRecord(store.records[id]))
	}
	return result, nil
}

func observeProtectionPath(location hostProtectionLocation) HostProtectionPathObservation {
	result := HostProtectionPathObservation{Path: location.Path, State: "unavailable"}
	if _, err := os.Lstat(location.Anchor); os.IsNotExist(err) {
		result.State = "missing"
		return result
	} else if err != nil {
		return result
	}
	identity, err := folderIdentity(location.Anchor)
	if err != nil {
		return result
	}
	if identity != location.Identity {
		result.State = "identity_changed"
		return result
	}
	canonical, err := canonicalPrivateLocation(location.Anchor)
	if err != nil {
		return result
	}
	if !protectionSamePath(canonical, location.Anchor) {
		result.State = "path_changed"
		return result
	}
	if location.Suffix == "" {
		result.State = "matched"
		return result
	}
	current, err := locateHostProtection(location.Path)
	if err != nil {
		return result
	}
	if !protectionSamePath(current.Path, location.Path) {
		result.State = "path_changed"
		return result
	}
	result.State = "pending_missing"
	if current.Suffix == "" {
		result.State = "pending_created"
	}
	return result
}

func inspectProtectionRecord(row hostProtectionRecord) HostProtectionInspectionRecord {
	result := HostProtectionInspectionRecord{ID: row.ID, Kind: row.Kind, State: row.State, Backend: row.Backend, Root: observeProtectionPath(row.Root), AdmissionScope: "overlap"}
	if b := row.Binding; b != nil {
		result.Binding = &HostProtectionBindingSummary{Actor: b.Actor, Workspace: b.Workspace, Host: b.Host, Session: b.Session, Job: b.Job, Revision: b.Revision}
	}
	if row.Journal != nil {
		observed := observeProtectionPath(*row.Journal)
		result.Journal = &observed
	}
	if row.State == "released" {
		result.AdmissionScope = "none"
		result.BlockingReason = "none"
		return result
	}
	for _, entry := range []struct {
		role        string
		observation *HostProtectionPathObservation
	}{{"root", &result.Root}, {"journal", result.Journal}} {
		if entry.observation == nil {
			continue
		}
		switch entry.observation.State {
		case "matched", "pending_missing", "pending_created":
		default:
			result.AdmissionScope = "all"
			result.BlockingReason = entry.role + "_" + entry.observation.State
			return result
		}
	}
	switch row.Kind {
	case "execution":
		result.BlockingReason = "execution_cleanup_unconfirmed"
	case "browser":
		result.BlockingReason = "protected_browser_storage"
		if row.State == "pending" {
			result.BlockingReason = "pending_browser_finalization"
		}
	}
	return result
}
