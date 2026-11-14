package execution

import (
	"context"
	"encoding/json"
)

// Private supervisor protocol. These references never carry executable hooks,
// new grants, paths outside the owned store, or model-call authority.
type WorkerArtifactRequest struct {
	Integrate     *IntegrateChangeSetRequest `json:"integrate,omitempty"`
	IntegrationID string                     `json:"integration_id,omitempty"`
	Operation     string                     `json:"operation"`
	Source        Binding                    `json:"source"`
	Seal          *SealChangeSetRequest      `json:"seal,omitempty"`
	ChangeSetID   string                     `json:"change_set_id,omitempty"`
	Hash          string                     `json:"hash,omitempty"`
	ArtifactID    string                     `json:"artifact_id,omitempty"`
	Offset        int64                      `json:"offset,omitempty"`
	Limit         int                        `json:"limit,omitempty"`
}

type WorkerArtifactReply struct {
	Recovery       *IntegrationRecoveryPage `json:"recovery,omitempty"`
	Integration    *IntegrationPage         `json:"integration,omitempty"`
	Candidate      *ChangeSet               `json:"candidate,omitempty"`
	CandidateState string                   `json:"candidate_state,omitempty"`
	CandidateError string                   `json:"candidate_error,omitempty"`
	Validation     *ValidationPage          `json:"validation,omitempty"`
	Copy           *WorkingCopyRecord       `json:"copy,omitempty"`
	State          string                   `json:"state"`
	ChangeSet      *ChangeSet               `json:"change_set,omitempty"`
	Page           *ChangeSetPage           `json:"page,omitempty"`
	Blob           *ChangeBlobPage          `json:"blob,omitempty"`
	Error          string                   `json:"error,omitempty"`
}

type WorkerArtifactEndpoint interface {
	WorkerArtifact(context.Context, WorkerArtifactRequest) (WorkerArtifactReply, error)
}

func (r WorkerArtifactRequest) validate() error {
	if r.Operation != "preserved" && r.ArtifactID != "" {
		return ErrProtocol
	}
	if r.Operation != "integrate" && r.Operation != "integration_status" && r.Operation != "integration_cancel" && r.Operation != "integration" && r.Operation != "recovery" && r.Operation != "preserved" && (r.Integrate != nil || r.IntegrationID != "") {
		return ErrProtocol
	}
	switch r.Operation {
	case "integrate", "integration_status", "integration_cancel":
		if r.Integrate == nil || r.Integrate.Source != r.Source || !validIdentity(r.Integrate.RequestID) || !validCopyID(r.Integrate.ChangeSetID) || !validCopyID(r.Integrate.OperationsDigest) || r.Integrate.InstructionVersion == 0 || r.Integrate.MaxReservedBytes < 1 || r.IntegrationID != "" || r.Seal != nil || r.ChangeSetID != "" || r.Hash != "" || r.Offset != 0 || r.Limit != 0 {
			return ErrProtocol
		}
	case "preserved":
		if r.Integrate != nil || !validCopyID(r.IntegrationID) || r.Seal != nil || r.ChangeSetID != "" || !validCopyID(r.Hash) || r.Offset < 0 || r.Limit < 1 || r.Limit > MaxPageBytes {
			return ErrProtocol
		}
		if _, _, err := recoveryArtifactID(r.ArtifactID); err != nil {
			return err
		}
	case "integration", "recovery":
		if r.Integrate != nil || !validCopyID(r.IntegrationID) || r.Seal != nil || r.ChangeSetID != "" || r.Hash != "" || r.Offset < 0 || r.Offset > 200000 || r.Limit < 1 || r.Limit > 64 {
			return ErrProtocol
		}
	case "seal", "seal_status":
		if r.Seal == nil || r.Seal.MaxReservedBytes < 0 || r.Seal.Source != r.Source || !validCopyID(r.Seal.CopyID) || !validIdentity(r.Seal.RequestID) || r.ChangeSetID != "" || r.Hash != "" || r.Offset != 0 || r.Limit != 0 {
			return ErrProtocol
		}
	case "inspect", "blob", "validation":
		if r.Seal != nil || !validCopyID(r.ChangeSetID) || r.Offset < 0 || r.Limit < 1 {
			return ErrProtocol
		}
		if (r.Operation == "inspect" || r.Operation == "validation") && (r.Offset > 100000 || r.Limit > 64 || r.Hash != "") || r.Operation == "blob" && (!validCopyID(r.Hash) || r.Limit > MaxPageBytes) {
			return ErrLimit
		}
	default:
		return ErrUnsupported
	}
	return nil
}

func (e *LocalEndpoint) WorkerArtifact(ctx context.Context, req WorkerArtifactRequest) (out WorkerArtifactReply, resultErr error) {
	if err := req.validate(); err != nil {
		return WorkerArtifactReply{}, err
	}
	config, err := e.Worker.Session.Snapshot(ctx, req.Source)
	if err != nil {
		return WorkerArtifactReply{}, err
	}
	if req.Operation == "recovery" || req.Operation == "preserved" {
		if !config.Permissions.Read {
			return WorkerArtifactReply{}, ErrAuthority
		}
		defer func() {
			if resultErr == nil {
				if _, err := e.Worker.Session.Snapshot(ctx, req.Source); err != nil {
					out, resultErr = WorkerArtifactReply{}, err
				}
			}
		}()
	}
	e.mu.Lock()
	store, closed := e.WorkingCopies, e.closed
	e.mu.Unlock()
	if closed || store == nil {
		return WorkerArtifactReply{}, ErrHostUnavailable
	}
	if req.Operation == "inspect" || req.Operation == "blob" || req.Operation == "validation" || req.Operation == "integration" || req.Operation == "recovery" || req.Operation == "preserved" {
		// A large seal must not block the private transport's cancellation lane.
		if !store.mu.TryLock() {
			return WorkerArtifactReply{}, ErrConflict
		}
		defer store.mu.Unlock()
	}
	switch req.Operation {
	case "recovery":
		page, err := store.inspectRecoveryLocked(ctx, req.IntegrationID, req.Source, int(req.Offset), req.Limit)
		return WorkerArtifactReply{State: "observed", Recovery: &page}, err
	case "preserved":
		page, err := store.readRecoveryBlobLocked(ctx, req.IntegrationID, req.Source, req.ArtifactID, req.Hash, req.Offset, req.Limit)
		return WorkerArtifactReply{State: "observed", Blob: &page}, err
	case "integration_cancel":
		return e.cancelWorkerIntegration(ctx, *req.Integrate)
	case "integrate", "integration_status":
		return e.workerIntegration(ctx, *req.Integrate, req.Operation == "integrate")
	case "integration":
		page, err := store.inspectIntegrationLocked(ctx, req.IntegrationID, req.Source, int(req.Offset), req.Limit)
		return WorkerArtifactReply{State: page.State, Integration: &page}, err
	case "seal", "seal_status":
		return e.workerSeal(ctx, *req.Seal, req.Operation == "seal")
	case "inspect":
		page, err := store.inspectChangeSetLocked(ctx, req.ChangeSetID, req.Source, int(req.Offset), req.Limit)
		return WorkerArtifactReply{State: page.ChangeSet.State, Page: &page}, err
	case "blob":
		page, err := store.readChangeBlobLocked(ctx, req.ChangeSetID, req.Source, req.Hash, req.Offset, req.Limit)
		return WorkerArtifactReply{State: "sealed", Blob: &page}, err
	case "validation":
		page, err := store.inspectValidationLocked(ctx, req.ChangeSetID, req.Source, int(req.Offset), req.Limit)
		return WorkerArtifactReply{State: "sealed", Validation: &page}, err
	}
	return WorkerArtifactReply{}, ErrUnsupported
}

type workerSeal struct {
	parent Binding
	done   chan struct{}
	cancel context.CancelFunc
	digest string
	reply  WorkerArtifactReply
}

func sealIntent(req SealChangeSetRequest) Request {
	id, _ := changeSetIdentity(req)
	data, _ := json.Marshal(req)
	return Request{Version: ProtocolVersion, ID: id, OperationID: "seal-" + id, Binding: req.Source, Operation: "supervisor_change_seal", Arguments: data}
}

func (e *LocalEndpoint) workerSeal(ctx context.Context, req SealChangeSetRequest, begin bool) (WorkerArtifactReply, error) {
	intent := sealIntent(req)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return WorkerArtifactReply{}, ErrHostUnavailable
	}
	if previous := e.seals[intent.OperationID]; previous != nil {
		if previous.digest != digest(intent.Payload()) {
			return WorkerArtifactReply{}, ErrConflict
		}
		return cloneArtifactReply(previous.reply), nil
	}
	if !begin {
		receipt, err := e.Worker.Journal.Inspect(ctx, intent)
		if err != nil {
			return WorkerArtifactReply{}, err
		}
		return recoveredSeal(receipt)
	}
	var recorded int
	if err := e.Worker.Journal.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_operations WHERE id=?`, intent.OperationID).Scan(&recorded); err != nil {
		return WorkerArtifactReply{}, err
	}
	if recorded != 0 {
		receipt, err := e.Worker.Journal.Inspect(ctx, intent)
		if err != nil {
			return WorkerArtifactReply{}, err
		}
		return recoveredSeal(receipt)
	}
	job := e.contexts[req.Source.Job]
	if job == nil || job.Err() != nil || len(e.seals) >= 128 {
		return WorkerArtifactReply{}, ErrAuthority
	}
	var worker *PreparedWorker
	for _, candidate := range e.prepared {
		if candidate.Record.ID == req.CopyID && candidate.Record.Source == req.Source {
			worker = candidate
			break
		}
	}
	if worker == nil {
		return WorkerArtifactReply{}, ErrAuthority
	}
	var manifest []byte
	if err := e.WorkingCopies.db.QueryRowContext(ctx, `SELECT manifest FROM worker_copies WHERE id=?`, req.CopyID).Scan(&manifest); err != nil {
		return WorkerArtifactReply{}, err
	}
	var baseline WorkingCopyManifest
	if digest(manifest) != worker.Record.ManifestDigest || json.Unmarshal(manifest, &baseline) != nil || baseline.InstructionVersion != req.InstructionVersion || baseline.ControlEpoch != req.ControlEpoch {
		return WorkerArtifactReply{}, ErrRevision
	}
	if err := e.Worker.initialize(); err != nil {
		return WorkerArtifactReply{}, err
	}
	receipt, fresh, err := e.Worker.Journal.Claim(ctx, intent.OperationID, intent.Payload())
	if err != nil {
		return WorkerArtifactReply{}, err
	}
	if !fresh {
		return recoveredSeal(receipt)
	}
	state := &workerSeal{digest: digest(intent.Payload()), reply: WorkerArtifactReply{State: "accepted"}}
	if e.seals == nil {
		e.seals = map[string]*workerSeal{}
	}
	e.seals[intent.OperationID] = state
	e.preparationWG.Add(1)
	go func() {
		defer e.preparationWG.Done()
		set, err := worker.SealChangeSet(job, req)
		reply := WorkerArtifactReply{State: set.State, ChangeSet: &set}
		if err == nil && set.State == "sealed" {
			candidate, candidateErr := worker.store.PrepareIntegrationCandidate(job, req)
			if candidate.ID != "" {
				reply.Candidate = &candidate
				reply.CandidateState = candidate.State
			}
			if candidateErr != nil {
				reply.CandidateState = "uncertain"
				reply.CandidateError = errorCode(candidateErr)
			}
			var measured WorkingCopyRecord
			measured, err = worker.store.Measure(context.Background(), req.CopyID, req.Source)
			if err == nil {
				reply.Copy = &measured
			}
		}
		if err != nil {
			reply = WorkerArtifactReply{State: "uncertain", Error: errorCode(err)}
		}
		data, _ := json.Marshal(reply)
		if err = e.Worker.Journal.Finish(context.Background(), intent.OperationID, intent.Payload(), Receipt{State: "complete", Result: data}); err != nil {
			reply = WorkerArtifactReply{State: "uncertain", Error: errorCode(err)}
		}
		e.mu.Lock()
		state.reply = reply
		e.mu.Unlock()
	}()
	return state.reply, nil
}

func cloneArtifactReply(reply WorkerArtifactReply) WorkerArtifactReply {
	if reply.Integration != nil {
		value := *reply.Integration
		value.Files = append([]IntegrationFileReceipt{}, value.Files...)
		if value.NextOffset != nil {
			next := *value.NextOffset
			value.NextOffset = &next
		}
		reply.Integration = &value
	}
	if reply.Candidate != nil {
		value := *reply.Candidate
		reply.Candidate = &value
	}
	if reply.Copy != nil {
		value := *reply.Copy
		reply.Copy = &value
	}
	if reply.ChangeSet != nil {
		value := *reply.ChangeSet
		reply.ChangeSet = &value
	}
	return reply
}

func recoveredSeal(receipt Receipt) (WorkerArtifactReply, error) {
	if receipt.State != "complete" {
		return WorkerArtifactReply{State: "uncertain"}, nil
	}
	var reply WorkerArtifactReply
	if err := decodeStrict(receipt.Result, &reply); err != nil {
		return reply, err
	}
	return reply, nil
}
