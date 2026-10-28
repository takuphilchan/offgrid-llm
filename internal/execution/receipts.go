package execution

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// NewIntent records an operation identity, not authority. Dispatch still checks
// the admitted job, pinned host grant, invocation policy and private channel.
func NewIntent(b Binding, call, operation string, arguments json.RawMessage, approved bool) (Request, error) {
	if !validIdentity(b.Job) || !validIdentity(call) || supervisorOperation(operation) || len(arguments) > MaxFrameBytes {
		return Request{}, ErrProtocol
	}
	var object map[string]json.RawMessage
	if decodeStrict(arguments, &object) != nil {
		return Request{}, ErrProtocol
	}
	identity, _ := json.Marshal([]string{b.Job, call})
	sum := sha256.Sum256(identity)
	return Request{Version: ProtocolVersion, ID: call, OperationID: hex.EncodeToString(sum[:]), Binding: b, Operation: operation, Arguments: arguments, Approved: approved && operation == "process_start"}, nil
}

// Inspect is observation-only. Absence is NOT evidence of no effects; callers
// must retain uncertainty rather than Claim or dispatch to reconstruct output.
func (j *Journal) Inspect(ctx context.Context, original Request) (Receipt, error) {
	var receipt Receipt
	var previous string
	var result []byte
	err := j.db.QueryRowContext(ctx, `SELECT digest,state,result FROM execution_operations WHERE id=?`, original.OperationID).Scan(&previous, &receipt.State, &result)
	if errors.Is(err, sql.ErrNoRows) {
		return Receipt{State: "uncertain"}, nil
	}
	if err != nil {
		return Receipt{}, err
	}
	if previous != digest(original.Payload()) {
		return Receipt{}, ErrConflict
	}
	receipt.Result = result
	if receipt.State == "claimed" {
		receipt.State = "uncertain"
	}
	return receipt, nil
}

type ReceiptEndpoint interface {
	InspectReceipt(context.Context, Request) (Response, error)
}

func (e *LocalEndpoint) InspectReceipt(ctx context.Context, original Request) (Response, error) {
	e.mu.Lock()
	closed := e.closed
	e.mu.Unlock()
	if closed {
		return Response{}, ErrHostUnavailable
	}
	current, err := e.Worker.Session.Snapshot(ctx, e.Worker.Session.config.Binding)
	if err != nil {
		return Response{}, err
	}
	expected := current.Binding
	expected.Job = original.Binding.Job
	if original.Token != "" || original.Binding != expected || !validIdentity(original.Binding.Job) {
		return Response{}, ErrAuthority
	}
	rebuilt, err := NewIntent(original.Binding, original.ID, original.Operation, original.Arguments, original.Approved)
	if err != nil || rebuilt.OperationID != original.OperationID || original.Version != ProtocolVersion {
		return Response{}, ErrProtocol
	}
	if err = e.Worker.initialize(); err != nil {
		return Response{}, err
	}
	receipt, err := e.Worker.Journal.Inspect(ctx, original)
	if err != nil {
		return Response{}, err
	}
	return Response{Version: ProtocolVersion, ID: original.ID, Receipt: &receipt}, nil
}

func (h *Hosts) InspectReceipt(ctx context.Context, actor, workspace string, ref Reference, original Request) (Response, error) {
	e, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return Response{}, err
	}
	b := e.config.Binding
	b.Job = original.Binding.Job
	if original.Binding != b || original.Token != "" {
		return Response{}, ErrAuthority
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return Response{}, err
	}
	reader, ok := e.endpoint.(ReceiptEndpoint)
	if !ok {
		return Response{}, ErrUnsupported
	}
	response, err := reader.InspectReceipt(ctx, original)
	if err != nil {
		return Response{}, err
	}
	if response.Version != ProtocolVersion || response.ID != original.ID || response.Error != "" || response.Receipt == nil {
		return Response{}, ErrProtocol
	}
	return response, nil
}
