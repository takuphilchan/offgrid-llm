package execution

import (
	"context"
	"time"
)

// WorkerCopyPolicy observes a live host's explicit opt-in; it never admits work.
func (h *Hosts) WorkerCopyPolicy(ctx context.Context, actor, workspace string, ref Reference) (WorkingCopyPolicy, error) {
	parent, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return WorkingCopyPolicy{}, err
	}
	if parent.parent != nil || parent.config.WorkerCopies == (WorkingCopyPolicy{}) {
		return WorkingCopyPolicy{}, ErrUnsupported
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return WorkingCopyPolicy{}, err
	}
	return parent.config.WorkerCopies, nil
}

func (h *Hosts) preparationParent(ctx context.Context, actor, workspace string, ref Reference, req WorkingCopyRequest) (*hostRegistration, WorkerPreparationEndpoint, error) {
	parent, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return nil, nil, err
	}
	expected := parent.config.Binding
	expected.Job = req.Parent.Job
	h.mu.RLock()
	admitted := parent.jobs[expected.Job] && !parent.cancelled[expected.Job]
	h.mu.RUnlock()
	if parent.parent != nil || !admitted || req.Parent != expected || parent.config.WorkerCopies == (WorkingCopyPolicy{}) {
		return nil, nil, ErrAuthority
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return nil, nil, err
	}
	preparer, ok := parent.endpoint.(WorkerPreparationEndpoint)
	if !ok {
		return nil, nil, ErrUnsupported
	}
	return parent, preparer, nil
}

// These are service-controller operations, never model tools or GET handlers.
// The caller records its canonical assignment and capacity before beginning.
func (h *Hosts) BeginPreparedWorker(ctx context.Context, actor, workspace string, ref Reference, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	_, preparer, err := h.preparationParent(ctx, actor, workspace, ref, req)
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	return preparer.BeginWorkerPreparation(ctx, req)
}

func (h *Hosts) InspectPreparedWorker(ctx context.Context, actor, workspace string, ref Reference, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	_, preparer, err := h.preparationParent(ctx, actor, workspace, ref, req)
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	return preparer.InspectWorkerPreparation(ctx, req)
}

func (h *Hosts) CancelPreparedWorker(ctx context.Context, actor, workspace string, ref Reference, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	_, preparer, err := h.preparationParent(ctx, actor, workspace, ref, req)
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	return preparer.CancelWorkerPreparation(ctx, req)
}

// Stop one assignment even if it has not yet published a service registration.
// This never admits a child or revokes the parent/sibling job bindings.
func (h *Hosts) StopPreparedWorker(ctx context.Context, actor, workspace string, ref Reference, req WorkingCopyRequest) error {
	parent, preparer, err := h.preparationParent(ctx, actor, workspace, ref, req)
	if err != nil {
		return err
	}
	status, err := preparer.CancelWorkerPreparation(ctx, req)
	for err == nil {
		switch status.State {
		case "cancelled", "closed", "failed":
			return nil
		case "ready":
			intent := preparationIntent(req)
			response, encodeErr := preparationResponse(status, intent.ID)
			if encodeErr != nil {
				return encodeErr
			}
			validation := &Peer{config: parent.config}
			if err = validation.recordPreparedChild(BridgeMessage{Binding: req.Parent, Request: &intent}, &response); err != nil {
				return err
			}
			return parent.endpoint.Cancel(ctx, status.Context.Binding)
		case "accepted", "cancelling":
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
			}
			status, err = preparer.InspectWorkerPreparation(ctx, req)
		default:
			return ErrConflict
		}
	}
	return err
}

// AdmitPreparedWorker links only a native-attested, already prepared child.
// Inspection is separate: observing a receipt alone never registers a resource.
func (h *Hosts) AdmitPreparedWorker(ctx context.Context, actor, workspace string, ref Reference, req WorkingCopyRequest) (Resource, error) {
	parent, preparer, err := h.preparationParent(ctx, actor, workspace, ref, req)
	if err != nil {
		return Resource{}, err
	}
	status, err := preparer.InspectWorkerPreparation(ctx, req)
	if err != nil {
		return Resource{}, err
	}
	if status.State != "ready" || status.Context == nil {
		return Resource{}, ErrHostUnavailable
	}
	// Reuse the bridge validator for in-process and private stdio connections.
	validation := &Peer{config: parent.config}
	intent := preparationIntent(req)
	response, err := preparationResponse(status, intent.ID)
	if err != nil {
		return Resource{}, err
	}
	if err = validation.recordPreparedChild(BridgeMessage{Binding: req.Parent, Request: &intent}, &response); err != nil {
		return Resource{}, err
	}
	config := *status.Context
	current, err := parent.endpoint.Check(ctx, config.Binding)
	if err != nil {
		return Resource{}, err
	}
	// cwd is mutable; the original prepared binding/root/permissions are not.
	current.Directory = config.Directory
	if current != config {
		return Resource{}, ErrAuthority
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.entries[ref.Session] != parent || parent.cancelled[req.Parent.Job] {
		return Resource{}, ErrHostUnavailable
	}
	if existing := h.entries[config.Binding.Session]; existing != nil {
		if existing.parent != parent || existing.parentJob != req.Parent.Job || existing.config != config || existing.cancelled[config.Binding.Job] {
			return Resource{}, ErrConflict
		}
		return registeredResource(existing), nil
	}
	count := 0
	for _, entry := range h.entries {
		if entry.parent == parent {
			count++
		}
	}
	if count >= 128 {
		return Resource{}, ErrLimit
	}
	entry := &hostRegistration{config: config, parent: parent, parentJob: req.Parent.Job, jobs: map[string]bool{config.Binding.Job: true}, endpoint: &preparedHostEndpoint{parent: parent.endpoint, binding: config.Binding}}
	h.entries[config.Binding.Session] = entry
	return registeredResource(entry), nil
}

func registeredResource(e *hostRegistration) Resource {
	return resourceForContext(e.config)
}

func resourceForContext(c Context) Resource {
	return Resource{Reference: reference(c.Binding), Backend: c.Backend, Root: c.Root, Directory: c.Directory, Permissions: c.Permissions, PreparedWorkers: c.Backend == "local" && c.WorkerCopies.Version == 1, ProjectContext: c.Backend == "local" && c.Permissions.Read && validCredential(c.ProjectHostID)}
}

// A child registration borrows transport, not ownership of the parent host.
// Revoking one child must never close sibling sessions or the shared channel.
type preparedHostEndpoint struct {
	parent  HostEndpoint
	binding Binding
}

func (e *preparedHostEndpoint) Check(ctx context.Context, b Binding) (Context, error) {
	if b != e.binding {
		return Context{}, ErrAuthority
	}
	return e.parent.Check(ctx, b)
}
func (e *preparedHostEndpoint) Admit(ctx context.Context, b Binding) error {
	if b != e.binding {
		return ErrAuthority
	}
	return e.parent.Admit(ctx, b)
}
func (e *preparedHostEndpoint) Call(ctx context.Context, r Request) (Response, error) {
	if r.Binding != e.binding {
		return Response{}, ErrAuthority
	}
	return e.parent.Call(ctx, r)
}
func (e *preparedHostEndpoint) Cancel(ctx context.Context, b Binding) error {
	if b != e.binding {
		return ErrAuthority
	}
	return e.parent.Cancel(ctx, b)
}
func (e *preparedHostEndpoint) InspectReceipt(ctx context.Context, r Request) (Response, error) {
	if r.Binding != e.binding {
		return Response{}, ErrAuthority
	}
	reader, ok := e.parent.(ReceiptEndpoint)
	if !ok {
		return Response{}, ErrUnsupported
	}
	return reader.InspectReceipt(ctx, r)
}
func (e *preparedHostEndpoint) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return e.parent.Cancel(ctx, e.binding)
}
