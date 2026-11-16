package rag

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// StartSynthesis shares comparison's bounded worker/queue. Each model lease is
// released between batches, allowing foreground inference to take precedence.
// A retry is explicit and starts at the last atomically committed source cursor.
func (e *Engine) StartSynthesis(owner, id string, provider document.SynthesisLocalProvider) error {
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
	if _, ok := c.running[id]; ok {
		return ErrAnalysisConflict
	}
	if len(c.running) >= 4 {
		return ErrAnalysisLimit
	}
	r, lease, err := s.beginSynthesis(owner, id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.running[id] = cancel
	c.wait.Add(1)
	go func() {
		defer c.wait.Done()
		defer func() { cancel(); c.mu.Lock(); delete(c.running, id); c.mu.Unlock() }()
		err := func() error {
			select {
			case c.gate <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}
			defer func() { <-c.gate }()
			traversal, err := s.synthesisTraversal(owner, id)
			if err != nil {
				return err
			}
			guard := func() error {
				if err := ctx.Err(); err != nil {
					return err
				}
				current, err := s.Synthesis(owner, id)
				if err != nil {
					return err
				}
				if current.State != "running" || current.Attempt != lease.attempt {
					return ErrAnalysisConflict
				}
				return nil
			}
			for r.UnitCursor < len(r.Plan.Units) {
				if err = guard(); err != nil {
					return err
				}
				batch, err := traversal.Batch(ctx, r.UnitCursor, r.NextByte, r.PrimaryBytes)
				if err != nil {
					return err
				}
				var execution *document.SynthesisExecution
				if strings.TrimSpace(batch.Evidence[0].Text) == "" {
					receipt, err := traversal.EmptyReceipt(batch, r.Model)
					if err != nil {
						return err
					}
					execution = &document.SynthesisExecution{Receipt: receipt}
				} else {
					batchCtx, stop := context.WithTimeout(ctx, 10*time.Minute)
					execution, err = traversal.ExecuteSynthesisBatch(batchCtx, batch, r.Model, provider, guard)
					stop()
					if err != nil {
						return err
					}
				}
				if err = guard(); err != nil {
					return err
				}
				if err = s.checkpointSynthesis(lease, r.CompletedBatches, traversal, batch, execution); err != nil {
					return err
				}
				r, err = s.Synthesis(owner, id)
				if err != nil {
					return err
				}
			}
			return nil
		}()
		if err != nil {
			code := "analysis_failed"
			if errors.Is(err, context.Canceled) {
				code = "analysis_interrupted"
			}
			if errors.Is(err, context.DeadlineExceeded) {
				code = "analysis_batch_timeout"
			}
			if errors.Is(err, ErrAnalysisUnavailable) {
				code = "analysis_source_unavailable"
			}
			if errors.Is(err, ErrAnalysisLimit) {
				code = "analysis_limit"
			}
			_ = s.finishAnalysisFailure(lease, code)
		}
	}()
	return nil
}
