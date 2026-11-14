package execution

import (
	"context"
	"encoding/json"
)

// DispatchBridgeMessage runs in the trusted local supervisor, never a renderer.
// Service messages are scoped operations, not local grants or executable code.
func DispatchBridgeMessage(ctx context.Context, endpoint HostEndpoint, approved Context, message BridgeMessage) BridgeReply {
	reply := BridgeReply{Version: ProtocolVersion, ID: message.ID}
	expected := approved.Binding
	expected.Job = message.Binding.Job
	child := expected != message.Binding
	if message.Version != ProtocolVersion || !validIdentity(message.ID) || !validIdentity(message.Binding.Job) {
		reply.Error = ErrAuthority.Error()
		return reply
	}
	if child {
		if approved.WorkerCopies == (WorkingCopyPolicy{}) || message.Binding.Actor != expected.Actor || message.Binding.Workspace != expected.Workspace || message.Binding.Host != expected.Host || message.Binding.Session == expected.Session {
			reply.Error = ErrAuthority.Error()
			return reply
		}
		// Only the native endpoint can attest to its own prepared child grant.
		if current, err := endpoint.Check(ctx, message.Binding); err != nil || current.Binding != message.Binding {
			reply.Error = ErrAuthority.Error()
			return reply
		}
	}
	var err error
	switch message.Kind {
	case "project":
		reader, ok := endpoint.(ProjectContextEndpoint)
		r := message.Request
		if !ok || child || message.Binding != approved.Binding || r == nil || r.Binding != message.Binding || r.Version != ProtocolVersion || r.Token != "" || r.Operation != "supervisor_project_context" || r.Approved || decodeStrict(r.Arguments, &struct{}{}) != nil {
			err = ErrAuthority
			break
		}
		// The service's peer snapshot may precede the next heartbeat. Check
		// the native session now; a changed identity cannot relabel a project.
		current, checkErr := endpoint.Check(ctx, message.Binding)
		current.Directory = approved.Directory
		if checkErr != nil || current != approved {
			err = ErrAuthority
			break
		}
		var snapshot ProjectContextSnapshot
		snapshot, err = reader.ProjectContext(ctx, message.Binding)
		if err == nil {
			data, _ := json.Marshal(snapshot)
			reply.Response = &Response{Version: ProtocolVersion, ID: r.ID, Receipt: &Receipt{State: "complete", Result: data}}
		}
	case "artifact":
		if child || approved.WorkerCopies.Version != 1 || message.Request == nil || message.Request.Token != "" || message.Request.Binding != message.Binding {
			err = ErrAuthority
			break
		}
		var response Response
		response, err = dispatchWorkerArtifact(ctx, endpoint, message.Request)
		if err == nil {
			reply.Response = &response
		}
	case "worker":
		if child || approved.WorkerCopies == (WorkingCopyPolicy{}) {
			err = ErrAuthority
			break
		}
		var response Response
		response, err = dispatchWorkerPreparation(ctx, endpoint, message)
		if err == nil {
			reply.Response = &response
		}
	case "check":
		if message.Request != nil || message.Binding != approved.Binding {
			err = ErrAuthority
			break
		}
		var current Context
		current, err = endpoint.Check(ctx, message.Binding)
		if err == nil {
			reply.Snapshot = &current
		}
	case "admit":
		if message.Request != nil {
			err = ErrProtocol
			break
		}
		err = endpoint.Admit(ctx, message.Binding)
	case "cancel":
		if message.Request != nil {
			err = ErrProtocol
			break
		}
		err = endpoint.Cancel(ctx, message.Binding)
	case "call":
		request := message.Request
		if request == nil || request.Token != "" || request.Binding != message.Binding || request.Version != ProtocolVersion || supervisorOperation(request.Operation) {
			err = ErrAuthority
			break
		}
		var response Response
		response, err = endpoint.Call(ctx, *request)
		if err == nil {
			reply.Response = &response
		}
	case "receipt":
		request := message.Request
		reader, ok := endpoint.(ReceiptEndpoint)
		if !ok || request == nil || request.Token != "" || request.Binding != message.Binding || supervisorOperation(request.Operation) {
			err = ErrAuthority
			break
		}
		var response Response
		response, err = reader.InspectReceipt(ctx, *request)
		if err == nil {
			reply.Response = &response
		}
	default:
		err = ErrUnsupported
	}
	if err != nil {
		reply.Error = errorCode(err)
		reply.Snapshot = nil
		reply.Response = nil
	}
	return reply
}
