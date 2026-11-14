package inference

import (
	"context"
	"errors"
	"slices"
	"sync"
)

// SchedulingScope is service-owned scheduling metadata, never authority.
// Callers derive actor/work identities from accepted requests, not model text.
type SchedulingScope struct {
	Actor      string
	Work       string
	Background bool
}

type schedulingScopeKey struct{}

const schedulingPolicyVersion = "conversation-fairness-v1"

type AdmissionStatus struct {
	Policy           string `json:"policy"`
	Admitted         int    `json:"admitted"`
	Capacity         int    `json:"capacity"`
	ForegroundQueued int    `json:"foreground_queued"`
	BackgroundQueued int    `json:"background_queued"`
	QueueCapacity    int    `json:"queue_capacity"`
}

func (s *ProviderScheduler) AdmissionStatus() AdmissionStatus {
	q := s.admission
	q.mu.Lock()
	defer q.mu.Unlock()
	return AdmissionStatus{Policy: schedulingPolicyVersion, Admitted: q.active, Capacity: q.capacity, ForegroundQueued: q.counts[0], BackgroundQueued: q.counts[1], QueueCapacity: q.maxWait}
}

func WithSchedulingScope(ctx context.Context, scope SchedulingScope) context.Context {
	return context.WithValue(ctx, schedulingScopeKey{}, scope)
}

func schedulingScope(ctx context.Context) SchedulingScope {
	if scope, ok := ctx.Value(schedulingScopeKey{}).(SchedulingScope); ok {
		return scope
	}
	// Unlabelled maintenance/indexing is not foreground user interaction.
	return SchedulingScope{Background: true}
}

type generationWaiter struct {
	ready              chan struct{}
	admitted, released bool
}

type generationWork struct {
	id      string
	waiters []*generationWaiter
}

type generationActor struct {
	id    string
	works []*generationWork
}

// Admission precedes provider-family and runtime resources. Each class rotates
// actors, then work items within each actor. Residency cannot jump this order.
type generationAdmission struct {
	mu                        sync.Mutex
	classes                   [2][]*generationActor
	counts                    [2]int
	active, capacity, maxWait int
	foregroundStreak          int
}

func (q *generationAdmission) acquire(ctx, shutdown context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if shutdown.Err() != nil {
		return nil, errors.New("provider scheduler is closed")
	}
	scope := schedulingScope(ctx)
	class := 0
	if scope.Background {
		class = 1
	}
	w := &generationWaiter{ready: make(chan struct{})}
	q.mu.Lock()
	if q.counts[0]+q.counts[1] >= q.maxWait {
		q.mu.Unlock()
		return nil, ErrInferenceQueueFull
	}
	var actor *generationActor
	for _, item := range q.classes[class] {
		if item.id == scope.Actor {
			actor = item
			break
		}
	}
	if actor == nil {
		actor = &generationActor{id: scope.Actor}
		q.classes[class] = append(q.classes[class], actor)
	}
	var work *generationWork
	for _, item := range actor.works {
		if item.id == scope.Work {
			work = item
			break
		}
	}
	if work == nil {
		work = &generationWork{id: scope.Work}
		actor.works = append(actor.works, work)
	}
	work.waiters = append(work.waiters, w)
	q.counts[class]++
	q.dispatchLocked()
	q.mu.Unlock()
	release := func() { q.release(class, w) }
	select {
	case <-w.ready:
	case <-ctx.Done():
		release()
		return nil, ctx.Err()
	case <-shutdown.Done():
		release()
		return nil, errors.New("provider scheduler is closed")
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	if shutdown.Err() != nil {
		release()
		return nil, errors.New("provider scheduler is closed")
	}
	return release, nil
}

func (q *generationAdmission) dispatchLocked() {
	for q.active < q.capacity && q.counts[0]+q.counts[1] > 0 {
		class := 0
		if q.counts[0] == 0 || q.counts[1] > 0 && q.foregroundStreak >= 3 {
			class = 1
			q.foregroundStreak = 0
		} else if q.counts[1] > 0 {
			q.foregroundStreak++
		} else {
			q.foregroundStreak = 0
		}
		actor := q.classes[class][0]
		work := actor.works[0]
		w := work.waiters[0]
		work.waiters = slices.Delete(work.waiters, 0, 1)
		actor.works = slices.Delete(actor.works, 0, 1)
		if len(work.waiters) > 0 {
			actor.works = append(actor.works, work)
		}
		q.classes[class] = slices.Delete(q.classes[class], 0, 1)
		if len(actor.works) > 0 {
			q.classes[class] = append(q.classes[class], actor)
		}
		q.counts[class]--
		q.active++
		w.admitted = true
		close(w.ready)
	}
}

func (q *generationAdmission) release(class int, w *generationWaiter) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if w.released {
		return
	}
	w.released = true
	if w.admitted {
		q.active--
	} else {
		for ai, actor := range q.classes[class] {
			for wi, work := range actor.works {
				if i := slices.Index(work.waiters, w); i >= 0 {
					work.waiters = slices.Delete(work.waiters, i, i+1)
					if len(work.waiters) == 0 {
						actor.works = slices.Delete(actor.works, wi, wi+1)
					}
					if len(actor.works) == 0 {
						q.classes[class] = slices.Delete(q.classes[class], ai, ai+1)
					}
					q.counts[class]--
					q.dispatchLocked()
					return
				}
			}
		}
	}
	q.dispatchLocked()
}
