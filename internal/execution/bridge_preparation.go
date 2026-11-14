package execution

import (
	"context"
	"encoding/json"
)

func dispatchWorkerPreparation(ctx context.Context, endpoint HostEndpoint, message BridgeMessage) (Response, error) {
	r := message.Request
	preparer, ok := endpoint.(WorkerPreparationEndpoint)
	if !ok {
		return Response{}, ErrUnsupported
	}
	if r == nil || r.Token != "" || r.Version != ProtocolVersion || r.Binding != message.Binding || r.Approved {
		return Response{}, ErrAuthority
	}
	var req WorkingCopyRequest
	if err := decodeStrict(r.Arguments, &req); err != nil || req.Parent != message.Binding {
		return Response{}, ErrProtocol
	}
	var status WorkerPreparationStatus
	var err error
	switch r.Operation {
	case "supervisor_worker_begin":
		status, err = preparer.BeginWorkerPreparation(ctx, req)
	case "supervisor_worker_inspect":
		status, err = preparer.InspectWorkerPreparation(ctx, req)
	case "supervisor_worker_cancel":
		status, err = preparer.CancelWorkerPreparation(ctx, req)
	default:
		err = ErrUnsupported
	}
	if err != nil {
		return Response{}, err
	}
	return preparationResponse(status, r.ID)
}

func preparationResponse(status WorkerPreparationStatus, id string) (Response, error) {
	data, err := json.Marshal(status)
	if err != nil {
		return Response{}, err
	}
	return Response{Version: ProtocolVersion, ID: id, Receipt: &Receipt{State: "complete", Result: data}}, nil
}

// Called with the peer lock held. A host reply can register only a narrowed
// child of this exact assignment; ordinary call replies cannot publish grants.
func (p *Peer) recordPreparedChild(message BridgeMessage, response *Response) error {
	if response == nil || response.Error != "" || response.Receipt == nil || response.Receipt.State != "complete" {
		return ErrProtocol
	}
	var status WorkerPreparationStatus
	var req WorkingCopyRequest
	if decodeStrict(response.Receipt.Result, &status) != nil || message.Request == nil || decodeStrict(message.Request.Arguments, &req) != nil || req.Parent != message.Binding {
		return ErrProtocol
	}
	switch status.State {
	case "accepted", "cancelling", "cancelled", "closed", "failed", "uncertain", "interrupted":
		if status.Context != nil {
			return ErrAuthority
		}
		return nil
	case "ready":
	default:
		return ErrProtocol
	}
	if status.Context == nil || status.Record == nil {
		return ErrProtocol
	}
	c, r := *status.Context, *status.Record
	b, parent := c.Binding, req.Parent
	if c.ProjectHostID != "" || c.Version != ProtocolVersion || c.Backend != "local" || c.Root == "" || c.Root == p.config.Root || c.Directory != "." || c.Isolation != (IsolationPolicy{}) || c.WorkerCopies != (WorkingCopyPolicy{}) || c.Permissions != req.Permissions || !PermissionsWithin(c.Permissions, p.config.Permissions) || b.Actor != parent.Actor || b.Workspace != parent.Workspace || b.Host != parent.Host || b.Session == parent.Session || !validIdentity(b.Session) || b.Grant == parent.Grant || !validCredential(b.Grant) || b.Revision != 1 || b.Job != req.WorkerRunID || r.Source != parent || r.WorkerRunID != req.WorkerRunID || r.State != "ready" || r.ReservedBytes != req.MaxBytes {
		return ErrAuthority
	}
	if previous, exists := p.children[b.Session]; exists {
		// Local cwd may have moved after the original preparation receipt.
		previous.Directory = "."
		if previous != c {
			return ErrConflict
		}
		return nil
	}
	if len(p.children) >= 128 {
		return ErrLimit
	}
	if p.children == nil {
		p.children = map[string]Context{}
	}
	p.children[b.Session] = c
	return nil
}

func (p *Peer) workerPreparation(ctx context.Context, req WorkingCopyRequest, operation string) (WorkerPreparationStatus, error) {
	intent := preparationIntent(req)
	intent.Operation = operation
	reply, err := p.exchange(ctx, "worker", req.Parent, &intent)
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	var status WorkerPreparationStatus
	if reply.Response == nil || reply.Response.Receipt == nil {
		return status, ErrProtocol
	}
	if err := decodeStrict(reply.Response.Receipt.Result, &status); err != nil {
		return status, err
	}
	return status, nil
}

func (p *Peer) BeginWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	return p.workerPreparation(ctx, req, "supervisor_worker_begin")
}
func (p *Peer) InspectWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	return p.workerPreparation(ctx, req, "supervisor_worker_inspect")
}
func (p *Peer) CancelWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	return p.workerPreparation(ctx, req, "supervisor_worker_cancel")
}
