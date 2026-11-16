package rag

import (
	"context"
	"errors"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

func (e *Engine) StartComposition(owner, id string, provider document.SynthesisLocalProvider) error {
	if provider == nil {
		return ErrAnalysisUnavailable
	}
	s, err := e.ImportStore()
	if err != nil {
		return err
	}
	c := &e.analyses
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return context.Canceled
	}
	if c.running == nil {
		c.running = map[string]context.CancelFunc{}
		c.gate = make(chan struct{}, 1)
	}
	key := "composition:" + id
	if _, exists := c.running[key]; exists {
		return ErrAnalysisConflict
	}
	if len(c.running) >= 4 {
		return ErrAnalysisLimit
	}
	r, lease, err := s.beginComposition(context.Background(), owner, id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.running[key] = cancel
	c.wait.Add(1)
	go func() {
		defer c.wait.Done()
		defer func() { cancel(); c.mu.Lock(); delete(c.running, key); c.mu.Unlock() }()
		err := func() error {
			select {
			case c.gate <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			defer func() { <-c.gate }()
			tr, err := s.compositionTraversal(ctx, owner, id)
			if err != nil {
				return err
			}
			guard := func() error {
				if err := ctx.Err(); err != nil {
					return err
				}
				current, err := s.Composition(ctx, owner, id)
				if err != nil {
					return err
				}
				if !current.ReviewsCurrent {
					return ErrCompositionReviewsChanged
				}
				if current.State != "running" || current.Attempt != lease.attempt {
					return ErrAnalysisConflict
				}
				return nil
			}
			for task := tr.Next(); task != nil; task = tr.Next() {
				if err = guard(); err != nil {
					return err
				}
				batchCtx, stop := context.WithTimeout(ctx, 10*time.Minute)
				execution, err := tr.Execute(batchCtx, task, r.Model, provider, guard)
				stop()
				if err != nil {
					return err
				}
				if err = guard(); err != nil {
					return err
				}
				if err = s.checkpointComposition(ctx, lease, tr, execution); err != nil {
					return err
				}
				if err = tr.Apply(r.Model, execution.Receipt, execution.RawResponse); err != nil {
					return err
				}
			}
			return nil
		}()
		if err != nil {
			code := "composition_failed"
			switch {
			case errors.Is(err, context.Canceled):
				code = "composition_interrupted"
			case errors.Is(err, context.DeadlineExceeded):
				code = "composition_step_timeout"
			case errors.Is(err, ErrCompositionReviewsChanged):
				code = "composition_reviews_changed"
			case errors.Is(err, ErrAnalysisUnavailable):
				code = "composition_source_unavailable"
			case errors.Is(err, ErrAnalysisLimit):
				code = "composition_limit"
			}
			_ = s.finishCompositionFailure(lease, code)
		}
	}()
	return nil
}

func (e *Engine) CancelComposition(owner, id string) error {
	s, err := e.ImportStore()
	if err != nil {
		return err
	}
	c := &e.analyses
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = s.CancelComposition(owner, id); err != nil {
		return err
	}
	if cancel := c.running["composition:"+id]; cancel != nil {
		cancel()
	}
	return nil
}
