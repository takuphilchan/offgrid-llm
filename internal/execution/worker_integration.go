package execution

import (
	"context"
	"encoding/json"
)

func integrationIntent(req IntegrateChangeSetRequest) Request {
	id, _ := integrationIdentity(req)
	data, _ := json.Marshal(req)
	return Request{Version: ProtocolVersion, ID: id, OperationID: "integrate-" + id, Binding: req.Source, Operation: "supervisor_change_integration", Arguments: data}
}

func (e *LocalEndpoint) workerIntegration(ctx context.Context, req IntegrateChangeSetRequest, begin bool) (WorkerArtifactReply, error) {
	intent := integrationIntent(req)
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
	var recorded int
	if err := e.Worker.Journal.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM execution_operations WHERE id=?`, intent.OperationID).Scan(&recorded); err != nil {
		return WorkerArtifactReply{}, err
	}
	if recorded != 0 || !begin {
		receipt, err := e.Worker.Journal.Inspect(ctx, intent)
		if err != nil {
			return WorkerArtifactReply{}, err
		}
		if receipt.State == "complete" {
			return recoveredSeal(receipt)
		}
		// After restart only observe recorded per-file state. Never restart the
		// native integration because the outer acknowledgment was lost.
		if !e.WorkingCopies.mu.TryLock() {
			return WorkerArtifactReply{}, ErrConflict
		}
		defer e.WorkingCopies.mu.Unlock()
		id, _ := integrationIdentity(req)
		page, err := e.WorkingCopies.inspectIntegrationLocked(ctx, id, req.Source, 0, 64)
		if err != nil {
			return WorkerArtifactReply{State: "uncertain"}, nil
		}
		return WorkerArtifactReply{State: page.State, Integration: &page}, nil
	}
	job := e.contexts[req.Source.Job]
	if job == nil || job.Err() != nil || len(e.seals) >= 128 {
		return WorkerArtifactReply{}, ErrAuthority
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
	operationCtx, cancel := context.WithCancel(job)
	state := &workerSeal{digest: digest(intent.Payload()), reply: WorkerArtifactReply{State: "accepted"}, parent: req.Source, done: make(chan struct{}), cancel: cancel}
	if e.seals == nil {
		e.seals = map[string]*workerSeal{}
	}
	e.seals[intent.OperationID] = state
	e.preparationWG.Add(1)
	go func() {
		defer e.preparationWG.Done()
		defer close(state.done)
		defer cancel()
		page, err := e.WorkingCopies.IntegrateChangeSet(operationCtx, e.Worker.Session, req)
		reply := WorkerArtifactReply{State: page.State}
		if page.ID != "" {
			reply.Integration = &page
			if set, inspectErr := e.WorkingCopies.InspectChangeSet(context.Background(), req.ChangeSetID, req.Source, 0, 1); inspectErr == nil {
				if record, measureErr := e.WorkingCopies.Measure(context.Background(), set.ChangeSet.CopyID, req.Source); measureErr == nil {
					reply.Copy = &record
				}
			}
		}
		if err != nil {
			reply.Error = errorCode(err)
			if reply.State == "" {
				reply.State = "uncertain"
			}
		}
		data, _ := json.Marshal(reply)
		if err = e.Worker.Journal.Finish(context.Background(), intent.OperationID, intent.Payload(), Receipt{State: "complete", Result: data}); err != nil {
			reply.State = "uncertain"
			reply.Error = errorCode(err)
		}
		e.mu.Lock()
		state.reply = reply
		e.mu.Unlock()
	}()
	return state.reply, nil
}

func (e *LocalEndpoint) cancelWorkerIntegration(ctx context.Context, req IntegrateChangeSetRequest) (WorkerArtifactReply, error) {
	intent := integrationIntent(req)
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return WorkerArtifactReply{}, ErrHostUnavailable
	}
	state := e.seals[intent.OperationID]
	if state != nil {
		if state.digest != digest(intent.Payload()) || state.parent != req.Source || state.cancel == nil {
			e.mu.Unlock()
			return WorkerArtifactReply{}, ErrConflict
		}
		state.cancel()
		done := state.done
		e.mu.Unlock()
		select {
		case <-done:
		case <-ctx.Done():
			return WorkerArtifactReply{State: "uncertain"}, ctx.Err()
		}
		e.mu.Lock()
		reply := cloneArtifactReply(state.reply)
		e.mu.Unlock()
		return reply, nil
	}
	// A cancellation arriving before a delayed begin records a tombstone under
	// the exact same identity. A later begin cannot acquire a fresh dispatch.
	receipt, fresh, err := e.Worker.Journal.Claim(ctx, intent.OperationID, intent.Payload())
	if err != nil {
		e.mu.Unlock()
		return WorkerArtifactReply{}, err
	}
	if !fresh {
		e.mu.Unlock()
		return recoveredSeal(receipt)
	}
	reply := WorkerArtifactReply{State: "cancelled"}
	data, _ := json.Marshal(reply)
	err = e.Worker.Journal.Finish(ctx, intent.OperationID, intent.Payload(), Receipt{State: "complete", Result: data})
	e.mu.Unlock()
	return reply, err
}
