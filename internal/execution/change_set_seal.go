package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"time"
)

func (w *PreparedWorker) SealChangeSet(ctx context.Context, req SealChangeSetRequest) (ChangeSet, error) {
	if req.CopyID != w.Record.ID || req.Source != w.Record.Source {
		return ChangeSet{}, ErrAuthority
	}
	if err := w.Close(); err != nil {
		return ChangeSet{}, err
	}
	return w.store.SealChangeSet(ctx, req)
}

// SealChangeSet captures private artifacts, not changes to the original folder.
// The trusted supervisor must first quiesce the worker. Concurrent writers are
// still detected by digest checks; their absence is not assumed from a hash.
func (s *WorkingCopies) SealChangeSet(ctx context.Context, req SealChangeSetRequest) (set ChangeSet, resultErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || !validCopyID(req.CopyID) || !validIdentity(req.RequestID) {
		return set, ErrAuthority
	}
	record, err := s.inspect(ctx, req.CopyID)
	if err != nil {
		return set, err
	}
	if record.Source != req.Source {
		return set, ErrAuthority
	}
	id, requestDigest := changeSetIdentity(req)
	var prior string
	if err = s.db.QueryRowContext(ctx, `SELECT request_digest FROM worker_change_sets WHERE id=?`, id).Scan(&prior); err == nil {
		if prior != requestDigest {
			return set, ErrConflict
		}
		set, _, err = s.changeSet(ctx, id, req.Source)
		return set, err
	} else if !errors.Is(err, sql.ErrNoRows) {
		return set, err
	}
	if record.State != "ready" {
		return set, ErrConflict
	}
	var baselineData []byte
	if err = s.db.QueryRowContext(ctx, `SELECT manifest FROM worker_copies WHERE id=?`, record.ID).Scan(&baselineData); err != nil {
		return set, err
	}
	if digest(baselineData) != record.ManifestDigest {
		return set, ErrRevision
	}
	var base WorkingCopyManifest
	if err = json.Unmarshal(baselineData, &base); err != nil {
		return set, err
	}
	if base.Source != req.Source || base.InstructionVersion != req.InstructionVersion || base.ControlEpoch != req.ControlEpoch {
		return set, ErrRevision
	}
	// Reconcile existing copy/artifact growth before reserving another snapshot.
	if err = s.refreshCopiesLocked(ctx); err != nil {
		return set, err
	}
	record, err = s.inspect(ctx, req.CopyID)
	if err != nil {
		return set, err
	}
	root, err := s.root.OpenRoot(req.CopyID + "/content")
	if err != nil {
		return set, err
	}
	defer root.Close()
	inventoryRequest := WorkingCopyRequest{Parent: req.Source, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch, MaxBytes: s.policy.MaxCopyBytes}
	result, err := inventoryWorkingCopy(ctx, root, inventoryRequest, s.policy)
	if err != nil {
		return set, err
	}
	if len(result.Exclusions) != 0 {
		return set, ErrUnsupported
	} // Worker-created Git metadata cannot disappear from a proposal.
	changes, blobBytes, err := diffWorkingCopy(&base, result)
	if err != nil {
		return set, err
	}
	validation, err := s.validationSummary(ctx, record, &base, result)
	if err != nil {
		return set, err
	}
	if req.MaxReservedBytes < 0 || req.MaxReservedBytes > 0 && (record.ContentBytes > req.MaxReservedBytes || record.ArtifactReservedBytes > req.MaxReservedBytes-record.ContentBytes || blobBytes > req.MaxReservedBytes-record.ContentBytes-record.ArtifactReservedBytes || record.ReservedBytes > req.MaxReservedBytes) {
		return set, ErrLimit
	}
	resultDigest, resultData, err := workingCopyManifestDigest(result)
	if err != nil {
		return set, err
	}
	operations, err := json.Marshal(changes)
	if err != nil {
		return set, err
	}
	if len(operations) > maxChangeSetMetadataBytes || len(resultData) > maxChangeSetMetadataBytes {
		return set, ErrLimit
	}
	set = ChangeSet{Version: 1, ID: id, CopyID: record.ID, Source: req.Source, WorkerRunID: record.WorkerRunID, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch, State: "draft", BaseManifestDigest: record.ManifestDigest, ResultManifestDigest: resultDigest, OperationsDigest: digest(operations), OperationCount: len(changes), BlobBytes: blobBytes, CreatedAt: time.Now().UTC()}
	set.Validation = validation
	if err = s.reserveChangeSet(ctx, &record, set, requestDigest, operations, resultData); err != nil {
		return ChangeSet{}, err
	}
	defer func() {
		if resultErr != nil {
			set.State = "uncertain"
			_, _ = s.db.Exec(`UPDATE worker_change_sets SET state='uncertain' WHERE id=? AND state='draft'`, id)
		}
	}()
	if err = makeArtifactDirectory(s.root, record.ID+"/artifacts"); err != nil {
		return set, err
	}
	artifactPath := record.ID + "/artifacts/" + id
	if err = s.root.Mkdir(artifactPath, 0700); err != nil {
		return set, err
	}
	target, err := s.root.OpenRoot(artifactPath)
	if err != nil {
		return set, err
	}
	defer target.Close()
	seen := map[string]string{}
	for _, change := range changes {
		entry := change.Result
		if entry == nil || entry.Kind != "file" || seen[entry.SHA256] != "" {
			continue
		}
		chunks, copyErr := copyChangeBlob(ctx, root, target, *entry)
		if copyErr != nil {
			return set, copyErr
		}
		chunkData, _ := json.Marshal(chunks)
		if _, err = s.db.ExecContext(ctx, `INSERT INTO worker_change_blobs(set_id,hash,chunks) VALUES(?,?,?)`, id, entry.SHA256, chunkData); err != nil {
			return set, err
		}
		seen[entry.SHA256] = digest(chunkData)
	}
	after, err := inventoryWorkingCopy(ctx, root, inventoryRequest, s.policy)
	if err != nil {
		return set, err
	}
	afterDigest, _, err := workingCopyManifestDigest(after)
	if err != nil {
		return set, err
	}
	if afterDigest != resultDigest {
		return set, ErrRevision
	}
	if _, err = s.measureLocked(ctx, record.ID, req.Source); err != nil {
		return set, err
	}
	set.State = "sealed"
	for i := range changes {
		if result := changes[i].Result; result != nil && result.Kind == "file" {
			changes[i].ChunksDigest = seen[result.SHA256]
		}
	}
	operations, _ = json.Marshal(changes)
	if len(operations) > maxChangeSetMetadataBytes {
		return set, ErrLimit
	}
	set.OperationsDigest = digest(operations)
	header, _ := json.Marshal(set)
	if _, err = s.db.ExecContext(ctx, `UPDATE worker_change_sets SET state='sealed',header=?,operations=? WHERE id=? AND state='draft'`, header, operations, id); err != nil {
		return set, err
	}
	return set, nil
}

func (s *WorkingCopies) reserveChangeSet(ctx context.Context, record *WorkingCopyRecord, set ChangeSet, requestDigest string, operations, manifest []byte, base ...[]byte) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	var used int64
	if err = tx.QueryRow(`SELECT COUNT(*) FROM worker_change_sets WHERE copy_id=?`, record.ID).Scan(&count); err != nil {
		return err
	}
	if err = tx.QueryRow(`SELECT COALESCE(SUM(reserved_bytes),0) FROM worker_copies WHERE state!='released'`).Scan(&used); err != nil {
		return err
	}
	if count >= 16 || used < 0 || set.BlobBytes < 0 || record.ReservedBytes < 0 || record.ArtifactReservedBytes < 0 || record.ContentBytes < 0 || record.ArtifactReservedBytes > math.MaxInt64-record.ContentBytes || set.BlobBytes > math.MaxInt64-record.ContentBytes-record.ArtifactReservedBytes {
		return ErrLimit
	}
	// The producer has closed. Its unused content allowance can now hold the
	// sealed artifacts, without charging that same capacity twice. Unknown
	// prior artifact reservations remain charged until explicit release.
	needed := max(record.ReservedBytes, record.ContentBytes+record.ArtifactReservedBytes+set.BlobBytes)
	if used > s.policy.MaxTotalBytes || needed-record.ReservedBytes > s.policy.MaxTotalBytes-used {
		return ErrLimit
	}
	record.ReservedBytes = needed
	record.ArtifactReservedBytes += set.BlobBytes
	record.UsageMeasured = false
	data, _ := json.Marshal(record)
	header, _ := json.Marshal(set)
	if _, err = tx.Exec(`INSERT INTO worker_change_sets(id,copy_id,request_digest,state,header,operations,result_manifest) VALUES(?,?,?,'draft',?,?,?)`, set.ID, record.ID, requestDigest, header, operations, manifest); err != nil {
		return err
	}
	if len(base) == 1 {
		if _, err = tx.Exec(`INSERT INTO worker_change_bases(set_id,manifest) VALUES(?,?)`, set.ID, base[0]); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`UPDATE worker_copies SET reserved_bytes=?,record=? WHERE id=? AND state='ready'`, record.ReservedBytes, data, record.ID); err != nil {
		return err
	}
	return tx.Commit()
}

func makeArtifactDirectory(root *os.Root, name string) error {
	if err := root.Mkdir(name, 0700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrPath
	}
	return nil
}
