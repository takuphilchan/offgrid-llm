package integrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/platform"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

type Connection struct {
	Version          int      `json:"version"`
	ModelID          string   `json:"model_id,omitempty"`
	BaseURL          string   `json:"base_url,omitempty"`
	ContractRevision string   `json:"contract_revision,omitempty"`
	Smoke            Evidence `json:"smoke"`
}

type ConnectionStore struct {
	Directory string
	Scope     string
}

func validID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func (s *ConnectionStore) path(id string, o HostOptions) (string, error) {
	if !validID(id) {
		return "", fmt.Errorf("invalid integration ID")
	}
	// Hash the credential and host-profile identities with the service: evidence and a
	// default for one account/service must not select another account's model.
	key := contentDigest([]byte(o.ServerURL + "\x00" + s.Scope + "\x00" + o.APIKey))
	return filepath.Join(s.Directory, id, key, "connection.json"), nil
}

func privatePath(path string, create bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if strings.HasPrefix(filepath.ToSlash(abs), "//") {
		return fmt.Errorf("integration state must use a local filesystem")
	}
	for current := abs; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 && !platform.IsSystemPathAlias(current) {
			return fmt.Errorf("integration state must not traverse a symbolic link: %s", current)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if create {
		return os.MkdirAll(abs, 0700)
	}
	return nil
}

func (s *ConnectionStore) Load(id string, o HostOptions) (Connection, error) {
	path, err := s.path(id, o)
	if err != nil {
		return Connection{}, err
	}
	if err = privatePath(path, false); err != nil {
		return Connection{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Connection{Version: 1, Smoke: Evidence{State: "unverified"}}, nil
	}
	if err != nil {
		return Connection{}, err
	}
	var c Connection
	if len(data) > 65536 || json.Unmarshal(data, &c) != nil || c.Version != 1 {
		return c, fmt.Errorf("unrecognized integration connection record; inspect %s", path)
	}
	return c, nil
}

func (s *ConnectionStore) Save(id string, o HostOptions, c Connection) error {
	path, err := s.path(id, o)
	if err != nil {
		return err
	}
	if err = privatePath(filepath.Dir(path), true); err != nil {
		return err
	}
	if err = privatePath(path, false); err != nil {
		return err
	}
	owner, err := storage.AcquireOwnership(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer owner.Close()
	c.Version = 1
	return storage.WriteJSON(path, c)
}

// A smoke test updates evidence only; configure owns the model selection.
func (s *ConnectionStore) RecordSmoke(id string, o HostOptions, e Evidence, contract string) error {
	path, err := s.path(id, o)
	if err != nil {
		return err
	}
	if err = privatePath(filepath.Dir(path), true); err != nil {
		return err
	}
	owner, err := storage.AcquireOwnership(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer owner.Close()
	c, err := s.Load(id, o)
	if err != nil {
		return err
	}
	c.Smoke = e
	c.ContractRevision = contract
	c.Version = 1
	return storage.WriteJSON(path, c)
}

type Snapshot struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Digest string `json:"digest,omitempty"`
	Backup string `json:"backup,omitempty"`
}

type Transaction struct {
	Directory string
	owner     *storage.Ownership
	files     []string
	baseline  []Snapshot
	stage     int
}

func (s *ConnectionStore) Begin(id string, files []string) (*Transaction, error) {
	if !validID(id) || len(files) == 0 {
		return nil, fmt.Errorf("integration configuration targets are required")
	}
	files = append([]string(nil), files...)
	sort.Strings(files)
	for i, path := range files {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if err = privatePath(abs, false); err != nil {
			return nil, err
		}
		files[i] = abs
	}
	// Lock by actual configuration targets, independent of endpoint/account.
	lockDir := filepath.Join(s.Directory, "locks", contentDigest([]byte(strings.Join(files, "\x00"))))
	if err := privatePath(lockDir, true); err != nil {
		return nil, err
	}
	owner, err := storage.AcquireOwnership(lockDir)
	if errors.Is(err, storage.ErrWorkspaceInUse) {
		return nil, fmt.Errorf("another integration configuration is in progress; wait for it to finish")
	}
	if err != nil {
		return nil, err
	}
	root := filepath.Join(s.Directory, id, "recovery")
	if err = privatePath(root, true); err != nil {
		owner.Close()
		return nil, err
	}
	dir, err := os.MkdirTemp(root, "configure-")
	if err != nil {
		owner.Close()
		return nil, err
	}
	tx := &Transaction{Directory: dir, owner: owner, files: files}
	if err = tx.Checkpoint(); err != nil {
		tx.Close()
		return nil, err
	}
	return tx, nil
}

func fileSnapshot(path string) (Snapshot, []byte, error) {
	s := Snapshot{Path: path}
	if err := privatePath(path, false); err != nil {
		return s, nil, err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return s, nil, nil
	}
	if err != nil {
		return s, nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return s, nil, fmt.Errorf("configuration is not a bounded regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, nil, err
	}
	s.Exists, s.Digest = true, contentDigest(data)
	return s, data, nil
}

func (t *Transaction) Checkpoint() error {
	var snapshots []Snapshot
	for i, path := range t.files {
		s, data, err := fileSnapshot(path)
		if err != nil {
			return err
		}
		if s.Exists {
			s.Backup = fmt.Sprintf("snapshot-%d-%d", t.stage, i)
			if err = storage.WriteBytes(filepath.Join(t.Directory, s.Backup), data); err != nil {
				return err
			}
		}
		snapshots = append(snapshots, s)
	}
	if err := storage.WriteJSON(filepath.Join(t.Directory, fmt.Sprintf("snapshot-%d.json", t.stage)), snapshots); err != nil {
		return err
	}
	t.stage++
	t.baseline = snapshots
	return nil
}

// VerifyUnchanged is called immediately before a provider mutation. Provider
// conditional writes additionally guard races inside its own config CLI.
func (t *Transaction) VerifyUnchanged() error {
	for _, before := range t.baseline {
		after, _, err := fileSnapshot(before.Path)
		if err != nil {
			return err
		}
		if before.Exists != after.Exists || before.Digest != after.Digest {
			return fmt.Errorf("configuration changed since inspection: %s; inspect before retrying", before.Path)
		}
	}
	return nil
}

func (t *Transaction) Record(phase string) error {
	return storage.WriteJSON(filepath.Join(t.Directory, "journal.json"), map[string]any{"version": 1, "phase": phase, "updated_at": time.Now().UTC().Format(time.RFC3339), "snapshot_count": t.stage})
}

func (t *Transaction) Close() error { return t.owner.Close() }

func ConfigurationDigest(files []string) (string, error) {
	var snapshots []Snapshot
	for _, path := range files {
		s, _, err := fileSnapshot(path)
		if err != nil {
			return "", err
		}
		snapshots = append(snapshots, s)
	}
	data, _ := json.Marshal(snapshots)
	return contentDigest(data), nil
}
