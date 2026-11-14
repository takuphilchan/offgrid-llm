package execution

import (
	"context"
	"encoding/json"
	"errors"
)

func (e *StdioEndpoint) workerPreparation(ctx context.Context, req WorkingCopyRequest, operation string) (WorkerPreparationStatus, error) {
	id, err := opaqueToken()
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	data, err := json.Marshal(req)
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	response, err := e.Call(ctx, Request{Version: ProtocolVersion, ID: id, OperationID: id, Binding: req.Parent, Operation: operation, Arguments: data})
	if err != nil {
		return WorkerPreparationStatus{}, err
	}
	if response.Error != "" {
		return WorkerPreparationStatus{}, executionTransportError(response.Error)
	}
	if response.Receipt == nil || response.Receipt.State != "complete" {
		return WorkerPreparationStatus{}, ErrProtocol
	}
	var status WorkerPreparationStatus
	if err = decodeStrict(response.Receipt.Result, &status); err != nil {
		return WorkerPreparationStatus{}, err
	}
	return status, nil
}

func executionTransportError(code string) error {
	for _, known := range []error{ErrAuthority, ErrConflict, ErrLimit, ErrProtocol, ErrUnsupported, ErrHostUnavailable, ErrPath, ErrRevision} {
		if code == known.Error() {
			return known
		}
	}
	return errors.New(code)
}

func (e *StdioEndpoint) BeginWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	return e.workerPreparation(ctx, req, "supervisor_worker_begin")
}

func (e *StdioEndpoint) InspectWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	return e.workerPreparation(ctx, req, "supervisor_worker_inspect")
}

func (e *StdioEndpoint) CancelWorkerPreparation(ctx context.Context, req WorkingCopyRequest) (WorkerPreparationStatus, error) {
	return e.workerPreparation(ctx, req, "supervisor_worker_cancel")
}
