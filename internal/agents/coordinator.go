package agents

import (
	"context"
	"log"
	"time"
)

// StartCoordinator keeps durable children and optional occurrence admission in
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
			log.Printf("Agent schedule coordination unavailable; inspect owned schedule/storage state before retrying")
		}
	}()
}
