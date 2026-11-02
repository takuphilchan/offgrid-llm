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
	queued, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	select {
	case s.active <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-s.active
			<-s.admitted
			return nil, err
		}
		var released sync.Once
		return func() { released.Do(func() { <-s.active; <-s.admitted }) }, nil
	case <-queued.Done():
		<-s.admitted
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, extractionBusy()
	}
}
