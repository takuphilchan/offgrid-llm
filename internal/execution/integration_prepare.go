package execution

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
)

func integrationSteps(changes []FileChange) ([]integrationStep, int64, error) {
	ordered := []FileChange{}
	var bytes int64
	for _, change := range changes {
		if !validCandidatePath(change.Path) || change.Base == nil && change.Result == nil {
			return nil, 0, ErrRevision
		}
		for _, entry := range []*WorkingCopyEntry{change.Base, change.Result} {
			if entry == nil {
				continue
			}
			if entry.Path != change.Path || !validCandidateEntry(*entry) || entry.Bytes > math.MaxInt64-bytes {
				return nil, 0, ErrRevision
			}
			bytes += entry.Bytes
		}
		if change.Base != nil && change.Result != nil && change.Base.Kind != change.Result.Kind {
			ordered = append(ordered, FileChange{Path: change.Path, Kind: "delete", Base: change.Base}, FileChange{Path: change.Path, Kind: "create", Result: change.Result, ChunksDigest: change.ChunksDigest})
		} else {
			ordered = append(ordered, change)
		}
	}
	// Remove children before parents, then create parents before children. File
	// type changes have two separately journaled steps and may be partial.
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if (a.Kind == "delete") != (b.Kind == "delete") {
			return a.Kind == "delete"
		}
		if a.Kind == "delete" && strings.Count(a.Path, "/") != strings.Count(b.Path, "/") {
			return strings.Count(a.Path, "/") > strings.Count(b.Path, "/")
		}
		return a.Path < b.Path
	})
	steps := make([]integrationStep, len(ordered))
	for i, change := range ordered {
		steps[i] = integrationStep{Ordinal: i, Change: change}
	}
	return steps, bytes, nil
}

func (s *WorkingCopies) reserveIntegration(ctx context.Context, req IntegrateChangeSetRequest, id, payloadDigest string, set ChangeSet, steps []integrationStep, extra int64) error {
	if err := s.refreshCopiesLocked(ctx); err != nil {
		return err
	}
	record, err := s.inspect(ctx, set.CopyID)
	if err != nil {
		return err
	}
	if record.State != "ready" || extra < 0 || record.ContentBytes < 0 || record.ArtifactReservedBytes < 0 || record.ArtifactReservedBytes > math.MaxInt64-record.ContentBytes || extra > math.MaxInt64-record.ContentBytes-record.ArtifactReservedBytes {
		return ErrLimit
	}
	needed := max(record.ReservedBytes, record.ContentBytes+record.ArtifactReservedBytes+extra)
	if needed > req.MaxReservedBytes {
		return ErrLimit
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var used int64
	if err = tx.QueryRow(`SELECT COALESCE(SUM(reserved_bytes),0) FROM worker_copies WHERE state!='released'`).Scan(&used); err != nil {
		return err
	}
	if used > s.policy.MaxTotalBytes || needed-record.ReservedBytes > s.policy.MaxTotalBytes-used {
		return ErrLimit
	}
	record.ReservedBytes = needed
	record.ArtifactReservedBytes += extra
	record.UsageMeasured = false
	data, _ := json.Marshal(record)
	request, _ := json.Marshal(req)
	if _, err = tx.Exec(`INSERT INTO worker_integrations(id,set_id,request_digest,request,state,created_at) VALUES(?,?,?,?,'applying',?)`, id, set.ID, payloadDigest, request, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	for _, step := range steps {
		encoded, _ := json.Marshal(step)
		if _, err = tx.Exec(`INSERT INTO worker_integration_files(integration_id,ordinal,state,step) VALUES(?,?,'pending',?)`, id, step.Ordinal, encoded); err != nil {
			return err
		}
	}
	set.IntegrationStarted = true
	set.State = "applying"
	header, _ := json.Marshal(set)
	result, err := tx.Exec(`UPDATE worker_change_sets SET state='applying',header=? WHERE id=? AND state='ready'`, header, set.ID)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return ErrConflict
	}
	if err = integrationRowChanged(tx.Exec(`UPDATE worker_copies SET reserved_bytes=?,record=? WHERE id=? AND state='ready'`, record.ReservedBytes, data, record.ID)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *WorkingCopies) preflightIntegration(ctx context.Context, root *os.Root, set ChangeSet) error {
	var encoded []byte
	if err := s.db.QueryRowContext(ctx, `SELECT manifest FROM worker_change_bases WHERE set_id=?`, set.ID).Scan(&encoded); err != nil {
		return err
	}
	var base WorkingCopyManifest
	if len(encoded) > maxChangeSetMetadataBytes || digest(encoded) != set.BaseManifestDigest || json.Unmarshal(encoded, &base) != nil || base.Source != set.Source || base.InstructionVersion != set.InstructionVersion || base.ControlEpoch != set.ControlEpoch {
		return ErrRevision
	}
	actual, err := inventoryWorkingCopy(ctx, root, WorkingCopyRequest{Parent: set.Source, MaxBytes: s.policy.MaxCopyBytes}, s.policy)
	if err != nil {
		return err
	}
	sort.Slice(base.Entries, func(i, j int) bool { return base.Entries[i].Path < base.Entries[j].Path })
	sort.Slice(actual.Entries, func(i, j int) bool { return actual.Entries[i].Path < actual.Entries[j].Path })
	before, after := map[string]WorkingCopyEntry{}, map[string]WorkingCopyEntry{}
	for _, entry := range base.Entries {
		before[entry.Path] = entry
	}
	for _, entry := range actual.Entries {
		after[entry.Path] = entry
	}
	for _, entry := range base.Entries {
		if after[entry.Path] != entry {
			return &integrationConflict{path: entry.Path}
		}
	}
	for _, entry := range actual.Entries {
		if before[entry.Path] != entry {
			return &integrationConflict{path: entry.Path}
		}
	}
	if !reflect.DeepEqual(base.Exclusions, actual.Exclusions) {
		return &integrationConflict{path: ".git"}
	}
	return nil
}

func (s *WorkingCopies) stageIntegrationStep(ctx context.Context, set ChangeSet, stage *os.Root, step integrationStep) error {
	entry := step.Change.Result
	if entry == nil {
		return nil
	}
	if entry.Kind == "directory" {
		return stage.Mkdir("new-directory", os.FileMode(entry.Mode))
	}
	source, err := s.root.OpenRoot(set.CopyID + "/artifacts/" + set.ID)
	if err != nil {
		return err
	}
	defer source.Close()
	copyEntry := *entry
	copyEntry.Path = entry.SHA256
	if _, err = copyChangeBlob(ctx, source, stage, copyEntry); err != nil {
		return err
	}
	return stage.Chmod(entry.SHA256, os.FileMode(entry.Mode))
}
