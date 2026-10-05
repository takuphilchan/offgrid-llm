package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

var acquisitionID = regexp.MustCompile(`^model-[a-f0-9]{32}$`)

type PackageCheckpoint struct {
	Phase     string
	Artifact  int
	BytesDone int64
	Verified  bool
}

// A checkpoint may fail (for example on full disk). Callers must durably record
// activation intent in the activating callback before publication may begin.
type PackageCheckpointFunc func(PackageCheckpoint) error

type packageInstallation struct {
	SchemaVersion  int             `json:"schema_version"`
	OperationID    string          `json:"operation_id"`
	ManifestDigest string          `json:"manifest_digest"`
	Provenance     ModelProvenance `json:"provenance"`
}
type packagePublication struct {
	SchemaVersion  int    `json:"schema_version"`
	OperationID    string `json:"operation_id"`
	ID             string `json:"id"`
	Revision       string `json:"revision"`
	ManifestDigest string `json:"manifest_digest"`
	Repair         bool   `json:"repair"`
}

func ManifestDigest(m ModelPackageManifest) string {
	b, _ := json.Marshal(m)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func acquisitionStage(operation string) (string, error) {
	if !acquisitionID.MatchString(operation) {
		return "", fmt.Errorf("invalid model operation identity")
	}
	return ".download-" + operation, nil
}

func checkedDirectory(root *os.Root, p string) error {
	for dir := p; dir != "."; dir = path.Dir(dir) {
		info, err := root.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("package directory is missing or replaced")
		}
	}
	return nil
}
func readPackageJSON(root *os.Root, p string, v any) error {
	f, err := regularPackageFile(root, p)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxPackageManifestBytes+1))
	if err != nil {
		return err
	}
	if len(b) > MaxPackageManifestBytes {
		return fmt.Errorf("package metadata exceeds limit")
	}
	return json.Unmarshal(b, v)
}
func hashPackageFile(ctx context.Context, f *os.File, size int64, digest string) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(contextPackageReader{ctx, f}, size+1))
	if err != nil {
		return err
	}
	if n != size || hex.EncodeToString(h.Sum(nil)) != digest {
		return acquisitionError("artifact_integrity_failed", "A required artifact failed its size or SHA-256 check. Partial data was retained for an explicit retry.")
	}
	return nil
}

func (hf *HuggingFaceClient) transferPackageArtifact(ctx context.Context, root *os.Root, partial string, a PackageArtifact, progress func(int64) error) error {
	if info, err := root.Lstat(partial); err == nil {
		if !info.Mode().IsRegular() {
			return acquisitionError("unsafe_staging", "Partial data is not a regular file.")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	f, err := root.OpenFile(partial, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return acquisitionError("unsafe_staging", "Partial data cannot be inspected.")
	}
	reset := func() error {
		if err := f.Truncate(0); err != nil {
			return err
		}
		_, err := f.Seek(0, io.SeekStart)
		return err
	}
	if info.Size() >= a.Size {
		if hashPackageFile(ctx, f, a.Size, a.SHA256) == nil {
			return progress(a.Size)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Explicit retry of a corrupt full artifact: never append new source
		// bytes to a known-bad object and never publish it as installed.
		if err = reset(); err != nil {
			return err
		}
	}
	info, err = f.Stat()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	c := hf.packageClient(false)
	c.Timeout = 0
	source := a.Source.Repository + "/resolve/" + a.Source.Revision + "/" + escapeHubPath(a.Source.Path)
	response, err := openResumableOffsetContext(ctx, c, source, "OffGrid-package-download/1", info.Size(), reset)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return acquisitionError("transfer_failed", "Model transfer was interrupted. Partial data was retained; check connectivity and Resume.")
	}
	if response.response != nil {
		defer response.response.Body.Close()
	}
	// Some CDN/proxy combinations omit Content-Length after redirecting to
	// object storage. The final byte count and SHA-256 check below remain the
	// authoritative identity checks; reject only a known, conflicting length.
	if response.total > 0 && response.total != a.Size {
		return acquisitionError("source_identity_mismatch", "Source length differs from the resolved immutable artifact.")
	}
	if response.complete {
		return hashPackageFile(ctx, f, a.Size, a.SHA256)
	}
	if _, err = f.Seek(response.offset, io.SeekStart); err != nil {
		return err
	}
	done := response.offset
	if err = progress(done); err != nil {
		return err
	}
	buffer := make([]byte, 128<<10)
	reader := io.LimitReader(response.response.Body, a.Size-done+1)
	last := time.Now()
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, readErr := reader.Read(buffer)
		if int64(n) > a.Size-done {
			return acquisitionError("source_identity_mismatch", "Source exceeded the declared artifact size.")
		}
		if n > 0 {
			written, writeErr := f.Write(buffer[:n])
			done += int64(written)
			if writeErr != nil {
				return writeErr
			}
			if written != n {
				return io.ErrShortWrite
			}
		}
		if time.Since(last) >= 250*time.Millisecond {
			if err = f.Sync(); err != nil {
				return err
			}
			if err = progress(done); err != nil {
				return err
			}
			last = time.Now()
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return acquisitionError("transfer_failed", "Model transfer was interrupted. Partial data was retained.")
		}
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = progress(done); err != nil {
		return err
	}
	return hashPackageFile(ctx, f, a.Size, a.SHA256)
}

// AcquireFromHub owns one package until transfer/verification/publication settle.
// Its durable operation is owned by the existing server download coordinator.
func (s *PackageStore) AcquireFromHub(ctx context.Context, m ModelPackageManifest, operation string, repair bool, provenance ModelProvenance, hf *HuggingFaceClient, checkpoint PackageCheckpointFunc) (PackageState, error) {
	encoded, err := json.Marshal(m)
	if err != nil {
		return PackageState{}, err
	}
	m, err = DecodePackageManifest(strings.NewReader(string(encoded)))
	if err != nil {
		return PackageState{}, err
	}
	if err := ValidateAcquisitionManifest(m); err != nil {
		return PackageState{}, err
	}
	stage, err := acquisitionStage(operation)
	if err != nil {
		return PackageState{}, err
	}
	if hf == nil || checkpoint == nil {
		return PackageState{}, fmt.Errorf("source and durable checkpoint callback required")
	}
	key, release, err := s.reserve(m.ID, m.Revision, "download", false)
	if err != nil {
		return PackageState{}, err
	}
	defer release()
	root, err := s.open()
	if err != nil {
		return PackageState{}, err
	}
	defer root.Close()
	// A committed receipt takes precedence over retrying a lost acknowledgment.
	if installed, err := s.reconcilePublication(ctx, root, m, operation); err != nil {
		return PackageState{}, err
	} else if installed {
		return s.verifyReserved(ctx, root, m.ID, m.Revision)
	}
	_, statErr := root.Lstat(key)
	if statErr == nil && !repair {
		return PackageState{}, ErrPackageConflict
	}
	if errors.Is(statErr, fs.ErrNotExist) && repair {
		return PackageState{}, ErrPackageNotFound
	}
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return PackageState{}, statErr
	}
	if err = root.Mkdir(stage, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return PackageState{}, err
	}
	if err = checkedDirectory(root, stage); err != nil {
		return PackageState{}, err
	}
	var binding ModelPackageManifest
	if err = readPackageJSON(root, stage+"/source.json", &binding); errors.Is(err, fs.ErrNotExist) {
		if err = writePackageJSON(root, stage+"/source.json", m); err != nil {
			return PackageState{}, err
		}
	} else if err != nil || ManifestDigest(binding) != ManifestDigest(m) {
		return PackageState{}, acquisitionError("source_conflict", "Retained bytes belong to another model source. Discard that operation explicitly.")
	}
	if err = root.Mkdir(stage+"/data", 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return PackageState{}, err
	}
	if err = checkedDirectory(root, stage+"/data"); err != nil {
		return PackageState{}, err
	}
	for i, a := range m.Artifacts {
		if ctx.Err() != nil {
			return PackageState{}, ctx.Err()
		}
		final := stage + "/data/" + a.Path
		if f, err := regularPackageFile(root, final); err == nil {
			checkErr := hashPackageFile(ctx, f, a.Size, a.SHA256)
			f.Close()
			if checkErr == nil {
				if err = checkpoint(PackageCheckpoint{"verifying", i, a.Size, true}); err != nil {
					return PackageState{}, err
				}
				continue
			}
			if ctx.Err() != nil {
				return PackageState{}, ctx.Err()
			}
			if err = root.Remove(final); err != nil {
				return PackageState{}, err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return PackageState{}, err
		}
		partial := stage + fmt.Sprintf("/artifact-%04d.part", i)
		if err = hf.transferPackageArtifact(ctx, root, partial, a, func(done int64) error { return checkpoint(PackageCheckpoint{"downloading", i, done, false}) }); err != nil {
			return PackageState{}, err
		}
		if err = root.MkdirAll(path.Dir(final), 0700); err != nil {
			return PackageState{}, err
		}
		if err = checkedDirectory(root, path.Dir(final)); err != nil {
			return PackageState{}, err
		}
		if err = root.Rename(partial, final); err != nil {
			return PackageState{}, err
		}
		if err = checkpoint(PackageCheckpoint{"verifying", i, a.Size, true}); err != nil {
			return PackageState{}, err
		}
	}
	for _, name := range []string{"manifest.json", "installation.json"} {
		// These files are generated by this transaction, never supplied by the
		// repository. A retry can replace them before the activation intent.
		if err = root.Remove(stage + "/data/" + name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return PackageState{}, err
		}
	}
	if err = writePackageJSON(root, stage+"/data/manifest.json", m); err != nil {
		return PackageState{}, err
	}
	receipt := packageInstallation{SchemaVersion: 1, OperationID: operation, ManifestDigest: ManifestDigest(m), Provenance: provenance}
	if err = writePackageJSON(root, stage+"/data/installation.json", receipt); err != nil {
		return PackageState{}, err
	}
	if ctx.Err() != nil {
		return PackageState{}, ctx.Err()
	}
	if err = checkpoint(PackageCheckpoint{Phase: "activating", Artifact: -1}); err != nil {
		return PackageState{}, err
	}
	if ctx.Err() != nil {
		return PackageState{}, ctx.Err()
	}
	journal := packagePublication{SchemaVersion: 1, OperationID: operation, ID: m.ID, Revision: m.Revision, ManifestDigest: ManifestDigest(m), Repair: repair}
	journalPath := ".publish-" + operation + ".json"
	if err = writePackageJSON(root, journalPath, journal); err != nil {
		return PackageState{}, err
	}
	if err = root.Mkdir(m.ID, 0700); err != nil && !errors.Is(err, fs.ErrExist) {
		return PackageState{}, err
	}
	if err = checkedDirectory(root, m.ID); err != nil {
		return PackageState{}, err
	}
	backup := ".previous-" + operation
	if repair {
		if err = checkedDirectory(root, key); err != nil {
			return PackageState{}, err
		}
		if err = root.Rename(key, backup); err != nil {
			return PackageState{}, err
		}
	}
	if err = root.Rename(stage+"/data", key); err != nil {
		if repair {
			if restoreErr := root.Rename(backup, key); restoreErr != nil {
				return PackageState{}, errors.Join(err, fmt.Errorf("previous package retained for recovery: %w", restoreErr))
			}
		}
		return PackageState{}, err
	}
	// Leave backup and journal until the service persists completion. A crash
	// here is reconciled by operation ID + manifest receipt, never redownloaded.
	s.mu.Lock()
	s.verified[key] = time.Now().UTC()
	delete(s.failed, key)
	state := s.stateLocked(root, m.ID, m.Revision)
	s.mu.Unlock()
	return state, nil
}

func (s *PackageStore) reconcilePublication(ctx context.Context, root *os.Root, m ModelPackageManifest, operation string) (bool, error) {
	key, _ := packageKey(m.ID, m.Revision)
	var receipt packageInstallation
	if err := readPackageJSON(root, key+"/installation.json", &receipt); err == nil && receipt.OperationID == operation {
		if receipt.SchemaVersion != 1 || receipt.ManifestDigest != ManifestDigest(m) {
			return false, acquisitionError("source_conflict", "Installed receipt differs from this operation.")
		}
		var stored ModelPackageManifest
		if err := readPackageJSON(root, key+"/manifest.json", &stored); err != nil || ManifestDigest(stored) != ManifestDigest(m) {
			return false, acquisitionError("source_conflict", "Installed manifest differs from its operation receipt.")
		}
		return true, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	var journal packagePublication
	journalPath := ".publish-" + operation + ".json"
	if err := readPackageJSON(root, journalPath, &journal); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if journal.SchemaVersion != 1 || journal.OperationID != operation || journal.ID != m.ID || journal.Revision != m.Revision || journal.ManifestDigest != ManifestDigest(m) {
		return false, acquisitionError("source_conflict", "Publication recovery metadata does not match this operation.")
	}
	backup := ".previous-" + operation
	if _, err := root.Lstat(backup); err == nil {
		if err = checkedDirectory(root, backup); err != nil {
			return false, err
		}
		if _, err = root.Lstat(key); !errors.Is(err, fs.ErrNotExist) {
			return false, acquisitionError("recovery_required", "Both prior and current package paths exist without a matching completion receipt. Preserve both and inspect recovery state.")
		}
		if err = root.Rename(backup, key); err != nil {
			return false, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := root.Remove(journalPath); err != nil {
		return false, err
	}
	return false, ctx.Err()
}

// Reconcile is observation/recovery only: it never starts network transfer.
func (s *PackageStore) Reconcile(ctx context.Context, m ModelPackageManifest, operation string) (bool, error) {
	if _, err := acquisitionStage(operation); err != nil {
		return false, err
	}
	if err := m.Validate(); err != nil {
		return false, err
	}
	_, release, err := s.reserve(m.ID, m.Revision, "recovery", false)
	if err != nil {
		return false, err
	}
	defer release()
	root, err := s.open()
	if err != nil {
		return false, err
	}
	defer root.Close()
	done, err := s.reconcilePublication(ctx, root, m, operation)
	if done && err == nil {
		_, err = s.verifyReserved(ctx, root, m.ID, m.Revision)
	}
	return done, err
}

// SettleAcquisition is called only after durable completion, or explicit discard
// of an inactive operation. It never removes the installed package itself.
func (s *PackageStore) SettleAcquisition(m ModelPackageManifest, operation string, discard bool) error {
	stage, err := acquisitionStage(operation)
	if err != nil {
		return err
	}
	_, release, err := s.reserve(m.ID, m.Revision, "cleanup", false)
	if err != nil {
		return err
	}
	defer release()
	root, err := s.open()
	if err != nil {
		return err
	}
	defer root.Close()
	completed, err := s.reconcilePublication(context.Background(), root, m, operation)
	if err != nil {
		return err
	}
	if !discard && !completed {
		return acquisitionError("recovery_required", "No committed installation receipt exists; partial data must be explicitly discarded.")
	}
	for _, p := range []string{stage, ".previous-" + operation} {
		if _, err := root.Lstat(p); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err = checkedDirectory(root, p); err != nil {
			return err
		}
		if err = root.RemoveAll(p); err != nil {
			return err
		}
	}
	if err = root.Remove(".publish-" + operation + ".json"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *PackageStore) RetainedAcquisitionBytes(operation string) (int64, error) {
	stage, err := acquisitionStage(operation)
	if err != nil {
		return 0, err
	}
	root, err := s.open()
	if err != nil {
		return 0, err
	}
	defer root.Close()
	var total int64
	for _, directory := range []string{stage, ".previous-" + operation} {
		if _, err = root.Lstat(directory); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return total, err
		}
		if err = checkedDirectory(root, directory); err != nil {
			return total, err
		}
		err = fs.WalkDir(root.FS(), directory, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !d.Type().IsRegular() {
				return acquisitionError("unsafe_staging", "Retained operation data includes links or special files.")
			}
			if p == stage+"/source.json" || p == stage+"/data/manifest.json" || p == stage+"/data/installation.json" || p == directory+"/manifest.json" || p == directory+"/installation.json" {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
			if total > MaxPackageBytes*2 {
				return acquisitionError("unsafe_staging", "Retained data exceeds package limits.")
			}
			return nil
		})
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
