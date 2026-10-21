package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestLegacyGenerationLeaseMeasuresOnlyAdmittedRuns(t *testing.T) {
	var releases atomic.Int32
	admission := ProviderTiming{Load: 4 * time.Millisecond, Queue: 3 * time.Millisecond}
	var observed ProviderTiming
	lease, err := NewLegacyGenerationLease(func() { releases.Add(1) }, admission, func(timing ProviderTiming) { observed = timing })
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := lease.Run(context.Background(), func(context.Context) error {
			time.Sleep(time.Millisecond)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	timing := lease.Timing()
	if timing.Load != admission.Load || timing.Queue != admission.Queue || timing.Generation <= 0 {
		t.Fatalf("unexpected timing: %+v", timing)
	}
	lease.Release()
	lease.Release()
	if observed != timing {
		t.Fatalf("recorded timing=%+v, want %+v", observed, timing)
	}
	if got := releases.Load(); got != 1 {
		t.Fatalf("release called %d times, want once", got)
	}
	if err := lease.Run(context.Background(), func(context.Context) error { return nil }); err == nil {
		t.Fatal("closed lease admitted generation")
	}
}

func TestLegacyGenerationLeaseRejectsCancellationBeforeDispatch(t *testing.T) {
	lease, err := NewLegacyGenerationLease(func() {}, ProviderTiming{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err = lease.Run(ctx, func(context.Context) error { called = true; return nil })
	if !errors.Is(err, context.Canceled) || called || lease.Timing().Generation != 0 {
		t.Fatalf("canceled generation dispatched: err=%v called=%t timing=%+v", err, called, lease.Timing())
	}
}

func TestLegacyGenerationLeaseReleaseWaitsForGeneration(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})
	released := make(chan struct{})
	lease, err := NewLegacyGenerationLease(func() { close(released) }, ProviderTiming{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runDone := make(chan error, 1)
	go func() {
		runDone <- lease.Run(context.Background(), func(context.Context) error {
			close(started)
			<-finish
			return nil
		})
	}()
	<-started
	releaseDone := make(chan struct{})
	go func() { lease.Release(); close(releaseDone) }()
	select {
	case <-released:
		t.Fatal("admission released while generation was still active")
	default:
	}
	close(finish)
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}
	<-releaseDone
	<-released
}

func TestLegacyGenerationLeaseRequiresAdmittedRelease(t *testing.T) {
	if _, err := NewLegacyGenerationLease(nil, ProviderTiming{}, nil); err == nil {
		t.Fatal("accepted a lease without admitted release")
	}
}

func TestLegacyGenerationLeaseConcurrentReleaseWaitsForSettlement(t *testing.T) {
	releasing := make(chan struct{})
	settle := make(chan struct{})
	lease, err := NewLegacyGenerationLease(func() {
		close(releasing)
		<-settle
	}, ProviderTiming{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstDone := make(chan struct{})
	go func() { lease.Release(); close(firstDone) }()
	<-releasing
	secondDone := make(chan struct{})
	go func() { lease.Release(); close(secondDone) }()
	select {
	case <-secondDone:
		t.Fatal("concurrent release returned before admission settled")
	default:
	}
	close(settle)
	<-firstDone
	<-secondDone
}

func TestLegacyGenerationLeaseRejectsLateSuccessAfterCancellation(t *testing.T) {
	lease, err := NewLegacyGenerationLease(func() {}, ProviderTiming{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	ctx, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	finish := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- lease.Run(ctx, func(context.Context) error {
			close(entered)
			<-finish // Simulate a runtime that ignores the cancelled context.
			return nil
		})
	}()
	<-entered
	cancel()
	<-ctx.Done()
	time.Sleep(time.Millisecond)
	close(finish)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("late model success escaped Stop: %v", err)
	}
	timing := lease.Timing()
	if timing.Generation <= 0 || timing.Cancel <= 0 || timing.Cancel > timing.Generation {
		t.Fatalf("cancel/generation timing is inconsistent: %+v", timing)
	}
}
