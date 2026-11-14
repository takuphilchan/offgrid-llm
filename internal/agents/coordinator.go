package agents

import (
	"context"
	"log"
	"time"
)

// StartCoordinator keeps durable children and service-owned input admission in
// the SAME runner. Restart never auto-resumes interrupted graphs or occurrences.
func (r *Runner) StartCoordinator(ctx context.Context) {
	r.coordinatorOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-ticker.C:
					r.mu.Lock()
					closing := r.closing
					r.mu.Unlock()
					if closing {
						return
					}
					r.coordinateWorkerPreparations(ctx)
					r.coordinateOccurrences(ctx, now)
					for _, task := range r.manager.coordinationCandidates() {
						if task.Status == TaskPending {
							_, _ = r.Continue(ctx, task.ID, task.Actor, "start", "", true)
						}
						if task.Status == TaskChildren {
							r.advanceDelegation(ctx, task.ID)
						}
					}
				}
			}
		}()
	})
}

// At most one preparation callback is in flight. It is owned/drained alongside
// model workers, so a slow metadata/Knowledge read cannot starve ready children.
func (r *Runner) coordinateOccurrences(ctx context.Context, now time.Time) {
	r.mu.Lock()
	if r.closing || r.Coordinate == nil || r.coordinateCancel != nil {
		r.mu.Unlock()
		return
	}
	work, cancel := context.WithTimeout(ctx, 30*time.Second)
	r.coordinateCancel = cancel
	r.workers.Add(1)
	r.mu.Unlock()
	go func() {
		defer func() { cancel(); r.mu.Lock(); r.coordinateCancel = nil; r.mu.Unlock(); r.workers.Done() }()
		if err := r.Coordinate(work, now); err != nil && work.Err() == nil {
			log.Printf("Agent work coordination unavailable; inspect saved work and service storage before retrying")
		}
	}()
}

// Native preparation has its own bounded controller slot. A slow host cannot
// occupy conversation-input delivery, a model slot, or the control-command lock.
func (r *Runner) coordinateWorkerPreparations(ctx context.Context) {
	if _, ok := r.tools.(executionWorkerPreparer); !ok {
		return
	}
	r.mu.Lock()
	if r.closing || r.workerPreparationCancel != nil {
		r.mu.Unlock()
		return
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	r.workerPreparationCancel = cancel
	r.workers.Add(1)
	r.mu.Unlock()
	go func() {
		defer func() { cancel(); r.mu.Lock(); r.workerPreparationCancel = nil; r.mu.Unlock(); r.workers.Done() }()
		r.coordinateExecutionWorkers(work)
		if observer, ok := r.tools.(interface{ ObserveWorkerIntegrations(context.Context) }); ok {
			observer.ObserveWorkerIntegrations(work)
		}
	}()
}
