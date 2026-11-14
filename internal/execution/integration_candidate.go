package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

func IntegrationCandidateRequest(req SealChangeSetRequest) SealChangeSetRequest {
	req.RequestID = "candidate-" + digest([]byte(req.RequestID))
	return req
}

// Reverse the exact, already combined prerequisite operations to recover the
// original folder baseline. No original bytes are read or changed by this step.
func (s *WorkingCopies) candidateSourceManifest(ctx context.Context, input *WorkingCopyManifest) (*WorkingCopyManifest, error) {
	entries := map[string]WorkingCopyEntry{}
	for _, entry := range input.Entries {
		entries[entry.Path] = entry
	}
	for i := len(input.Prerequisites) - 1; i >= 0; i-- {
		ref := input.Prerequisites[i]
		alreadyApplied := false
		for _, applied := range input.AppliedPrerequisites {
			if applied == ref {
				alreadyApplied = true
				break
			}
		}
		if alreadyApplied {
			continue
		}
		owner := input.Source
		if ref.SourceJob != "" {
			owner.Job = ref.SourceJob
		}
		set, changes, err := s.changeSet(ctx, ref.ID, owner)
		if err != nil {
			return nil, err
		}
		if set.State != "sealed" && set.State != "ready" || set.OperationsDigest != ref.OperationsDigest || set.InstructionVersion != ref.InstructionVersion || set.ControlEpoch != ref.ControlEpoch {
			return nil, ErrRevision
		}
		for _, change := range changes {
			actual, exists := entries[change.Path]
			if change.Result == nil && exists || change.Result != nil && (!exists || actual != *change.Result) {
				return nil, ErrRevision
			}
			if change.Base == nil {
				delete(entries, change.Path)
			} else {
				entries[change.Path] = *change.Base
			}
		}
	}
	base := *input
	base.Entries = []WorkingCopyEntry{}
	base.Prerequisites = nil
	base.AppliedPrerequisites = nil
	base.Bytes = 0
	for _, entry := range entries {
		base.Entries = append(base.Entries, entry)
		base.Bytes += entry.Bytes
	}
	sort.Slice(base.Entries, func(i, j int) bool { return base.Entries[i].Path < base.Entries[j].Path })
	return &base, nil
}

// A ready candidate contains the combined edits that were tested, excluding
// test-generated logs/caches. It does not apply changes or claim semantic success.
func (s *WorkingCopies) PrepareIntegrationCandidate(ctx context.Context, original SealChangeSetRequest) (candidate ChangeSet, resultErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return candidate, ErrHostUnavailable
	}
	sealID, sealDigest := changeSetIdentity(original)
	var storedSealDigest string
	if err := s.db.QueryRowContext(ctx, `SELECT request_digest FROM worker_change_sets WHERE id=?`, sealID).Scan(&storedSealDigest); err != nil {
		return candidate, err
	}
	if storedSealDigest != sealDigest {
		return candidate, ErrConflict
	}
	sealed, _, err := s.changeSet(ctx, sealID, original.Source)
	if err != nil {
		return candidate, err
	}
	if sealed.State != "sealed" || sealed.Validation.State != "passed" {
		return candidate, nil
	}
	req := IntegrationCandidateRequest(original)
	id, requestDigest := changeSetIdentity(req)
	var previous string
	if err = s.db.QueryRowContext(ctx, `SELECT request_digest FROM worker_change_sets WHERE id=?`, id).Scan(&previous); err == nil {
		if previous != requestDigest {
			return candidate, ErrConflict
		}
		candidate, _, err = s.changeSet(ctx, id, req.Source)
		return candidate, err
	} else if !errors.Is(err, sql.ErrNoRows) {
		return candidate, err
	}
	record, err := s.inspect(ctx, req.CopyID)
	if err != nil {
		return candidate, err
	}
	if record.Source != req.Source || record.State != "ready" || record.Purpose != "test" {
		return candidate, ErrAuthority
	}
	var inputData []byte
	if err = s.db.QueryRowContext(ctx, `SELECT manifest FROM worker_copies WHERE id=?`, record.ID).Scan(&inputData); err != nil {
		return candidate, err
	}
	var input WorkingCopyManifest
	if digest(inputData) != sealed.Validation.InputManifestDigest || json.Unmarshal(inputData, &input) != nil {
		return candidate, ErrRevision
	}
	if len(input.Prerequisites) == 0 {
		return candidate, nil
	}
	base, err := s.candidateSourceManifest(ctx, &input)
	if err != nil {
		return candidate, err
	}
	baseDigest, baseData, err := workingCopyManifestDigest(base)
	if err != nil {
		return candidate, err
	}
	// diffWorkingCopy expects observed private modes, while the admitted input
	// manifest deliberately retains original source modes.
	observed := input
	observed.Entries = append([]WorkingCopyEntry(nil), input.Entries...)
	for i := range observed.Entries {
		observed.Entries[i].Mode = workingCopyPrivateMode(observed.Entries[i])
	}
	changes, blobBytes, err := diffWorkingCopy(base, &observed)
	if err != nil {
		return candidate, err
	}
	if len(changes) == 0 {
		return candidate, nil
	}
	if err = s.refreshCopiesLocked(ctx); err != nil {
		return candidate, err
	}
	record, err = s.inspect(ctx, req.CopyID)
	if err != nil {
		return candidate, err
	}
	if req.MaxReservedBytes > 0 && (record.ContentBytes > req.MaxReservedBytes || record.ArtifactReservedBytes > req.MaxReservedBytes-record.ContentBytes || blobBytes > req.MaxReservedBytes-record.ContentBytes-record.ArtifactReservedBytes || record.ReservedBytes > req.MaxReservedBytes) {
		return candidate, ErrLimit
	}
	root, err := s.root.OpenRoot(record.ID + "/content")
	if err != nil {
		return candidate, err
	}
	defer root.Close()
	inventoryReq := WorkingCopyRequest{Parent: req.Source, MaxBytes: s.policy.MaxCopyBytes}
	actual, err := inventoryWorkingCopy(ctx, root, inventoryReq, s.policy)
	if err != nil {
		return candidate, err
	}
	if !validationInputsMatch(&input, actual) {
		return candidate, ErrRevision
	}
	operations, _ := json.Marshal(changes)
	if len(operations) > maxChangeSetMetadataBytes || len(baseData) > maxChangeSetMetadataBytes {
		return candidate, ErrLimit
	}
	candidate = ChangeSet{Version: 1, ID: id, CopyID: record.ID, Source: req.Source, WorkerRunID: record.WorkerRunID, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch, State: "draft", BaseManifestDigest: baseDigest, ResultManifestDigest: record.ManifestDigest, OperationsDigest: digest(operations), OperationCount: len(changes), BlobBytes: blobBytes, CreatedAt: time.Now().UTC(), Validation: sealed.Validation}
	if err = s.reserveChangeSet(ctx, &record, candidate, requestDigest, operations, inputData, baseData); err != nil {
		return ChangeSet{}, err
	}
	defer func() {
		if resultErr != nil {
			candidate.State = "uncertain"
			_, _ = s.db.Exec(`UPDATE worker_change_sets SET state='uncertain' WHERE id=? AND state IN ('draft','validating')`, id)
			if recorded, _, err := s.changeSet(context.Background(), id, req.Source); err == nil {
				candidate = recorded
			}
		}
	}()
	if _, err = s.db.ExecContext(ctx, `UPDATE worker_change_sets SET state='validating' WHERE id=? AND state='draft'`, id); err != nil {
		return candidate, err
	}
	if err = makeArtifactDirectory(s.root, record.ID+"/artifacts"); err != nil {
		return candidate, err
	}
	if err = s.root.Mkdir(record.ID+"/artifacts/"+id, 0700); err != nil {
		return candidate, err
	}
	target, err := s.root.OpenRoot(record.ID + "/artifacts/" + id)
	if err != nil {
		return candidate, err
	}
	defer target.Close()
	seen := map[string]string{}
	for i := range changes {
		entry := changes[i].Result
		if entry == nil || entry.Kind != "file" {
			continue
		}
		if seen[entry.SHA256] == "" {
			chunks, err := copyChangeBlob(ctx, root, target, *entry)
			if err != nil {
				return candidate, err
			}
			data, _ := json.Marshal(chunks)
			if _, err = s.db.ExecContext(ctx, `INSERT INTO worker_change_blobs(set_id,hash,chunks) VALUES(?,?,?)`, id, entry.SHA256, data); err != nil {
				return candidate, err
			}
			seen[entry.SHA256] = digest(data)
		}
		changes[i].ChunksDigest = seen[entry.SHA256]
	}
	actual, err = inventoryWorkingCopy(ctx, root, inventoryReq, s.policy)
	if err != nil {
		return candidate, err
	}
	if !validationInputsMatch(&input, actual) {
		return candidate, ErrRevision
	}
	if _, err = s.measureLocked(ctx, record.ID, req.Source); err != nil {
		return candidate, err
	}
	candidate.State = "ready"
	operations, _ = json.Marshal(changes)
	if len(operations) > maxChangeSetMetadataBytes {
		return candidate, ErrLimit
	}
	candidate.OperationsDigest = digest(operations)
	header, _ := json.Marshal(candidate)
	if _, err = s.db.ExecContext(ctx, `UPDATE worker_change_sets SET state='ready',header=?,operations=? WHERE id=? AND state='validating'`, header, operations, id); err != nil {
		return candidate, err
	}
	return candidate, nil
}
