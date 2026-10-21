package inference

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// LegacyGenerationLease records actual engine-call time while retaining the
// caller's scheduler/model admission. It does not grant admission by itself;
// the supplied release must come from an already acquired model lease.
type LegacyGenerationLease struct {
	mu      sync.Mutex
	release func()
	record  func(ProviderTiming)
	timing  ProviderTiming
	closed  bool
	done    chan struct{}
}

func NewLegacyGenerationLease(release func(), admission ProviderTiming, record func(ProviderTiming)) (*LegacyGenerationLease, error) {
	if release == nil {
		return nil, errors.New("legacy generation requires an admitted model lease")
	}
	return &LegacyGenerationLease{release: release, record: record, timing: admission, done: make(chan struct{})}, nil
}

// Run serializes generations on this one lease, not across independent
// admitted callers. Context cancellation remains owned by the caller and its
// engine; measured time includes an interrupted generation until it returns.
func (l *LegacyGenerationLease) Run(ctx context.Context, generate func(context.Context) error) error {
	if l == nil || generate == nil {
		return errors.New("legacy generation requires a lease and operation")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return errors.New("legacy generation lease is closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var cancelledAt atomic.Int64
	cancelObserved := make(chan struct{})
	stopCancelWatch := context.AfterFunc(ctx, func() {
		cancelledAt.CompareAndSwap(0, time.Now().UnixNano())
		close(cancelObserved)
	})
	started := time.Now()
	err := generate(ctx)
	finished := time.Now()
	if stopped := stopCancelWatch(); ctx.Err() != nil {
		if !stopped {
			<-cancelObserved
		} else {
			cancelledAt.CompareAndSwap(0, finished.UnixNano())
		}
	}
	l.timing.Generation += finished.Sub(started)
	if when := cancelledAt.Load(); when != 0 {
		elapsed := finished.Sub(time.Unix(0, when))
		if elapsed <= 0 {
			elapsed = time.Nanosecond // Cancellation raced the return boundary.
		}
		if generation := finished.Sub(started); elapsed > generation {
			elapsed = generation
		}
		l.timing.Cancel += elapsed
	}
	// An engine that ignores Stop or a deadline cannot turn its late output
	// into a successful decision or tool dispatch.
	if err == nil {
		err = ctx.Err()
	}
	return err
}

func (l *LegacyGenerationLease) Timing() ProviderTiming {
	if l == nil {
		return ProviderTiming{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.timing
}

func (l *LegacyGenerationLease) Release() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.closed {
		done := l.done
		l.mu.Unlock()
		<-done
		return
	}
	l.closed = true
	release := l.release
	record := l.record
	timing := l.timing
	l.mu.Unlock()
	release()
	if record != nil {
		record(timing)
	}
	close(l.done)
}
