package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// PreparedWorker owns an independent session, cwd, process tree and journal.
// Its files are a working copy, not a security sandbox for trusted native code.
type PreparedWorker struct {
	Record        WorkingCopyRecord
	Context       Context
	Endpoint      *LocalEndpoint
	store         *WorkingCopies
	once          sync.Once
	closeErr      error
	done          chan struct{}
	requestDigest string
	settleMu      sync.Mutex
	settled       bool
}

func (w *PreparedWorker) Close() error {
	w.once.Do(func() {
		w.closeErr = w.Endpoint.Close()
		if w.Record.Purpose == "test" {
			evidence, err := w.Endpoint.Worker.Journal.validationEvidence(context.Background(), w.Context.Binding, w.Record.ManifestDigest)
			if err == nil {
				err = w.store.recordValidation(context.Background(), w.Record, evidence)
			}
			w.closeErr = errors.Join(w.closeErr, err)
		}
		w.closeErr = errors.Join(w.closeErr, w.Endpoint.Worker.Session.Close(), w.Endpoint.Worker.Journal.Close())
		close(w.done)
	})
	if w.closeErr != nil || !w.Endpoint.Worker.Processes.cleanupConfirmed() {
		// Closing the endpoint can fail before Release reaches the store. Save
		// that uncertainty here without clearing the lease or refunding bytes.
		return errors.Join(ErrHostUnavailable, w.closeErr, w.store.recordWorkerUncertain(context.Background(), w.Record.ID, w.Record.Source))
	}
	// A failed settlement write may be retried only by this retained, closed
	// owner. Restart cannot manufacture this proof or clear the durable lease.
	w.settleMu.Lock()
	defer w.settleMu.Unlock()
	if w.settled {
		return nil
	}
	if err := w.store.settleWorkerUse(context.Background(), w.Record.ID, w.Record.Source); err != nil {
		return errors.Join(err, w.store.recordWorkerUncertain(context.Background(), w.Record.ID, w.Record.Source))
	}
	w.settled = true
	return nil
}

func (w *PreparedWorker) Release(ctx context.Context) error {
	if err := w.Close(); err != nil {
		return err
	}
	return w.store.Release(ctx, w.Record.ID, w.Record.Source)
}

// PrepareWorker is a trusted supervisor operation. Callers must provide a live,
// already admitted source binding and an explicitly configured host store. It
// has no model/HTTP route and never falls back to the original directory.
func (e *LocalEndpoint) PrepareWorker(ctx context.Context, req WorkingCopyRequest) (*PreparedWorker, error) {
	e.preparing.Lock()
	defer e.preparing.Unlock()
	if _, err := e.Worker.Session.Snapshot(ctx, req.Parent); err != nil {
		return nil, err
	}
	requestJSON, _ := json.Marshal(req)
	requestDigest := digest(requestJSON)
	e.mu.Lock()
	jobCtx, store, closed := e.contexts[req.Parent.Job], e.WorkingCopies, e.closed
	if closed || store == nil || jobCtx == nil || jobCtx.Err() != nil {
		e.mu.Unlock()
		return nil, ErrAuthority
	}
	for _, child := range e.prepared {
		if child.Record.Source == req.Parent && child.Record.WorkerRunID == req.WorkerRunID {
			e.mu.Unlock()
			select {
			case <-child.done:
				return nil, ErrConflict
			default:
			}
			if child.requestDigest == requestDigest {
				return child, nil
			}
			return nil, ErrConflict
		}
	}
	e.mu.Unlock()
	if closed || store == nil || jobCtx == nil || jobCtx.Err() != nil {
		return nil, ErrAuthority
	}
	copyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(jobCtx, cancel)
	defer stop()
	record, err := store.Prepare(copyCtx, e.Worker.Session, req)
	if err != nil {
		return nil, err
	}
	id, err := opaqueToken()
	if err != nil {
		return nil, err
	}
	grant, err := opaqueToken()
	if err != nil {
		return nil, err
	}
	b := req.Parent
	b.Job = req.WorkerRunID
	b.Session = "worker-" + id
	b.Grant = grant
	b.Revision = 1
	config := Context{Version: ProtocolVersion, Binding: b, Backend: "local", Root: store.contentPath(record.ID), Directory: ".", Permissions: req.Permissions}
	if err = store.beginWorkerUse(copyCtx, record.ID, req.Parent); err != nil {
		return nil, err
	}
	session, err := openSessionEnvironment(config, e.Worker.Session.environment)
	if err != nil {
		return nil, err
	}
	session.startProcess = e.Worker.Session.startProcess
	actual, err := session.root.Stat(".")
	if err != nil || record.rootInfo == nil || !os.SameFile(actual, record.rootInfo) {
		session.Close()
		return nil, ErrRevision
	}
	journalDir := filepath.Join(store.directory, record.ID, "journal")
	if err = EnsurePrivateDirectory(journalDir); err != nil {
		session.Close()
		return nil, err
	}
	journal, err := OpenJournal(journalDir)
	if err != nil {
		session.Close()
		return nil, err
	}
	journal.reserveReplacement = func(ctx context.Context, bytes int64) error {
		return store.reserveWriteRecovery(ctx, record.ID, req.Parent, bytes, req.MaxBytes)
	}
	token, err := opaqueToken()
	if err != nil {
		session.Close()
		journal.Close()
		return nil, err
	}
	worker := &Worker{Session: session, Journal: journal, Processes: NewProcessManager(session), Token: token, AuthorizeProcess: e.Worker.AuthorizeProcess}
	if req.Purpose == "test" {
		worker.inputManifestDigest = record.ManifestDigest
		worker.validateProcessInput = func(ctx context.Context) error { return store.checkValidationInputs(ctx, record) }
	}
	endpoint := NewLocalEndpoint(worker)
	endpoint.boundJob = b.Job
	prepared := &PreparedWorker{Record: record, Context: config, Endpoint: endpoint, store: store, done: make(chan struct{}), requestDigest: requestDigest}
	if err = endpoint.Admit(copyCtx, b); err != nil {
		prepared.Close()
		return nil, err
	}
	e.mu.Lock()
	if e.closed || jobCtx.Err() != nil || copyCtx.Err() != nil {
		e.mu.Unlock()
		prepared.Close()
		return nil, ErrAuthority
	}
	if e.prepared == nil {
		e.prepared = map[string]*PreparedWorker{}
	}
	e.prepared[b.Session] = prepared
	e.mu.Unlock()
	go func() {
		select {
		case <-e.Worker.Session.revokedSignal:
			prepared.Close()
		case <-jobCtx.Done():
			prepared.Close()
		case <-prepared.done:
		}
	}()
	return prepared, nil
}
