package execution

import (
	"context"
	"encoding/json"
)

const artifactOperation = "supervisor_artifact"

func dispatchWorkerArtifact(ctx context.Context, endpoint HostEndpoint, r *Request) (Response, error) {
	broker, ok := endpoint.(WorkerArtifactEndpoint)
	if !ok {
		return Response{}, ErrUnsupported
	}
	if r == nil || r.Operation != artifactOperation || r.Approved || r.Version != ProtocolVersion {
		return Response{}, ErrProtocol
	}
	var req WorkerArtifactRequest
	if decodeStrict(r.Arguments, &req) != nil || req.Source != r.Binding {
		return Response{}, ErrAuthority
	}
	reply, err := broker.WorkerArtifact(ctx, req)
	if err != nil {
		return Response{}, err
	}
	for {
		data, err := json.Marshal(reply)
		if err != nil {
			return Response{}, err
		}
		if len(data) <= MaxFrameBytes-4096 {
			return Response{Version: ProtocolVersion, ID: r.ID, Receipt: &Receipt{State: "complete", Result: data}}, nil
		}
		if reply.Integration != nil && len(reply.Integration.Files) > 1 {
			reply.Integration.Files = reply.Integration.Files[:len(reply.Integration.Files)/2]
			next := int(req.Offset) + len(reply.Integration.Files)
			reply.Integration.NextOffset = &next
			continue
		}
		if reply.Validation != nil && len(reply.Validation.Checks) > 1 {
			reply.Validation.Checks = reply.Validation.Checks[:len(reply.Validation.Checks)/2]
			next := int(req.Offset) + len(reply.Validation.Checks)
			reply.Validation.NextOffset = &next
			continue
		}
		if reply.Recovery != nil && len(reply.Recovery.Files) > 2 {
			// Recovery cursors count integration steps, not artifact entries.
			// Keep both possible retained files for a step on the same page.
			end := len(reply.Recovery.Files) / 2
			next, _, err := recoveryArtifactID(reply.Recovery.Files[end].ID)
			if err != nil {
				return Response{}, err
			}
			for end > 0 {
				previous, _, err := recoveryArtifactID(reply.Recovery.Files[end-1].ID)
				if err != nil {
					return Response{}, err
				}
				if previous != next {
					break
				}
				end--
			}
			if end == 0 {
				return Response{}, ErrLimit
			}
			reply.Recovery.Files = reply.Recovery.Files[:end]
			reply.Recovery.NextOffset = &next
			continue
		}
		if reply.Page == nil || len(reply.Page.Operations) <= 1 {
			return Response{}, ErrLimit
		}
		reply.Page.Operations = reply.Page.Operations[:len(reply.Page.Operations)/2]
		next := int(req.Offset) + len(reply.Page.Operations)
		reply.Page.NextOffset = &next
	}
}

func artifactRequest(req WorkerArtifactRequest) (Request, error) {
	if err := req.validate(); err != nil {
		return Request{}, err
	}
	id, err := opaqueToken()
	if err != nil {
		return Request{}, err
	}
	data, err := json.Marshal(req)
	return Request{Version: ProtocolVersion, ID: id, OperationID: id, Binding: req.Source, Operation: artifactOperation, Arguments: data}, err
}

func decodeArtifactResponse(response Response) (WorkerArtifactReply, error) {
	if response.Error != "" {
		return WorkerArtifactReply{}, executionTransportError(response.Error)
	}
	if response.Receipt == nil || response.Receipt.State != "complete" {
		return WorkerArtifactReply{}, ErrProtocol
	}
	var reply WorkerArtifactReply
	err := decodeStrict(response.Receipt.Result, &reply)
	return reply, err
}

func (e *StdioEndpoint) WorkerArtifact(ctx context.Context, req WorkerArtifactRequest) (WorkerArtifactReply, error) {
	r, err := artifactRequest(req)
	if err != nil {
		return WorkerArtifactReply{}, err
	}
	response, err := e.Call(ctx, r)
	if err != nil {
		return WorkerArtifactReply{}, err
	}
	return decodeArtifactResponse(response)
}

func (p *Peer) WorkerArtifact(ctx context.Context, req WorkerArtifactRequest) (WorkerArtifactReply, error) {
	r, err := artifactRequest(req)
	if err != nil {
		return WorkerArtifactReply{}, err
	}
	reply, err := p.exchange(ctx, "artifact", req.Source, &r)
	if err != nil {
		return WorkerArtifactReply{}, err
	}
	if reply.Response == nil {
		return WorkerArtifactReply{}, ErrProtocol
	}
	return decodeArtifactResponse(*reply.Response)
}

func (h *Hosts) WorkerArtifact(ctx context.Context, actor, workspace string, ref Reference, req WorkerArtifactRequest) (WorkerArtifactReply, error) {
	parent, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return WorkerArtifactReply{}, err
	}
	expected := parent.config.Binding
	expected.Job = req.Source.Job
	if parent.parent != nil || expected != req.Source || parent.config.WorkerCopies.Version != 1 {
		return WorkerArtifactReply{}, ErrAuthority
	}
	if req.Operation == "seal" || req.Operation == "integrate" {
		h.mu.RLock()
		admitted := parent.jobs[req.Source.Job] && !parent.cancelled[req.Source.Job]
		h.mu.RUnlock()
		if !admitted {
			return WorkerArtifactReply{}, ErrAuthority
		}
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return WorkerArtifactReply{}, err
	}
	broker, ok := parent.endpoint.(WorkerArtifactEndpoint)
	if !ok {
		return WorkerArtifactReply{}, ErrUnsupported
	}
	return broker.WorkerArtifact(ctx, req)
}
