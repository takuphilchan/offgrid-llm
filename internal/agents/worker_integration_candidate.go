package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// This projection is evidence, never authority to integrate. Dispatch must still
// check current instructions, control state, source access and original files.
type WorkerIntegrationCandidate struct {
	State                string `json:"state"`
	ChangeSetID          string `json:"change_set_id,omitempty"`
	OperationsDigest     string `json:"operations_digest,omitempty"`
	OperationCount       int    `json:"operation_count,omitempty"`
	BaseManifestDigest   string `json:"base_manifest_digest,omitempty"`
	ResultManifestDigest string `json:"result_manifest_digest,omitempty"`
	Error                string `json:"error,omitempty"`
}

func sameIntegrationCandidate(a, b *WorkerIntegrationCandidate) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func capturedIntegrationCandidate(req execution.SealChangeSetRequest, childID string, reply execution.WorkerArtifactReply) (*WorkerIntegrationCandidate, error) {
	if reply.CandidateState == "" {
		if reply.Candidate != nil || reply.CandidateError != "" {
			return nil, ErrRunConflict
		}
		return nil, nil
	}
	if reply.CandidateState != "ready" && reply.CandidateState != "uncertain" {
		return nil, ErrRunConflict
	}
	projection := &WorkerIntegrationCandidate{State: reply.CandidateState, Error: reply.CandidateError}
	set := reply.Candidate
	if set == nil {
		if projection.State != "uncertain" {
			return nil, ErrRunConflict
		}
		return projection, nil
	}
	candidateReq := execution.IntegrationCandidateRequest(req)
	identity, _ := json.Marshal(struct {
		Source            execution.Binding
		CopyID, RequestID string
	}{req.Source, req.CopyID, candidateReq.RequestID})
	hash := sha256.Sum256(identity)
	if set.Version != 1 || set.ID != hex.EncodeToString(hash[:]) || set.CopyID != req.CopyID || set.Source != req.Source || set.WorkerRunID != childID || set.InstructionVersion != req.InstructionVersion || set.ControlEpoch != req.ControlEpoch || set.State != projection.State || set.IntegrationStarted || reply.ChangeSet == nil || set.Validation != reply.ChangeSet.Validation || set.Validation.State != "passed" || set.OperationCount < 1 || set.BlobBytes < 0 {
		return nil, ErrRunConflict
	}
	for _, value := range []string{set.OperationsDigest, set.BaseManifestDigest, set.ResultManifestDigest} {
		if len(value) != 64 {
			return nil, ErrRunConflict
		}
		if _, err := hex.DecodeString(value); err != nil {
			return nil, ErrRunConflict
		}
	}
	if projection.State == "ready" && (projection.Error != "" || set.ResultManifestDigest != set.Validation.InputManifestDigest) {
		return nil, ErrRunConflict
	}
	projection.ChangeSetID, projection.OperationsDigest, projection.OperationCount = set.ID, set.OperationsDigest, set.OperationCount
	projection.BaseManifestDigest, projection.ResultManifestDigest = set.BaseManifestDigest, set.ResultManifestDigest
	return projection, nil
}
