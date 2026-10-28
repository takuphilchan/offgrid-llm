package execution

import "context"

// DispatchBridgeMessage runs in the trusted local supervisor, never a renderer.
// Service messages are scoped operations, not local grants or executable code.
func DispatchBridgeMessage(ctx context.Context, endpoint HostEndpoint, approved Context, message BridgeMessage) BridgeReply {
	reply := BridgeReply{Version: ProtocolVersion, ID: message.ID}
	expected := approved.Binding
	expected.Job = message.Binding.Job
	if message.Version != ProtocolVersion || !validIdentity(message.ID) || !validIdentity(message.Binding.Job) || expected != message.Binding {
		reply.Error = ErrAuthority.Error()
		return reply
	}
	var err error
	switch message.Kind {
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
