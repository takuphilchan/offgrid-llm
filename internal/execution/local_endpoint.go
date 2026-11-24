package execution

import (
	"context"
	"errors"
	"sync"
)

// LocalEndpoint is the supervisor side of an already locally approved worker.
// The worker still validates its private credential, binding and journal. No
// HTTP/model payload may construct this endpoint or turn on process authority.
type LocalEndpoint struct {
	Worker        *Worker
	mu            sync.Mutex
	jobs          map[string]context.CancelFunc
	contexts      map[string]context.Context
	closed        bool
	dispatch      sync.Mutex
	preparing     sync.Mutex
	WorkingCopies *WorkingCopies
	prepared      map[string]*PreparedWorker
	boundJob      string
	preparations  map[string]*workerPreparation
	seals         map[string]*workerSeal
	preparationWG sync.WaitGroup
}

func NewLocalEndpoint(worker *Worker) *LocalEndpoint {
	return &LocalEndpoint{Worker: worker, WorkingCopies: worker.WorkingCopies, jobs: map[string]context.CancelFunc{}, contexts: map[string]context.Context{}}
}

func (e *LocalEndpoint) Check(ctx context.Context, b Binding) (Context, error) {
	if child, err := e.childEndpoint(b); child != nil || err != nil {
		if err != nil {
			return Context{}, err
		}
		return child.Check(ctx, b)
	}
	e.mu.Lock()
	closed := e.closed
	e.mu.Unlock()
	if closed {
		return Context{}, ErrHostUnavailable
	}
	return e.Worker.Session.Snapshot(ctx, b)
}

func (e *LocalEndpoint) Admit(ctx context.Context, b Binding) error {
	if child, err := e.childEndpoint(b); child != nil || err != nil {
		if err != nil {
			return err
		}
		return child.Admit(ctx, b)
	}
	if e.boundJob != "" && e.boundJob != b.Job {
		return ErrAuthority
	}
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
	if child, err := e.childEndpoint(r.Binding); child != nil || err != nil {
		if err != nil {
			return Response{}, err
		}
		return child.Call(ctx, r)
	}
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
	if child, err := e.childEndpoint(b); child != nil || err != nil {
		if err != nil {
			return err
		}
		return child.Cancel(ctx, b)
	}
	if _, err := e.Check(ctx, b); err != nil {
		return err
	}
	e.mu.Lock()
	cancel := e.jobs[b.Job]
	children := []*PreparedWorker{}
	pending := []<-chan struct{}{}
	for _, preparation := range e.preparations {
		if preparation.parent == b {
			pending = append(pending, preparation.done)
		}
	}
	for _, operation := range e.seals {
		if operation.parent == b && operation.done != nil {
			pending = append(pending, operation.done)
		}
	}
	for _, child := range e.prepared {
		if child.Record.Source == b {
			children = append(children, child)
		}
	}
	e.mu.Unlock()
	if cancel == nil {
		return ErrAuthority
	}
	cancel()
	resultErr := e.Worker.Processes.CancelJob(ctx, b)
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return errors.Join(resultErr, ctx.Err())
		}
	}
	for _, child := range children {
		select {
		case <-child.done:
			resultErr = errors.Join(resultErr, child.completionErr)
		case <-ctx.Done():
			return errors.Join(resultErr, ctx.Err())
		}
	}
	return resultErr
}

func (e *LocalEndpoint) Close() error {
	e.mu.Lock()
	e.closed = true
	for _, cancel := range e.jobs {
		cancel()
	}
	children := make([]*PreparedWorker, 0, len(e.prepared))
	for _, child := range e.prepared {
		children = append(children, child)
	}
	e.mu.Unlock()
	e.Worker.Session.Revoke()
	e.preparationWG.Wait()
	// Revoke first, then settle the last private dispatch before waiting on
	// receipt goroutines. No concurrent Handle may add work after Wait begins.
	e.dispatch.Lock()
	defer e.dispatch.Unlock()
	err := e.Worker.Processes.Close()
	e.Worker.finishing.Wait()
	for _, child := range children {
		if childErr := child.Close(); childErr != nil && err == nil {
			err = childErr
		}
	}
	return err
}
