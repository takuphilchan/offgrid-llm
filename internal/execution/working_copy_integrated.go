package execution

import (
	"context"
	"encoding/json"
)

// A receipt reference grants no access. It tells the supervisor which completed
// effects must not be overlaid again when constructing a retained worker input.
type IntegrationReference struct {
	ID        string `json:"id"`
	SourceJob string `json:"source_job"`
}

func validateIntegrationReferences(refs []IntegrationReference) error {
	if len(refs) > maxCandidatePrerequisites {
		return ErrLimit
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		if !validCopyID(ref.ID) || !validIdentity(ref.SourceJob) || seen[ref.ID] {
			return ErrRevision
		}
		seen[ref.ID] = true
	}
	return nil
}

// Caller holds the copy-store lock. Only a completed native journal, matching
// its immutable tested input manifest, can mark prerequisite effects applied.
func (s *WorkingCopies) integratedPrerequisites(ctx context.Context, req WorkingCopyRequest) (map[string]ChangeSetReference, error) {
	refs := map[string]ChangeSetReference{}
	metadataBytes := 0
	for _, proof := range req.Integrated {
		owner := req.Parent
		owner.Job = proof.SourceJob
		page, err := s.inspectIntegrationLocked(ctx, proof.ID, owner, 0, 1)
		if err != nil {
			return nil, err
		}
		if page.State != "integrated" || page.Total < 1 || page.Integrated != page.Total {
			return nil, ErrConflict
		}
		set, _, err := s.changeSet(ctx, page.ChangeSetID, owner)
		if err != nil {
			return nil, err
		}
		if set.State != "integrated" || set.Validation.State != "passed" || set.InstructionVersion != req.InstructionVersion || set.ControlEpoch > req.ControlEpoch {
			return nil, ErrRevision
		}
		var data []byte
		if err = s.db.QueryRowContext(ctx, `SELECT result_manifest FROM worker_change_sets WHERE id=?`, set.ID).Scan(&data); err != nil {
			return nil, err
		}
		metadataBytes += len(data)
		if metadataBytes > maxChangeSetMetadataBytes || digest(data) != set.ResultManifestDigest {
			return nil, ErrRevision
		}
		var input WorkingCopyManifest
		if json.Unmarshal(data, &input) != nil || input.Source != owner || input.InstructionVersion != set.InstructionVersion || input.ControlEpoch != set.ControlEpoch {
			return nil, ErrRevision
		}
		all := append([]ChangeSetReference{{ID: set.ID, SourceJob: owner.Job, OperationsDigest: set.OperationsDigest, InstructionVersion: set.InstructionVersion, ControlEpoch: set.ControlEpoch}}, input.Prerequisites...)
		for _, ref := range all {
			if ref.SourceJob == "" {
				ref.SourceJob = owner.Job
			}
			if old, exists := refs[ref.ID]; exists && old != ref {
				return nil, ErrRevision
			}
			refs[ref.ID] = ref
			if len(refs) > maxCandidatePrerequisites {
				return nil, ErrLimit
			}
		}
	}
	return refs, nil
}
