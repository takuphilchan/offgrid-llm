package execution

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Supervisor messages are accepted only by the private transport, not by
// Worker.Handle's model-operation dispatcher. The private credential is never
// present in a tool schema or in a process environment.
func (e *LocalEndpoint) supervisor(ctx context.Context, r Request) Response {
	response := Response{Version: ProtocolVersion, ID: r.ID}
	if err := r.validate(e.Worker.Token); err != nil {
		response.Error = errorCode(err)
		return response
	}
	if r.Operation == "supervisor_receipt" {
		var original Request
		if err := decodeStrict(r.Arguments, &original); err != nil || original.Binding != r.Binding {
			response.Error = ErrProtocol.Error()
			return response
		}
		result, err := e.InspectReceipt(ctx, original)
		if err != nil {
			response.Error = errorCode(err)
			return response
		}
		result.ID = r.ID
		return result
	}
	if err := decodeStrict(r.Arguments, &struct{}{}); err != nil {
		response.Error = errorCode(err)
		return response
	}
	var value any
	var err error
	switch r.Operation {
	case "supervisor_admit":
		err = e.Admit(ctx, r.Binding)
	case "supervisor_cancel":
		bounded, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err = e.Cancel(bounded, r.Binding)
	case "supervisor_info":
		value, err = e.Check(ctx, r.Binding)
	default:
		err = ErrUnsupported
	}
	if err != nil {
		response.Error = errorCode(err)
		return response
	}
	encoded, _ := json.Marshal(value)
	response.Receipt = &Receipt{State: "complete", Result: encoded}
	return response
}

func supervisorOperation(operation string) bool { return strings.HasPrefix(operation, "supervisor_") }
