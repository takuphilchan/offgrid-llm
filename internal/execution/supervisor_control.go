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
	if r.Operation == artifactOperation {
		result, err := dispatchWorkerArtifact(ctx, e, &r)
		if err != nil {
			response.Error = errorCode(err)
			return response
		}
		return result
	}
	if strings.HasPrefix(r.Operation, "supervisor_worker_") {
		var request WorkingCopyRequest
		if err := decodeStrict(r.Arguments, &request); err != nil || request.Parent != r.Binding {
			response.Error = ErrProtocol.Error()
			return response
		}
		var status WorkerPreparationStatus
		var err error
		switch r.Operation {
		case "supervisor_worker_begin":
			status, err = e.BeginWorkerPreparation(ctx, request)
		case "supervisor_worker_inspect":
			status, err = e.InspectWorkerPreparation(ctx, request)
		case "supervisor_worker_cancel":
			status, err = e.CancelWorkerPreparation(ctx, request)
		default:
			err = ErrUnsupported
		}
		if err != nil {
			response.Error = errorCode(err)
			return response
		}
		data, _ := json.Marshal(status)
		response.Receipt = &Receipt{State: "complete", Result: data}
		return response
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
	case "supervisor_project_context":
		value, err = e.ProjectContext(ctx, r.Binding)
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
