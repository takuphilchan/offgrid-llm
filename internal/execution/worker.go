package execution

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Worker receives an already approved session from its trusted supervisor.
// Process authorization is deliberately fail-closed until that supervisor has
// reviewed the invocation; remote metadata and model prose are not policy.
type Worker struct {
	Version             string
	Session             *Session
	Journal             *Journal
	Processes           *ProcessManager
	Token               string
	AuthorizeProcess    func(context.Context, Binding, ProcessSpec, bool) error
	finishing           sync.WaitGroup
	initializing        sync.Once
	initializationError error
}

type fileArguments struct {
	Path     string `json:"path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
	Revision string `json:"revision"`
}

type operation func() (any, error)

func (w *Worker) prepare(ctx context.Context, r Request) (operation, bool, error) {
	switch r.Operation {
	case "workspace_info":
		if err := decodeStrict(r.Arguments, &struct{}{}); err != nil {
			return nil, false, err
		}
		return func() (any, error) { return w.Session.Snapshot(ctx, r.Binding) }, false, nil
	case "directory_change":
		var a struct {
			Path string `json:"path"`
		}
		if err := decodeStrict(r.Arguments, &a); err != nil {
			return nil, false, err
		}
		return func() (any, error) { return w.Session.ChangeDirectory(ctx, r.Binding, a.Path) }, true, nil
	case "file_list", "file_read":
		var a fileArguments
		if err := decodeStrict(r.Arguments, &a); err != nil {
			return nil, false, err
		}
		if a.Limit == 0 {
			if r.Operation == "file_list" {
				a.Limit = MaxEntriesPerPage
			} else {
				a.Limit = MaxPageBytes
			}
		}
		return func() (any, error) {
			if r.Operation == "file_list" {
				return w.Session.List(ctx, r.Binding, a.Path, a.Offset, a.Limit, a.Revision)
			}
			return w.Session.Read(ctx, r.Binding, a.Path, a.Offset, a.Limit, a.Revision)
		}, false, nil
	case "file_search":
		var a struct {
			Path  string `json:"path"`
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := decodeStrict(r.Arguments, &a); err != nil {
			return nil, false, err
		}
		if a.Limit == 0 {
			a.Limit = 32
		}
		return func() (any, error) { return w.Session.Search(ctx, r.Binding, a.Path, a.Query, a.Limit) }, false, nil
	case "file_write":
		var a struct {
			Path     string `json:"path"`
			Data     []byte `json:"data"`
			Revision string `json:"revision"`
		}
		if err := decodeStrict(r.Arguments, &a); err != nil {
			return nil, false, err
		}
		return func() (any, error) { return w.Session.Write(ctx, r.Binding, a.Path, a.Data, a.Revision) }, true, nil
	case "process_start":
		var a ProcessSpec
		if err := decodeStrict(r.Arguments, &a); err != nil {
			return nil, false, err
		}
		if w.AuthorizeProcess == nil {
			return nil, false, ErrAuthority
		}
		if err := w.AuthorizeProcess(ctx, r.Binding, a, r.Approved); err != nil {
			return nil, false, err
		}
		return func() (any, error) { return w.Processes.start(ctx, r.Binding, a, &r) }, true, nil
	case "process_inspect":
		var a struct {
			Handle string `json:"handle"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := decodeStrict(r.Arguments, &a); err != nil {
			return nil, false, err
		}
		if a.Limit == 0 {
			a.Limit = MaxPageBytes
		}
		return func() (any, error) { return w.Processes.Output(ctx, r.Binding, a.Handle, a.Offset, a.Limit) }, false, nil
	case "process_wait", "process_stop":
		var a struct {
			Handle     string `json:"handle"`
			WaitMillis int    `json:"wait_ms"`
		}
		if err := decodeStrict(r.Arguments, &a); err != nil {
			return nil, false, err
		}
		if a.WaitMillis < 0 || a.WaitMillis > 1000 {
			return nil, false, ErrLimit
		}
		if a.WaitMillis == 0 {
			a.WaitMillis = 250
		}
		return func() (any, error) {
			bounded, cancel := context.WithTimeout(ctx, time.Duration(a.WaitMillis)*time.Millisecond)
			defer cancel()
			var err error
			if r.Operation == "process_stop" {
				_, err = w.Processes.Stop(bounded, r.Binding, a.Handle)
			} else {
				_, err = w.Processes.Wait(bounded, r.Binding, a.Handle)
			}
			if err != nil && !errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			return w.Processes.Output(ctx, r.Binding, a.Handle, 0, MaxPageBytes)
		}, false, nil
	default:
		return nil, false, ErrUnsupported
	}
}

func errorCode(err error) string {
	if errors.Is(err, os.ErrExist) {
		return "execution_target_exists"
	}
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, exec.ErrDot) {
		return "execution_program_unavailable"
	}
	for _, known := range []error{ErrAuthority, ErrPath, ErrRevision, ErrLimit, ErrUnsupported, ErrConflict, ErrProtocol} {
		if errors.Is(err, known) {
			return known.Error()
		}
	}
	if errors.Is(err, context.Canceled) {
		return "execution_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "execution_timeout"
	}
	return "execution_operation_failed"
}

func (w *Worker) Handle(ctx context.Context, r Request) Response {
	response := Response{Version: ProtocolVersion, ID: r.ID}
	if err := r.validate(w.Token); err != nil {
		response.Error = errorCode(err)
		return response
	}
	if err := w.initialize(); err != nil {
		response.Error = errorCode(err)
		return response
	}
	if _, err := w.Session.Snapshot(ctx, r.Binding); err != nil {
		response.Error = errorCode(err)
		return response
	}
	perform, mutating, err := w.prepare(ctx, r)
	if err != nil {
		response.Error = errorCode(err)
		return response
	}
	payload := r.Payload()
	if mutating {
		receipt, fresh, err := w.Journal.Claim(ctx, r.OperationID, payload)
		if err != nil {
			response.Error = errorCode(err)
			return response
		}
		if !fresh {
			response.Receipt = &receipt
			return response
		}
	}
	value, operationErr := perform()
	receipt := Receipt{State: "complete"}
	if operationErr != nil {
		code := errorCode(operationErr)
		receipt.Result, _ = json.Marshal(map[string]string{"error": code})
		receipt.State = "rejected"
		var mutation *MutationError
		if errors.As(operationErr, &mutation) {
			receipt.State = mutation.Effects
			if receipt.State == "unknown" {
				receipt.State = "uncertain"
			}
		}
	} else {
		receipt.Result, err = json.Marshal(value)
		if err != nil {
			receipt.State = "uncertain"
		}
	}
	if mutating {
		if r.Operation == "directory_change" && operationErr == nil {
			current := value.(Context)
			if err = w.Journal.SaveDirectory(r.Binding.Session, current.Directory); err != nil {
				w.Session.Revoke()
				response.Error = "execution_receipt_unavailable"
				response.Receipt = &Receipt{State: "uncertain"}
				return response
			}
		}
		if r.Operation == "process_start" && operationErr == nil {
			receipt.State = "dispatched"
		}
		if err = w.Journal.Finish(context.WithoutCancel(ctx), r.OperationID, payload, receipt); err != nil {
			w.Session.Revoke()
			response.Receipt = &Receipt{State: "uncertain"}
			response.Error = "execution_receipt_unavailable"
			return response
		}
		if receipt.State == "dispatched" {
			p := value.(ProcessResult)
			w.finishing.Add(1)
			go func() {
				defer w.finishing.Done()
				result, err := w.Processes.Wait(context.Background(), r.Binding, p.Handle)
				if err == nil {
					encoded, _ := json.Marshal(result)
					state := "complete"
					if result.State == "uncertain" || result.ExitCode == nil {
						state = "uncertain"
						w.Session.Revoke()
					}
					if err = w.Journal.settle(context.Background(), r.OperationID, payload, encoded, state); err != nil {
						w.Session.Revoke()
					}
				}
			}()
		}
	}
	response.Receipt = &receipt
	return response
}

// Serve is supervised private stdio, not a public HTTP command endpoint. The
// parent owns the input stream and credential. EOF/cancellation stops children.
func (w *Worker) Serve(ctx context.Context, input io.ReadCloser, output io.Writer) error {
	stop := context.AfterFunc(ctx, func() { input.Close() })
	defer stop()
	endpoint := NewLocalEndpoint(w)
	defer endpoint.Close()
	if err := w.initialize(); err != nil {
		return err
	}
	if err := endpoint.Admit(ctx, w.Session.config.Binding); err != nil {
		return err
	}
	identity, _ := json.Marshal(workerIdentity(w.Version))
	if err := WriteFrame(output, Response{Version: ProtocolVersion, ID: "startup", Receipt: &Receipt{State: "ready", Result: identity}}); err != nil {
		return err
	}
	for {
		var request Request
		if err := ReadFrame(input, &request); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
		var response Response
		if supervisorOperation(request.Operation) {
			response = endpoint.supervisor(ctx, request)
		} else {
			// The endpoint adds admitted-job lifetime without removing the worker's
			// independent binding, private-token and durable intent checks.
			var err error
			if request.validate(w.Token) != nil {
				response = Response{Version: ProtocolVersion, ID: request.ID, Error: ErrAuthority.Error()}
			} else {
				response, err = endpoint.Call(ctx, request)
			}
			if err != nil {
				response = Response{Version: ProtocolVersion, ID: request.ID, Error: errorCode(err)}
			}
		}
		if err := WriteFrame(output, response); err != nil {
			return err
		}
	}
}

func (w *Worker) initialize() error {
	w.initializing.Do(func() { w.initializationError = w.Journal.RestoreSession(w.Session) })
	return w.initializationError
}
