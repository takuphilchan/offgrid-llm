package execution

import (
	"context"
	"sync"
)

// LocalEndpoint is the supervisor side of an already locally approved worker.
// The worker still validates its private credential, binding and journal. No
// HTTP/model payload may construct this endpoint or turn on process authority.
type LocalEndpoint struct {
	Worker   *Worker
	mu       sync.Mutex
	jobs     map[string]context.CancelFunc
	contexts map[string]context.Context
	closed   bool
	dispatch sync.Mutex
}

func NewLocalEndpoint(worker *Worker) *LocalEndpoint {
	return &LocalEndpoint{Worker: worker, jobs: map[string]context.CancelFunc{}, contexts: map[string]context.Context{}}
}

func (e *LocalEndpoint) Check(ctx context.Context, b Binding) (Context, error) {
	e.mu.Lock()
	closed := e.closed
	e.mu.Unlock()
	if closed {
		return Context{}, ErrHostUnavailable
	}
	return e.Worker.Session.Snapshot(ctx, b)
}

func (e *LocalEndpoint) Admit(ctx context.Context, b Binding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return ErrHostUnavailable
	}
	if err := e.Worker.Session.AdmitJob(b); err != nil {
		return err
	}
	if _, exists := e.jobs[b.Job]; exists {
		if e.contexts[b.Job].Err() != nil {
			return ErrAuthority
		}
		return nil
	}
	// Job lifetime is not one HTTP/tool request. A background process survives
	// completion of process_start, but never cancellation of its admitted job.
	jobctx, cancel := context.WithCancel(context.Background())
	e.jobs[b.Job] = cancel
	e.contexts[b.Job] = jobctx
	return nil
}

func (e *LocalEndpoint) Call(ctx context.Context, r Request) (Response, error) {
	e.dispatch.Lock()
	defer e.dispatch.Unlock()
	e.mu.Lock()
	jobctx := e.contexts[r.Binding.Job]
	closed := e.closed
	e.mu.Unlock()
	if closed || jobctx == nil {
		return Response{}, ErrAuthority
	}
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if err := jobctx.Err(); err != nil {
		return Response{}, err
	}
	operationctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(jobctx, cancel)
	defer cancel()
	defer stop()
	if r.Operation == "process_start" {
		operationctx = jobctx
	}
	r.Token = e.Worker.Token
	return e.Worker.Handle(operationctx, r), nil
}

func (e *LocalEndpoint) Cancel(ctx context.Context, b Binding) error {
	if _, err := e.Check(ctx, b); err != nil {
		return err
	}
	e.mu.Lock()
	cancel := e.jobs[b.Job]
	e.mu.Unlock()
	if cancel == nil {
		return ErrAuthority
	}
	cancel()
	return e.Worker.Processes.CancelJob(ctx, b)
}

func (e *LocalEndpoint) Close() error {
	e.mu.Lock()
	e.closed = true
	for _, cancel := range e.jobs {
		cancel()
	}
	e.mu.Unlock()
	e.Worker.Session.Revoke()
	// Revoke first, then settle the last private dispatch before waiting on
	// receipt goroutines. No concurrent Handle may add work after Wait begins.
	e.dispatch.Lock()
	defer e.dispatch.Unlock()
	err := e.Worker.Processes.Close()
	e.Worker.finishing.Wait()
	return err
}
