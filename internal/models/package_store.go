package models

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const PackageDirectory = ".packages"

var ErrPackageConflict = errors.New("package revision already exists")
var ErrPackageInUse = errors.New("package is in use")
var ErrPackageNotFound = errors.New("package not found")

type PackageState struct {
	ID                string                `json:"id"`
	Revision          string                `json:"revision"`
	Manifest          *ModelPackageManifest `json:"manifest,omitempty"`
	Installed         bool                  `json:"installed"`
	Integrity         string                `json:"integrity"`
	VerifiedAt        *time.Time            `json:"verified_at,omitempty"`
	RuntimeCompatible bool                  `json:"runtime_compatible"`
	SmokeTested       bool                  `json:"smoke_tested"`
	Qualified         bool                  `json:"qualified"`
	Provenance        string                `json:"provenance"`
	Issue             string                `json:"issue,omitempty"`
	InUse             int                   `json:"in_use"`
	Operation         string                `json:"operation,omitempty"`
}

// PackageStore is owned by a Registry, not a parallel download manager. Future
// runtime adapters must hold Acquire leases for their entire model lifetime.
// Its locks are process-local: the service's existing workspace ownership policy
// remains responsible for excluding a second writer process.
type PackageStore struct {
	mu        sync.Mutex
	modelsDir string
	leases    map[string]map[string]int
	verified  map[string]time.Time
	failed    map[string]string
	busy      map[string]string
	wake      map[string]chan struct{}
}

func NewPackageStore(modelsDir string) *PackageStore {
	return &PackageStore{modelsDir: modelsDir, leases: map[string]map[string]int{}, verified: map[string]time.Time{}, failed: map[string]string{}, busy: map[string]string{}, wake: map[string]chan struct{}{}}
}

// Reserve one package, not the whole model platform. Transfers and hashes run
// outside mu; mutation, verification and removal cannot overtake each other.
func (s *PackageStore) reserve(id, revision, operation string, allowLeases bool) (string, func(), error) {
	key, err := packageKey(id, revision)
	if err != nil {
		return "", nil, err
	}
	s.mu.Lock()
	if s.busy[key] != "" || (!allowLeases && len(s.leases[key]) != 0) {
		s.mu.Unlock()
		return "", nil, ErrPackageInUse
	}
	s.busy[key] = operation
	s.wake[key] = make(chan struct{})
	s.mu.Unlock()
	var once sync.Once
	return key, func() {
		once.Do(func() { s.mu.Lock(); delete(s.busy, key); close(s.wake[key]); delete(s.wake, key); s.mu.Unlock() })
	}, nil
}

func (s *PackageStore) reserveVerification(ctx context.Context, id, revision string) (string, func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return "", nil, err
		}
		key, release, err := s.reserve(id, revision, "verify", true)
		if !errors.Is(err, ErrPackageInUse) {
			return key, release, err
		}
		key, _ = packageKey(id, revision)
		s.mu.Lock()
		busy, wake := s.busy[key], s.wake[key]
		s.mu.Unlock()
		if busy == "" {
			continue
		}
		if busy != "verify" {
			return "", nil, ErrPackageInUse
		}
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case <-wake:
		}
	}
}

func packageKey(id, revision string) (string, error) {
	if !validPackageComponent(id) || !validPackageComponent(revision) {
		return "", fmt.Errorf("invalid package identity")
	}
	return id + "/" + revision, nil
}

func (s *PackageStore) open() (*os.Root, error) {
	if err := os.MkdirAll(s.modelsDir, 0755); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(s.modelsDir)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	if err := r.Mkdir(PackageDirectory, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	info, err := r.Lstat(PackageDirectory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("package storage must be a regular directory")
	}
	return r.OpenRoot(PackageDirectory)
}

func regularPackageFile(root *os.Root, name string) (*os.File, error) {
	for p := name; p != "."; p = path.Dir(p) {
		info, err := root.Lstat(p)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || (p != name && !info.IsDir()) || (p == name && !info.Mode().IsRegular()) {
			return nil, fmt.Errorf("package links and special files are not allowed")
		}
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("not a regular package file")
	}
	return f, nil
}

type contextPackageReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextPackageReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// PackageArtifactReader supplies bytes only. It cannot introduce paths, archives,
// runtime commands, or extra artifacts into managed storage.
type PackageArtifactReader func(context.Context, PackageArtifact) (io.ReadCloser, error)

func (s *PackageStore) Import(ctx context.Context, manifest ModelPackageManifest, source PackageArtifactReader, validateEnd ...func() error) (PackageState, error) {
	// Copy slices/pointers as well as validating, so caller mutations cannot alter
	// the manifest after validation or after it has been persisted.
	b, err := json.Marshal(manifest)
	if err != nil {
		return PackageState{}, err
	}
	m, err := DecodePackageManifest(strings.NewReader(string(b)))
	if err != nil {
		return PackageState{}, err
	}
	if source == nil {
		return PackageState{}, fmt.Errorf("artifact source is required")
	}
	if len(validateEnd) > 1 {
		return PackageState{}, fmt.Errorf("only one source completion validator is permitted")
	}
	key, release, err := s.reserve(m.ID, m.Revision, "import", false)
	if err != nil {
		return PackageState{}, err
	}
	defer release()
	root, err := s.open()
	if err != nil {
		return PackageState{}, err
	}
	defer root.Close()
	if _, err := root.Lstat(key); err == nil {
		return PackageState{}, ErrPackageConflict
	} else if !errors.Is(err, fs.ErrNotExist) {
		return PackageState{}, err
	}
	var size int64
	for _, a := range m.Artifacts {
		size += a.Size
	}
	space, err := getDiskSpace(filepath.Join(s.modelsDir, PackageDirectory))
	if err != nil {
		return PackageState{}, fmt.Errorf("cannot check package disk space: %w", err)
	}
	if space < size+(16<<20) {
		return PackageState{}, fmt.Errorf("insufficient space for complete model package")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return PackageState{}, err
	}
	staging := ".stage-" + hex.EncodeToString(nonce[:])
	if err := root.Mkdir(staging, 0700); err != nil {
		return PackageState{}, err
	}
	defer root.RemoveAll(staging) // Exact, newly created confined staging directory only.
	for _, a := range m.Artifacts {
		if err := ctx.Err(); err != nil {
			return PackageState{}, err
		}
		if err := root.MkdirAll(staging+"/"+path.Dir(a.Path), 0700); err != nil {
			return PackageState{}, err
		}
		input, err := source(ctx, a)
		if err != nil {
			return PackageState{}, err
		}
		output, err := root.OpenFile(staging+"/"+a.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return PackageState{}, err
		}
		hash := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(contextPackageReader{ctx, input}, a.Size+1))
		inputErr := input.Close()
		syncErr := output.Sync()
		closeErr := output.Close()
		if err := errors.Join(copyErr, inputErr, syncErr, closeErr); err != nil {
			return PackageState{}, err
		}
		if n != a.Size || hex.EncodeToString(hash.Sum(nil)) != a.SHA256 {
			return PackageState{}, fmt.Errorf("size or digest mismatch for %q", a.Path)
		}
	}
	if len(validateEnd) == 1 && validateEnd[0] != nil {
		if err := validateEnd[0](); err != nil {
			return PackageState{}, err
		}
	}
	if err := writePackageJSON(root, staging+"/manifest.json", m); err != nil {
		return PackageState{}, err
	}
	if err := ctx.Err(); err != nil {
		return PackageState{}, err
	}
	if err := root.Mkdir(m.ID, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return PackageState{}, err
	}
	info, err := root.Lstat(m.ID)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return PackageState{}, fmt.Errorf("invalid package directory")
	}
	if err := root.Rename(staging, key); err != nil {
		return PackageState{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verified[key] = time.Now().UTC()
	return s.stateLocked(root, m.ID, m.Revision), nil
}

func writePackageJSON(root *os.Root, name string, value any) error {
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(value)
	return errors.Join(err, f.Sync(), f.Close())
}

// ImportDirectory never alters sources. All files must be declared; links and
// devices are rejected before opening. os.Root also blocks escape on replacement.
func (s *PackageStore) ImportDirectory(ctx context.Context, directory string) (PackageState, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		return PackageState{}, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return PackageState{}, fmt.Errorf("import source must be a regular directory")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return PackageState{}, err
	}
	defer root.Close()
	f, err := regularPackageFile(root, "manifest.json")
	if err != nil {
		return PackageState{}, err
	}
	m, err := DecodePackageManifest(f)
	f.Close()
	if err != nil {
		return PackageState{}, err
	}
	allowed := map[string]bool{"manifest.json": true}
	for _, a := range m.Artifacts {
		allowed[a.Path] = true
	}
	if err := fs.WalkDir(root.FS(), ".", func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("linked package entry is not allowed")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || !allowed[p] {
			return fmt.Errorf("undeclared or special package entry %q", p)
		}
		return nil
	}); err != nil {
		return PackageState{}, err
	}
	return s.Import(ctx, m, func(_ context.Context, a PackageArtifact) (io.ReadCloser, error) {
		return regularPackageFile(root, a.Path)
	})
}

func (s *PackageStore) stateLocked(root *os.Root, id, revision string) PackageState {
	key, _ := packageKey(id, revision)
	state := PackageState{ID: id, Revision: revision, Installed: true, Integrity: "unchecked", Provenance: "local_untrusted", Issue: "runtime_unavailable"}
	state.Operation = s.busy[key]
	for _, count := range s.leases[key] {
		state.InUse += count
	}
	f, err := regularPackageFile(root, key+"/manifest.json")
	if err != nil {
		state.Integrity, state.Issue = "failed", "manifest_invalid"
		return state
	}
	m, err := DecodePackageManifest(f)
	f.Close()
	if err != nil || m.ID != id || m.Revision != revision {
		state.Integrity, state.Issue = "failed", "manifest_invalid"
		return state
	}
	state.Manifest = &m
	var receipt packageInstallation
	if err := readPackageJSON(root, key+"/installation.json", &receipt); err == nil {
		if receipt.SchemaVersion != 1 || !acquisitionID.MatchString(receipt.OperationID) || receipt.ManifestDigest != ManifestDigest(m) || (receipt.Provenance.Kind != "repository_metadata" && receipt.Provenance.Kind != "curated_manifest") {
			state.Integrity, state.Issue = "failed", "installation_invalid"
			return state
		}
		state.Provenance = receipt.Provenance.Kind
	} else if !errors.Is(err, fs.ErrNotExist) {
		state.Integrity, state.Issue = "failed", "installation_invalid"
		return state
	}
	if issue, ok := s.failed[key]; ok {
		state.Integrity, state.Issue = "failed", issue
		return state
	}
	if at, ok := s.verified[key]; ok {
		state.Integrity, state.VerifiedAt = "checked", &at
	}
	return state
}

func (s *PackageStore) List() ([]PackageState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := s.open()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	ids, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	result := []PackageState{}
	for _, id := range ids {
		if !validPackageComponent(id.Name()) {
			continue
		} // private staging is not installed
		if !id.IsDir() {
			return nil, fmt.Errorf("invalid model package storage entry")
		}
		revisions, err := fs.ReadDir(root.FS(), id.Name())
		if err != nil {
			return nil, err
		}
		for _, rev := range revisions {
			if !validPackageComponent(rev.Name()) {
				return nil, fmt.Errorf("invalid model package revision entry")
			}
			result = append(result, s.stateLocked(root, id.Name(), rev.Name()))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID+"/"+result[i].Revision < result[j].ID+"/"+result[j].Revision })
	return result, nil
}

func (s *PackageStore) verifyReserved(ctx context.Context, root *os.Root, id, revision string) (PackageState, error) {
	key, err := packageKey(id, revision)
	if err != nil {
		return PackageState{}, err
	}
	if _, err := root.Lstat(key); errors.Is(err, fs.ErrNotExist) {
		return PackageState{}, ErrPackageNotFound
	} else if err != nil {
		return PackageState{}, err
	}
	s.mu.Lock()
	delete(s.verified, key)
	delete(s.failed, key)
	state := s.stateLocked(root, id, revision)
	s.mu.Unlock()
	fail := func(issue string) { s.mu.Lock(); s.failed[key] = issue; s.mu.Unlock() }
	if state.Manifest == nil || state.Issue == "installation_invalid" {
		fail("manifest_invalid")
		return state, fmt.Errorf("invalid installed package manifest")
	}
	for _, a := range state.Manifest.Artifacts {
		f, err := regularPackageFile(root, key+"/"+a.Path)
		if err != nil {
			state.Integrity, state.Issue = "failed", "artifact_invalid"
			fail(state.Issue)
			return state, fmt.Errorf("missing or unsafe artifact %q", a.Path)
		}
		hash := sha256.New()
		n, err := io.Copy(hash, io.LimitReader(contextPackageReader{ctx, f}, a.Size+1))
		f.Close()
		if err != nil {
			return state, err
		}
		if n != a.Size || hex.EncodeToString(hash.Sum(nil)) != a.SHA256 {
			state.Integrity, state.Issue = "failed", "artifact_invalid"
			fail(state.Issue)
			return state, fmt.Errorf("size or digest mismatch for %q", a.Path)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.verified[key] = time.Now().UTC()
	return s.stateLocked(root, id, revision), nil
}

func (s *PackageStore) Verify(ctx context.Context, id, revision string) (PackageState, error) {
	_, release, err := s.reserveVerification(ctx, id, revision)
	if err != nil {
		return PackageState{}, err
	}
	defer release()
	root, err := s.open()
	if err != nil {
		return PackageState{}, err
	}
	defer root.Close()
	return s.verifyReserved(ctx, root, id, revision)
}

// Acquire verifies bytes again and pins an immutable revision until release.
// It does not imply adapter compatibility or permission to execute the model.
func (s *PackageStore) Acquire(ctx context.Context, id, revision, consumer string) (PackageState, func(), error) {
	if strings.TrimSpace(consumer) == "" || len(consumer) > 200 {
		return PackageState{}, nil, fmt.Errorf("lease consumer is required")
	}
	key, releaseCheck, err := s.reserveVerification(ctx, id, revision)
	if err != nil {
		return PackageState{}, nil, err
	}
	defer releaseCheck()
	root, err := s.open()
	if err != nil {
		return PackageState{}, nil, err
	}
	defer root.Close()
	state, err := s.verifyReserved(ctx, root, id, revision)
	if err != nil {
		return state, nil, err
	}
	s.mu.Lock()
	if s.leases[key] == nil {
		s.leases[key] = map[string]int{}
	}
	s.leases[key][consumer]++
	s.mu.Unlock()
	state.InUse++
	var once sync.Once
	release := func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.leases[key][consumer]--
			if s.leases[key][consumer] == 0 {
				delete(s.leases[key], consumer)
			}
			if len(s.leases[key]) == 0 {
				delete(s.leases, key)
			}
		})
	}
	return state, release, nil
}

func (s *PackageStore) Remove(id, revision string) error {
	key, release, err := s.reserve(id, revision, "remove", false)
	if err != nil {
		return err
	}
	defer release()
	root, err := s.open()
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Lstat(key); errors.Is(err, fs.ErrNotExist) {
		return ErrPackageNotFound
	} else if err != nil {
		return err
	}
	for directory := key; directory != "."; directory = path.Dir(directory) {
		info, err := root.Lstat(directory)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing removal through a replaced package directory")
		}
	}
	// Never follow a replaced package directory. RemoveAll operates only on the
	// validated two-component key under the confined managed root.
	if err := root.RemoveAll(key); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.verified, key)
	delete(s.failed, key)
	return nil
}
