package rag

import (
	"context"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/document"
)

// StartComparison runs independently of the initiating HTTP connection, with a
// single worker and a bounded queue. Each complete source-unit pair commits its
// result and progress together. Restart/retry resumes after the last checkpoint.
func (e *Engine) StartComparison(owner, id string) error {
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
	r, lease, err := s.beginAnalysis(owner, id)
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
			graphs, err := s.analysisGraphs(owner, id)
			if err != nil {
				return err
			}
			comparison, err := document.PrepareComparison(graphs[0], graphs[1])
			if err != nil {
				return err
			}
			for step := r.CompletedSteps; step < len(r.Pairs); step++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				if _, err = s.Analysis(owner, id); err != nil {
					return err
				}
				result, err := comparison.Step(ctx, step, r.Pairs[step])
				if err != nil {
					return err
				}
				if err = ctx.Err(); err != nil {
					return err
				}
				if err = s.checkpointComparison(lease, step, result); err != nil {
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
			if errors.Is(err, ErrAnalysisUnavailable) {
				code = "analysis_source_unavailable"
			}
			if errors.Is(err, ErrAnalysisLimit) {
				code = "analysis_limit"
			}
			// No raw error/source text enters ordinary logs or durable error codes.
			_ = s.finishAnalysisFailure(lease, code)
		}
	}()
	return nil
}

func (s *SQLiteStore) analysisGraphs(owner, id string) ([]*document.Extraction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, err := s.analysisLocked(owner, id)
	if err != nil {
		return nil, err
	}
	graphs := []*document.Extraction{}
	for _, source := range r.Plan.Sources {
		g, err := s.extractionLocked(source.DocumentID, source.Revision)
		if err != nil {
			return nil, ErrAnalysisUnavailable
		}
		graphs = append(graphs, g)
	}
	plan, err := document.BuildAnalysisPlan(r.Plan.Workflow, r.Plan.Sources, graphs)
	if err != nil || plan.ID != r.Plan.ID {
		return nil, ErrAnalysisInvalid
	}
	if len(graphs) != 2 || r.Plan.Workflow != "comparison" {
		return nil, ErrAnalysisInvalid
	}
	return graphs, nil
}

func (e *Engine) CancelAnalysis(owner, id string) error {
	s, err := e.ImportStore()
	if err != nil {
		return err
	}
	c := &e.analyses
	c.mu.Lock()
	defer c.mu.Unlock()
	if err = s.CancelAnalysis(owner, id); err != nil {
		return err
	}
	if cancel := c.running[id]; cancel != nil {
		cancel()
	}
	return nil
}

func (e *Engine) closeAnalyses() {
	c := &e.analyses
	c.mu.Lock()
	c.closed = true
	for _, cancel := range c.running {
		cancel()
	}
	c.mu.Unlock()
	c.wait.Wait()
}
