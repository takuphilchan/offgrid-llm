package rag

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
)

type importExecution struct {
	mu      sync.Mutex
	gate    chan struct{}
	running map[string]context.CancelFunc
	closed  bool
	wait    sync.WaitGroup
}

func (e *Engine) ImportStore() (*SQLiteStore, error) {
	s, ok := e.store.(*SQLiteStore)
	if !ok || e.storageErr != nil {
		return nil, fmt.Errorf("durable Knowledge storage unavailable")
	}
	return s, nil
}

// RunImport is request-owned and bounded. Accepted bytes and progress outlive
// the request; disconnect/cancel requires an explicit retry of the saved import.
func (e *Engine) RunImport(parent context.Context, owner, id string) (doc *Document, err error) {
	s, err := e.ImportStore()
	if err != nil {
		return nil, err
	}
	r, err := s.Import(owner, id)
	if err != nil {
		return nil, err
	}
	if r.State == "ready" {
		doc = e.ForActor(owner).GetDocument(r.DocumentID)
		if doc == nil {
			return nil, ErrImportNotFound
		}
		return doc, nil
	}
	parent = context.WithValue(e.ForActor(owner).context(parent), collectionDestinationKey{}, r.CollectionID)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	control := &e.imports
	control.mu.Lock()
	if control.closed {
		control.mu.Unlock()
		return nil, context.Canceled
	}
	if control.running == nil {
		control.running = map[string]context.CancelFunc{}
		control.gate = make(chan struct{}, 1)
	}
	if _, active := control.running[id]; active {
		control.mu.Unlock()
		return nil, ErrImportConflict
	}
	if len(control.running) >= maxQueuedExtractions+1 {
		control.mu.Unlock()
		return nil, extractionBusy()
	}
	r, err = s.beginImport(owner, id)
	if err != nil {
		control.mu.Unlock()
		return nil, err
	}
	control.running[id] = cancel
	control.wait.Add(1)
	control.mu.Unlock()
	lease := importLease{id: id, owner: owner, attempt: r.Attempt, collection: r.CollectionID}
	defer func() {
		if err != nil {
			state, code := "failed", "document_index_failed"
			if errors.Is(err, ErrCollectionUnavailable) {
				code = "document_unavailable"
			}
			var problem *ParseError
			if errors.As(err, &problem) {
				code = problem.Code
				state = "needs_input"
				if code == "document_busy" {
					state = "failed"
				}
			}
			if errors.Is(err, context.Canceled) {
				state, code = "cancelled", "document_cancelled"
			}
			if errors.Is(err, context.DeadlineExceeded) {
				state, code = "failed", "document_timeout"
			}
			if saveErr := s.finishImportFailure(lease, state, code); saveErr != nil {
				err = errors.Join(err, fmt.Errorf("persist import outcome: %w", saveErr))
			}
		}
		control.mu.Lock()
		delete(control.running, id)
		control.mu.Unlock()
		control.wait.Done()
	}()
	queued, stopWaiting := context.WithTimeout(ctx, extractionQueueTimeout)
	defer stopWaiting()
	select {
	case control.gate <- struct{}{}:
		defer func() { <-control.gate }()
	case <-queued.Done():
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, extractionBusy()
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	// Acquire the shared reader before loading a retained source into memory.
	// Release it before waiting for embedding/index work, so Chat attachments
	// can continue while a library document is being indexed.
	var data []byte
	var parsed *ParseResult
	err = func() error {
		release, acquireErr := sharedExtractionScheduler.acquire(ctx, extractionQueueTimeout)
		if acquireErr != nil {
			return acquireErr
		}
		defer release()
		if stepErr := s.advanceImport(lease, "queued", "extracting", ""); stepErr != nil {
			return stepErr
		}
		var readErr error
		data, readErr = s.importBytes(lease, r)
		if readErr != nil {
			return readErr
		}
		parsed, readErr = e.DocumentParser().parseContext(ctx, data, r.Name, filepath.Ext(r.Name))
		return readErr
	}()
	if err != nil {
		if parsed != nil && parsed.State == ExtractionPartial && ctx.Err() == nil {
			if saveErr := s.saveImportExtraction(lease, parsed); saveErr != nil {
				return nil, errors.Join(saveErr, err)
			}
		}
		return nil, err
	}
	if err = s.advanceImport(lease, "extracting", "indexing", ""); err != nil {
		return nil, err
	}
	metadata := map[string]string{"original_filename": r.Name, "file_ext": filepath.Ext(r.Name), "content_type": parsed.ContentType}
	for key, value := range parsed.Metadata {
		metadata[key] = value
	}
	return e.ingestOriginal(ctx, r.Name, data, parsed, metadata, &lease)
}

func (e *Engine) StopImport(owner, id string, remove bool) error {
	s, err := e.ImportStore()
	if err != nil {
		return err
	}
	if _, err = s.Import(owner, id); err != nil {
		return err
	}
	e.imports.mu.Lock()
	defer e.imports.mu.Unlock()
	if err = s.stopImport(owner, id, remove); err != nil {
		return err
	}
	if cancel := e.imports.running[id]; cancel != nil {
		cancel()
	}
	return nil
}

func (e *Engine) closeImports() {
	e.imports.mu.Lock()
	e.imports.closed = true
	for _, cancel := range e.imports.running {
		cancel()
	}
	e.imports.mu.Unlock()
	e.imports.wait.Wait()
}
