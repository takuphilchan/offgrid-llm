package execution

import (
	"context"
	"encoding/json"
)

// WorkerPreparationStatus is a private supervisor reply. The live context is
// deliberately excluded from durable receipts: restart cannot restore grants.
type WorkerPreparationStatus struct {
	State   string             `json:"state"`
	Record  *WorkingCopyRecord `json:"record,omitempty"`
	Context *Context           `json:"context,omitempty"`
	Error   string             `json:"error,omitempty"`
}

type WorkerPreparationEndpoint interface {
	BeginWorkerPreparation(context.Context, WorkingCopyRequest) (WorkerPreparationStatus, error)
	InspectWorkerPreparation(context.Context, WorkingCopyRequest) (WorkerPreparationStatus, error)
	CancelWorkerPreparation(context.Context, WorkingCopyRequest) (WorkerPreparationStatus, error)
}

type workerPreparation struct {
	parent Binding
	digest string
	status WorkerPreparationStatus
	worker *PreparedWorker
	cancel context.CancelFunc
	done   chan struct{}
}

func preparationIntent(req WorkingCopyRequest) Request {
	identity, _ := json.Marshal(struct {
		Parent Binding
		ID     string
	}{req.Parent, req.RequestID})
	payload, _ := json.Marshal(req)
	return Request{Version: ProtocolVersion, ID: req.RequestID, OperationID: "preparation-" + digest(identity), Binding: req.Parent, Operation: "supervisor_worker_begin", Arguments: payload}
}

// childEndpoint only routes exact, currently owned child bindings. Knowing a
// root grant or substituting a different run never admits work on a child.
func (e *LocalEndpoint) childEndpoint(b Binding) (*LocalEndpoint, error) {
	if b.Session == e.Worker.Session.config.Binding.Session {
		return nil, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	child := e.prepared[b.Session]
	if e.closed || child == nil || child.Context.Binding != b {
		return nil, ErrAuthority
	}
	return child.Endpoint, nil
}

// Begin persists acceptance before launching preparation outside the private
// request loop. A retry observes that claim, including an uncertain restart;
// neither inspection nor reconnect launches a replacement preparation.
func (e *LocalEndpoint) BeginWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	req.RequiredPaths = append([]string(nil), req.RequiredPaths...)
	req.Prerequisites = append([]ChangeSetReference(nil), req.Prerequisites...)
	req.Integrated = append([]IntegrationReference(nil), req.Integrated...)
	if _, err := e.Worker.Session.Snapshot(ctx, req.Parent); err != nil {
		return WorkerPreparationStatus{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	job := e.contexts[req.Parent.Job]
	if e.closed || e.WorkingCopies == nil || job == nil || job.Err() != nil {
		return WorkerPreparationStatus{}, ErrAuthority
	}
	if err := validateWorkingCopyRequest(req, e.WorkingCopies.policy); err != nil {
		return WorkerPreparationStatus{}, err
	}
	if !PermissionsWithin(req.Permissions, e.Worker.Session.config.Permissions) {
		return WorkerPreparationStatus{}, ErrAuthority
	}
	intent := preparationIntent(req)
	if existing := e.preparations[intent.OperationID]; existing != nil {
		return preparationSnapshot(existing, intent)
	}
	// Bound even failed commands and queued work within one supervisor lifetime.
	if len(e.preparations) >= 128 {
		return WorkerPreparationStatus{}, ErrLimit
	}
	if err := e.Worker.initialize(); err != nil {
		return WorkerPreparationStatus{}, err
	}
	receipt, fresh, err := e.Worker.Journal.Claim(ctx, intent.OperationID, intent.Payload())
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	if !fresh {
		return recoveredPreparation(receipt)
	}
	copyCtx, cancel := context.WithCancel(job)
	p := &workerPreparation{parent: req.Parent, digest: digest(intent.Payload()), status: WorkerPreparationStatus{State: "accepted"}, cancel: cancel, done: make(chan struct{})}
	if e.preparations == nil {
		e.preparations = map[string]*workerPreparation{}
	}
	e.preparations[intent.OperationID] = p
	e.preparationWG.Add(1)
	go e.runPreparation(copyCtx, req, intent, p)
	return p.status, nil
}

func preparationSnapshot(p *workerPreparation, intent Request) (WorkerPreparationStatus, error) {
	if p.digest != digest(intent.Payload()) {
		return WorkerPreparationStatus{}, ErrConflict
	}
	status := p.status
	if p.worker != nil && status.State == "ready" {
		select {
		case <-p.worker.done:
			status.State = "closed"
		default:
			p.worker.Endpoint.mu.Lock()
			cancelled := p.worker.Endpoint.contexts[p.worker.Context.Binding.Job].Err() != nil
			p.worker.Endpoint.mu.Unlock()
			if cancelled {
				status.State = "closed"
			} else {
				live := p.worker.Context
				status.Context = &live
			}
		}
	}
	return status, nil
}

func recoveredPreparation(receipt Receipt) (WorkerPreparationStatus, error) {
	status := WorkerPreparationStatus{State: "uncertain"}
	if receipt.State == "complete" && len(receipt.Result) != 0 {
		if err := decodeStrict(receipt.Result, &status); err != nil {
			return WorkerPreparationStatus{}, err
		}
		status.Context = nil
		if status.State == "ready" {
			status.State = "interrupted"
		}
	}
	return status, nil
}

func (e *LocalEndpoint) runPreparation(ctx context.Context, req WorkingCopyRequest, intent Request, p *workerPreparation) {
	defer e.preparationWG.Done()
	defer close(p.done)
	defer p.cancel()
	worker, err := e.PrepareWorker(ctx, req)
	status := WorkerPreparationStatus{State: "ready"}
	if err != nil {
		status.State, status.Error = "failed", errorCode(err)
	}
	if worker != nil {
		record := worker.Record
		status.Record = &record
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx.Err() != nil {
		status.State, status.Error = "cancelled", ""
		if worker != nil {
			worker.Close()
		}
	}
	// Durable completion describes artifacts only; never the live child grant.
	data, _ := json.Marshal(status)
	if err := e.Worker.Journal.Finish(context.Background(), intent.OperationID, intent.Payload(), Receipt{State: "complete", Result: data}); err != nil {
		if worker != nil {
			worker.Close()
		}
		status.State, status.Error = "uncertain", errorCode(err)
	}
	p.worker, p.status = worker, status
}

func (e *LocalEndpoint) InspectWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	if _, err := e.Worker.Session.Snapshot(ctx, req.Parent); err != nil {
		return WorkerPreparationStatus{}, err
	}
	intent := preparationIntent(req)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return WorkerPreparationStatus{}, ErrHostUnavailable
	}
	if p := e.preparations[intent.OperationID]; p != nil {
		return preparationSnapshot(p, intent)
	}
	receipt, err := e.Worker.Journal.Inspect(ctx, intent)
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	return recoveredPreparation(receipt)
}

func (e *LocalEndpoint) CancelWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	if _, err := e.Worker.Session.Snapshot(ctx, req.Parent); err != nil {
		return WorkerPreparationStatus{}, err
	}
	intent := preparationIntent(req)
	e.mu.Lock()
	if e.closed || e.WorkingCopies == nil || e.contexts[req.Parent.Job] == nil || validateWorkingCopyRequest(req, e.WorkingCopies.policy) != nil || !PermissionsWithin(req.Permissions, e.Worker.Session.config.Permissions) {
		e.mu.Unlock()
		return WorkerPreparationStatus{}, ErrAuthority
	}
	p := e.preparations[intent.OperationID]
	if p == nil {
		// A Stop that wins against Begin leaves a durable tombstone. A late
		// Begin observes cancellation and cannot launch a new copy operation.
		if err := e.Worker.initialize(); err != nil {
			e.mu.Unlock()
			return WorkerPreparationStatus{}, err
		}
		receipt, fresh, err := e.Worker.Journal.Claim(ctx, intent.OperationID, intent.Payload())
		if err != nil {
			e.mu.Unlock()
			return WorkerPreparationStatus{}, err
		}
		if fresh {
			status := WorkerPreparationStatus{State: "cancelled"}
			data, _ := json.Marshal(status)
			err = e.Worker.Journal.Finish(ctx, intent.OperationID, intent.Payload(), Receipt{State: "complete", Result: data})
			e.mu.Unlock()
			return status, err
		}
		e.mu.Unlock()
		return recoveredPreparation(receipt)
	}
	if p.digest != digest(intent.Payload()) {
		e.mu.Unlock()
		return WorkerPreparationStatus{}, ErrConflict
	}
	// Cancelling preparation does not stop a successfully published worker.
	// That is a separate child-binding Cancel command with process receipts.
	if p.status.State != "accepted" && p.status.State != "cancelling" {
		status, err := preparationSnapshot(p, intent)
		e.mu.Unlock()
		return status, err
	} else {
		p.cancel()
		p.status = WorkerPreparationStatus{State: "cancelling"}
		e.mu.Unlock()
		return WorkerPreparationStatus{State: "cancelling"}, nil
	}
}
