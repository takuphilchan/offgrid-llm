package execution

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func validateWorkingCopyRequest(req WorkingCopyRequest, policy WorkingCopyPolicy) error {
	if req.Purpose != "" && req.Purpose != "edit" && req.Purpose != "test" {
		return ErrUnsupported
	}
	if !validIdentity(req.RequestID) || !validIdentity(req.WorkerRunID) || req.WorkerRunID == req.Parent.Job || req.InstructionVersion < 1 || !req.Permissions.Read || req.Permissions.Processes && !req.Permissions.Write || req.MaxBytes < 1 || req.MaxBytes > policy.MaxCopyBytes || len(req.RequiredPaths) > 128 {
		return ErrAuthority
	}
	for _, name := range req.RequiredPaths {
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || len(name) > 4096 {
			return ErrPath
		}
	}
	return validateCandidateReferences(req.Prerequisites, req)
}

func (s *WorkingCopies) Prepare(ctx context.Context, source *Session, req WorkingCopyRequest) (record WorkingCopyRecord, resultErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || source == nil {
		return record, ErrHostUnavailable
	}
	if err := validateWorkingCopyRequest(req, s.policy); err != nil {
		return record, err
	}
	source.mu.Lock()
	err := source.authorize(ctx, req.Parent)
	config := source.config
	if err == nil && !PermissionsWithin(req.Permissions, config.Permissions) {
		err = ErrAuthority
	}
	if err == nil && config.Backend != "local" {
		err = fmt.Errorf("working copy preparation is unavailable for backend %s: %w", config.Backend, ErrUnsupported)
	}
	var root *os.Root
	if err == nil {
		root, err = os.OpenRoot(config.Root)
	}
	if err == nil {
		a, e1 := source.root.Stat(".")
		b, e2 := root.Stat(".")
		if e1 != nil || e2 != nil || !os.SameFile(a, b) {
			err = ErrPath
		}
	}
	source.mu.Unlock()
	if root != nil {
		defer root.Close()
	}
	if err != nil {
		return record, err
	}
	if err = PrivateStateOutsideRoot(config.Root, s.directory); err != nil {
		return record, err
	}
	if err = PrivateStateOutsideRoot(s.directory, config.Root); err != nil {
		return record, err
	}
	copyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-source.revokedSignal:
			cancel()
		case <-copyCtx.Done():
		}
	}()
	encoded, _ := json.Marshal(struct {
		Request WorkingCopyRequest
		Source  Context
		Policy  WorkingCopyPolicy
	}{req, config, s.policy})
	requestDigest := digest(encoded)
	identity, _ := json.Marshal(struct {
		Parent Binding
		ID     string
	}{req.Parent, req.RequestID})
	id := digest(identity)
	var previous string
	if err = s.db.QueryRowContext(ctx, `SELECT digest FROM worker_copies WHERE id=?`, id).Scan(&previous); err == nil {
		// Existing artifacts never reconstruct a live grant or repeat preparation.
		return record, fmt.Errorf("working copy already recorded; inspect its original preparation: %w", ErrConflict)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return record, err
	}
	if err = s.refreshCopiesLocked(copyCtx); err != nil {
		return record, err
	}
	scanReq := req
	scanReq.RequiredPaths = nil // Required inputs may be created by a prerequisite.
	sourceManifest, err := inventoryWorkingCopy(copyCtx, root, scanReq, s.policy)
	if err != nil {
		return record, err
	}
	sourceDigest, _, err := workingCopyManifestDigest(sourceManifest)
	if err != nil {
		return record, err
	}
	manifest, blobs, err := s.candidateManifest(copyCtx, sourceManifest, req)
	if err != nil {
		return record, err
	}
	manifestDigest, manifestData, err := workingCopyManifestDigest(manifest)
	if err != nil {
		return record, err
	}
	record = WorkingCopyRecord{ID: id, State: "preparing", Source: req.Parent, WorkerRunID: req.WorkerRunID, ReservedBytes: req.MaxBytes, InputBytes: manifest.Bytes, ManifestDigest: manifestDigest}
	record.Purpose = req.Purpose
	data, _ := json.Marshal(record)
	tx, err := s.db.BeginTx(copyCtx, nil)
	if err != nil {
		return record, err
	}
	defer tx.Rollback()
	var used int64
	var copies int
	if err = tx.QueryRow(`SELECT COALESCE(SUM(reserved_bytes),0),COUNT(*) FROM worker_copies WHERE state!='released'`).Scan(&used, &copies); err != nil {
		return record, err
	}
	if used < 0 || copies >= s.policy.MaxCopies || req.MaxBytes > s.policy.MaxTotalBytes-used {
		return record, ErrLimit
	}
	if _, err = tx.Exec(`INSERT INTO worker_copies(id,digest,state,reserved_bytes,record,manifest) VALUES(?,?,'preparing',?,?,?)`, id, requestDigest, req.MaxBytes, data, manifestData); err != nil {
		return record, err
	}
	if err = tx.Commit(); err != nil {
		return record, err
	}
	defer func() {
		if resultErr != nil {
			_, _ = s.db.Exec(`UPDATE worker_copies SET state='uncertain' WHERE id=? AND state='preparing'`, id)
		}
	}()
	// Capacity is committed before creating the first copied byte.
	if err = s.root.Mkdir(id, 0700); err != nil {
		return record, err
	}
	if err = s.root.Mkdir(id+"/content", 0700); err != nil {
		return record, err
	}
	target, err := s.root.OpenRoot(id + "/content")
	if err != nil {
		return record, err
	}
	defer target.Close()
	record.rootInfo, err = target.Stat(".")
	if err != nil {
		return record, err
	}
	if err = copyCandidateTree(copyCtx, root, target, s.root, manifest, blobs); err != nil {
		return record, err
	}
	actual, err := inventoryWorkingCopy(copyCtx, target, req, s.policy)
	if err != nil {
		return record, err
	}
	changes, _, err := diffWorkingCopy(manifest, actual)
	if err != nil {
		return record, err
	}
	if len(changes) != 0 || len(actual.Exclusions) != 0 {
		return record, ErrRevision
	}
	after, err := inventoryWorkingCopy(copyCtx, root, scanReq, s.policy)
	if err != nil {
		return record, err
	}
	afterDigest, _, err := workingCopyManifestDigest(after)
	if err != nil {
		return record, err
	}
	if afterDigest != sourceDigest {
		return record, ErrRevision
	}
	if _, err = source.Snapshot(copyCtx, req.Parent); err != nil {
		return record, err
	}
	record.MeasuredBytes, record.UsageMeasured = manifest.Bytes, true
	data, _ = json.Marshal(record)
	if _, err = s.db.ExecContext(copyCtx, `UPDATE worker_copies SET state='ready',record=? WHERE id=? AND state='preparing'`, data, id); err != nil {
		return record, err
	}
	record.State = "ready"
	return record, nil
}

func copyWorkingTree(ctx context.Context, source, target *os.Root, manifest *WorkingCopyManifest) error {
	return copyCandidateTree(ctx, source, target, nil, manifest, nil)
}

func copyCandidateTree(ctx context.Context, source, target, artifacts *os.Root, manifest *WorkingCopyManifest, blobs map[string]string) error {
	for _, entry := range manifest.Entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Kind == "directory" {
			if err := target.Mkdir(entry.Path, 0700); err != nil {
				return err
			}
			continue
		}
		inputRoot, inputPath := source, entry.Path
		if blob := blobs[entry.Path]; blob != "" {
			inputRoot, inputPath = artifacts, blob
		}
		info, err := inputRoot.Lstat(inputPath)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() != entry.Bytes {
			return ErrRevision
		}
		input, err := openWorkingCopyFile(inputRoot, inputPath, info)
		if err != nil {
			return err
		}
		output, err := target.OpenFile(entry.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			input.Close()
			return err
		}
		hash := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(copyContextReader{ctx, input}, entry.Bytes))
		var extra [1]byte
		extraBytes, probeErr := input.Read(extra[:])
		if extraBytes != 0 || probeErr != io.EOF {
			copyErr = errors.Join(copyErr, ErrRevision)
		}
		inputErr := input.Close()
		syncErr := output.Sync()
		mode := os.FileMode(0600)
		if entry.Mode&0111 != 0 {
			mode |= 0100
		}
		modeErr := output.Chmod(mode)
		closeErr := output.Close()
		if err = errors.Join(copyErr, inputErr, syncErr, modeErr, closeErr); err != nil {
			return err
		}
		if n != entry.Bytes || hex.EncodeToString(hash.Sum(nil)) != entry.SHA256 {
			return ErrRevision
		}
	}
	return nil
}

func (s *WorkingCopies) contentPath(id string) string {
	return filepath.Join(s.directory, id, "content")
}
