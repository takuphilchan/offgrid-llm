package execution

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FolderTrust is a native-client preference, never a service grant. Every use
// still creates a fresh owner-bound session through normal authenticated pairing.
type FolderTrust struct {
	Version     int         `json:"version"`
	ID          string      `json:"id"`
	Service     string      `json:"service"`
	Workspace   string      `json:"workspace"`
	Actor       string      `json:"actor"`
	Host        string      `json:"host"`
	Root        string      `json:"root"`
	Identity    string      `json:"identity"`
	Permissions Permissions `json:"permissions"`
	Created     time.Time   `json:"created"`
}
type FolderTrustStore struct{ Directory string }

func trustScope(a LocalApproval) (FolderTrust, error) {
	if a.Backend != "local" || a.Isolation != (IsolationPolicy{}) || !validIdentity(a.Actor) || !validIdentity(a.Host) || !validIdentity(a.Workspace) || a.Service == "" || !filepath.IsAbs(a.Root) {
		return FolderTrust{}, ErrAuthority
	}
	entry, err := os.Lstat(a.Root)
	if err != nil || !entry.IsDir() || entry.Mode()&os.ModeSymlink != 0 {
		return FolderTrust{}, ErrPath
	}
	root, err := filepath.EvalSymlinks(a.Root)
	if err != nil {
		return FolderTrust{}, ErrPath
	}
	identity, err := folderIdentity(root)
	if err != nil {
		return FolderTrust{}, err
	}
	r := FolderTrust{Version: 1, Service: a.Service, Workspace: a.Workspace, Actor: a.Actor, Host: a.Host, Root: root, Identity: identity, Permissions: a.Permissions}
	data, _ := json.Marshal(r)
	sum := sha256.Sum256(data)
	r.ID = hex.EncodeToString(sum[:])
	return r, nil
}
func (s FolderTrustStore) ready(root string) error {
	if !filepath.IsAbs(s.Directory) {
		return ErrPath
	}
	if root != "" {
		if err := PrivateStateOutsideRoot(root, s.Directory); err != nil {
			return err
		}
	}
	if err := EnsurePrivateDirectory(s.Directory); err != nil {
		return err
	}
	return nil
}
func (s FolderTrustStore) Lookup(a LocalApproval) (bool, error) {
	r, err := trustScope(a)
	if err != nil {
		return false, err
	}
	if err = s.ready(a.Root); err != nil {
		return false, err
	}
	saved, err := s.read(r.ID)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	saved.Created = time.Time{}
	return saved == r, nil
}
func (s FolderTrustStore) Remember(a LocalApproval) error {
	r, err := trustScope(a)
	if err != nil {
		return err
	}
	if err = s.ready(a.Root); err != nil {
		return err
	}
	r.Created = time.Now().UTC()
	data, _ := json.Marshal(r)
	file, err := os.OpenFile(filepath.Join(s.Directory, r.ID+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		ok, lookupErr := s.Lookup(a)
		if lookupErr == nil && ok {
			return nil
		}
		return ErrAuthority
	}
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err = errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return ProtectPrivatePath(filepath.Join(s.Directory, r.ID+".json"))
}
func (s FolderTrustStore) read(id string) (FolderTrust, error) {
	var r FolderTrust
	if len(id) != 64 || strings.ToLower(id) != id {
		return r, ErrPath
	}
	if _, err := hex.DecodeString(id); err != nil {
		return r, ErrPath
	}
	path := filepath.Join(s.Directory, id+".json")
	if err := CheckPrivatePath(path); err != nil {
		return r, err
	}
	file, err := os.Open(path)
	if err != nil {
		return r, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(data) > 8192 {
		return r, ErrLimit
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&r) != nil || decoder.Decode(new(any)) != io.EOF || r.Version != 1 || r.ID != id || r.Created.IsZero() {
		return FolderTrust{}, ErrAuthority
	}
	return r, nil
}
func (s FolderTrustStore) List() ([]FolderTrust, error) {
	if err := s.ready(""); err != nil {
		return nil, err
	}
	files, err := os.ReadDir(s.Directory)
	if err != nil {
		return nil, err
	}
	if len(files) > 1024 {
		return nil, ErrLimit
	}
	out := []FolderTrust{}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		r, err := s.read(strings.TrimSuffix(f.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
func (s FolderTrustStore) Revoke(id string) error {
	if err := s.ready(""); err != nil {
		return err
	}
	if _, err := s.read(id); err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.Directory, id+".json"))
}
