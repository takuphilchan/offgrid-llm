package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// References pin content, not authority. The host still requires the same live
// source authority and exact pinned versions before preparing a candidate.
// Earlier attempts may supply retained content under the same live folder grant.
type ChangeSetReference struct {
	SourceJob          string `json:"source_job,omitempty"`
	ID                 string `json:"id"`
	OperationsDigest   string `json:"operations_digest"`
	InstructionVersion uint64 `json:"instruction_version"`
	ControlEpoch       uint64 `json:"control_epoch"`
}

const maxCandidatePrerequisites = 128

func validateCandidateReferences(refs []ChangeSetReference, req WorkingCopyRequest) error {
	if err := validateIntegrationReferences(req.Integrated); err != nil {
		return err
	}
	if len(refs) > maxCandidatePrerequisites {
		return ErrLimit
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		if !validCopyID(ref.ID) || !validCopyID(ref.OperationsDigest) || ref.SourceJob != "" && !validIdentity(ref.SourceJob) || ref.InstructionVersion != req.InstructionVersion || ref.ControlEpoch > req.ControlEpoch || seen[ref.ID] {
			return ErrRevision
		}
		seen[ref.ID] = true
	}
	return nil
}

// Build the final inventory before reserving/copying bytes. Files come from
// either the approved original or immutable sealed blobs; a worker never sees
// another worker's live directory. The store lock protects retained artifacts.
func (s *WorkingCopies) candidateManifest(ctx context.Context, base *WorkingCopyManifest, req WorkingCopyRequest) (*WorkingCopyManifest, map[string]string, error) {
	applied, err := s.integratedPrerequisites(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	usedApplied := []ChangeSetReference{}
	entries := map[string]WorkingCopyEntry{}
	for _, entry := range base.Entries {
		entries[entry.Path] = entry
	}
	blobs := map[string]string{}
	visited, active := map[string]ChangeSetReference{}, map[string]bool{}
	ordered := []ChangeSetReference{}
	metadataBytes := 0
	var visit func(ChangeSetReference) error
	visit = func(ref ChangeSetReference) error {
		if ref.SourceJob == "" {
			ref.SourceJob = req.Parent.Job
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if old, ok := visited[ref.ID]; ok {
			if old != ref {
				return ErrRevision
			}
			return nil
		}
		if active[ref.ID] || len(active)+len(visited) >= maxCandidatePrerequisites {
			return ErrLimit
		}
		active[ref.ID] = true
		owner := req.Parent
		owner.Job = ref.SourceJob
		set, changes, err := s.changeSet(ctx, ref.ID, owner)
		if err != nil {
			return err
		}
		if proof, ok := applied[ref.ID]; ok {
			if proof != ref || set.OperationsDigest != ref.OperationsDigest || set.InstructionVersion != ref.InstructionVersion || set.ControlEpoch != ref.ControlEpoch {
				return ErrRevision
			}
			// These effects are already in the original folder. Preserve the
			// current inventory, including subsequent user edits, as the input.
			delete(active, ref.ID)
			visited[ref.ID] = ref
			ordered = append(ordered, ref)
			usedApplied = append(usedApplied, ref)
			return nil
		}
		if set.State != "sealed" && set.State != "ready" || set.OperationsDigest != ref.OperationsDigest || set.InstructionVersion != ref.InstructionVersion || set.ControlEpoch != ref.ControlEpoch {
			return ErrRevision
		}
		var data []byte
		var operationsBytes int
		if err = s.db.QueryRowContext(ctx, `SELECT COALESCE(b.manifest,c.manifest),length(operations) FROM worker_copies c JOIN worker_change_sets s ON s.copy_id=c.id LEFT JOIN worker_change_bases b ON b.set_id=s.id WHERE s.id=?`, set.ID).Scan(&data, &operationsBytes); err != nil {
			return err
		}
		metadataBytes += len(data) + operationsBytes
		if metadataBytes > maxChangeSetMetadataBytes || digest(data) != set.BaseManifestDigest {
			return ErrRevision
		}
		var input WorkingCopyManifest
		if json.Unmarshal(data, &input) != nil || input.Source != owner || input.InstructionVersion != ref.InstructionVersion || input.ControlEpoch != ref.ControlEpoch {
			return ErrRevision
		}
		if err = validateCandidateReferences(input.Prerequisites, req); err != nil {
			return err
		}
		for _, prerequisite := range input.Prerequisites {
			if prerequisite.SourceJob == "" {
				prerequisite.SourceJob = input.Source.Job
			}
			if err = visit(prerequisite); err != nil {
				return err
			}
		}
		for _, change := range changes {
			if !validCandidatePath(change.Path) {
				return ErrPath
			}
			old, exists := entries[change.Path]
			if (change.Base == nil && exists) || (change.Base != nil && (!exists || old != *change.Base)) {
				return fmt.Errorf("candidate preimage conflicts at %s: %w", change.Path, ErrConflict)
			}
			delete(blobs, change.Path)
			if change.Result == nil {
				delete(entries, change.Path)
				continue
			}
			entry := *change.Result
			if entry.Path != change.Path || !validCandidateEntry(entry) {
				return ErrRevision
			}
			entries[change.Path] = entry
			if entry.Kind == "file" {
				blobs[change.Path] = set.CopyID + "/artifacts/" + set.ID + "/" + entry.SHA256
			}
		}
		delete(active, ref.ID)
		visited[ref.ID] = ref
		ordered = append(ordered, ref)
		return nil
	}
	for _, ref := range req.Prerequisites {
		if err := visit(ref); err != nil {
			return nil, nil, err
		}
	}
	result := *base
	result.Entries, result.Bytes, result.Prerequisites = []WorkingCopyEntry{}, 0, ordered
	result.AppliedPrerequisites = usedApplied
	if len(entries) > s.policy.MaxEntries {
		return nil, nil, ErrLimit
	}
	for _, entry := range entries {
		if entry.Bytes > req.MaxBytes-result.Bytes {
			return nil, nil, ErrLimit
		}
		result.Bytes += entry.Bytes
		if parent := path.Dir(entry.Path); parent != "." && entries[parent].Kind != "directory" {
			return nil, nil, ErrConflict
		}
		result.Entries = append(result.Entries, entry)
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Path < result.Entries[j].Path })
	if err := requireWorkingCopyPaths(&result, req.RequiredPaths); err != nil {
		return nil, nil, err
	}
	return &result, blobs, nil
}

func validCandidatePath(name string) bool {
	if name == "." || !fs.ValidPath(name) || strings.ContainsAny(name, "\\:\x00") || len(name) > 4096 || strings.Count(name, "/") > 128 {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == ".git" {
			return false
		}
	}
	return true
}

func validCandidateEntry(entry WorkingCopyEntry) bool {
	return entry.Mode & ^uint32(0777) == 0 && ((entry.Kind == "file" && entry.Bytes >= 0 && validCopyID(entry.SHA256)) || (entry.Kind == "directory" && entry.Bytes == 0 && entry.SHA256 == ""))
}
