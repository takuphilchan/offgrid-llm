package rag

import (
	"context"
	"sync"
	"time"
)

const maxQueuedExtractions = 4
const extractionQueueTimeout = 30 * time.Second

// PDF, OCR and Office readers share one process-wide resource boundary, including
// conversation attachments and legacy ingestion. It is independent of inference:
// waiting for a reader never takes an inference slot or an index-write lock.
var sharedExtractionScheduler extractionScheduler

type extractionScheduler struct {
	once     sync.Once
	admitted chan struct{}
	active   chan struct{}
}

func extractionBusy() error {
	return &ParseError{Code: "document_busy", Message: "Other documents are being processed. The saved source is unchanged; retry it after the current work finishes."}
}

func (s *extractionScheduler) acquire(ctx context.Context, wait time.Duration) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.once.Do(func() {
		s.admitted = make(chan struct{}, maxQueuedExtractions+1)
		s.active = make(chan struct{}, 1)
	})
	select {
	case s.admitted <- struct{}{}:
	default:
		return nil, extractionBusy()
	}
	if err := acquireDocumentSlot(ctx, s.active, wait); err != nil {
		<-s.admitted
		return nil, err
	}
	var released sync.Once
	return func() { released.Do(func() { <-s.active; <-s.admitted }) }, nil
}

// A ready slot and an expired deadline may both win a select. Recheck after
// acquiring so expired work cannot begin parsing, load source bytes or index.
// The caller owns the slot only when this returns nil.
func acquireDocumentSlot(ctx context.Context, slot chan struct{}, wait time.Duration) error {
	queued, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	select {
	case slot <- struct{}{}:
		if queued.Err() == nil {
			return nil
		}
		<-slot
	case <-queued.Done():
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return extractionBusy()
}
