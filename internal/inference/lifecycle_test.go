package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifecycleGateSharesActiveModel(t *testing.T) {
	gate := NewLifecycleGate(2)
	var switches atomic.Int32
	switcher := func(context.Context, string) error { switches.Add(1); return nil }
	releaseOne, err := gate.Acquire(context.Background(), "model-a", switcher)
	if err != nil {
		t.Fatal(err)
	}
	releaseTwo, err := gate.Acquire(context.Background(), "model-a", switcher)
	if err != nil {
		t.Fatal(err)
	}
	if status := gate.Status(); status.Active != 2 || status.CurrentModel != "model-a" || switches.Load() != 1 {
		t.Fatalf("unexpected lifecycle status: %#v switches=%d", status, switches.Load())
	}
	releaseOne()
	releaseTwo()
}

func TestLifecycleContextSwitchIsExclusive(t *testing.T) {
	gate := NewLifecycleGate(2)
	calls := 0
	switcher := func(context.Context, string) error { calls++; return nil }
	release, err := gate.AcquireWithContext(context.Background(), "same-model", 8192, switcher)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := gate.AcquireWithContext(ctx, "same-model", 65536, switcher); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("profile changed during active inference: %v", err)
	}
	if calls != 1 {
		t.Fatalf("switches=%d", calls)
	}
	release()
	release, err = gate.AcquireWithContext(context.Background(), "same-model", 65536, switcher)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if gate.Status().ContextWindow != 65536 || calls != 2 {
		t.Fatalf("wrong profile: %+v", gate.Status())
	}
}

func TestLifecycleGateCancelsQueuedModelSwitch(t *testing.T) {
	gate := NewLifecycleGate(1)
	release, err := gate.Acquire(context.Background(), "model-a", func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = gate.Acquire(ctx, "model-b", func(context.Context, string) error {
		t.Fatal("switch should not start while another model has an active lease")
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued acquisition error = %v", err)
	}
	release()
}

func TestModelCachePendingLoadWaitIsCancellable(t *testing.T) {
	pending := &pendingLoad{done: make(chan struct{})}
	cache := &ModelCache{
		instances:    make(map[string]*ModelInstance),
		pendingLoads: map[string]*pendingLoad{"model-a": pending},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cache.GetOrLoadContext(ctx, "model-a", "unused", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("pending load wait error = %v", err)
	}
}

func TestModelCachePendingLoadBroadcastsError(t *testing.T) {
	want := errors.New("load failed")
	pending := &pendingLoad{done: make(chan struct{}), err: want}
	close(pending.done)
	cache := &ModelCache{
		instances:    make(map[string]*ModelInstance),
		pendingLoads: map[string]*pendingLoad{"model-a": pending},
	}
	for i := 0; i < 2; i++ {
		_, err := cache.GetOrLoadContext(context.Background(), "model-a", "unused", "")
		if !errors.Is(err, want) {
			t.Fatalf("waiter %d error = %v", i, err)
		}
	}
}
