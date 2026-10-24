package inference

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func waitPreparationDemand(t *testing.T, s *ProviderScheduler, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		count := 0
		for _, pending := range s.legacyPreparing {
			count += pending.waiters
		}
		s.mu.Unlock()
		if count == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("preparation demand did not reach %d", want)
}

func TestLegacyPreparationSurvivesAuxiliaryCancellation(t *testing.T) {
	s, _ := NewProviderScheduler(NewProviderRegistry(), 1)
	defer s.Close()
	g := NewLifecycleGate(1)
	started, ready := make(chan struct{}), make(chan struct{})
	var loads atomic.Int32
	load := func(ctx context.Context, _ string) error {
		if loads.Add(1) == 1 {
			close(started)
		}
		select {
		case <-ready:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	short, cancel := context.WithCancel(context.Background())
	defer cancel()
	first, second := make(chan error, 1), make(chan error, 1)
	go func() {
		release, _, err := s.AcquireLegacyModel(short, g, "any-model", 65536, nil, load)
		if release != nil {
			release()
		}
		first <- err
	}()
	<-started
	go func() {
		release, _, err := s.AcquireLegacyModel(context.Background(), g, "any-model", 32768, nil, load)
		if release != nil {
			release()
		}
		second <- err
	}()
	waitPreparationDemand(t, s, 2)
	cancel()
	select {
	case err := <-first:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled auxiliary caller remained blocked")
	}
	close(ready)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 1 || g.Status().ContextWindow != 65536 {
		t.Fatal("compatible demand restarted or shrank loading", loads.Load(), g.Status())
	}
}

func TestLegacyPreparationAllCancelledRetiresBeforeRetry(t *testing.T) {
	s, _ := NewProviderScheduler(NewProviderRegistry(), 1)
	defer s.Close()
	g := NewLifecycleGate(1)
	started, cleaning, cleaned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var loads atomic.Int32
	load := func(ctx context.Context, _ string) error {
		if loads.Add(1) == 1 {
			close(started)
			<-ctx.Done()
			close(cleaning)
			<-cleaned
			return ctx.Err()
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() {
		release, _, err := s.AcquireLegacyModel(ctx, g, "fixture", 4096, nil, load)
		if release != nil {
			release()
		}
		first <- err
	}()
	<-started
	cancel()
	if err := <-first; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	<-cleaning
	second := make(chan error, 1)
	go func() {
		release, _, err := s.AcquireLegacyModel(context.Background(), g, "fixture", 4096, nil, load)
		if release != nil {
			release()
		}
		second <- err
	}()
	select {
	case err := <-second:
		t.Fatal("new request overlapped retiring preparation", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(cleaned)
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if loads.Load() != 2 {
		t.Fatal(loads.Load())
	}
}

func TestLegacyPreparationShutdownCancelsAndDrains(t *testing.T) {
	s, _ := NewProviderScheduler(NewProviderRegistry(), 1)
	g := NewLifecycleGate(1)
	started, stopped := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		release, _, err := s.AcquireLegacyModel(context.Background(), g, "fixture", 4096, nil, func(ctx context.Context, _ string) error {
			close(started)
			<-ctx.Done()
			close(stopped)
			return ctx.Err()
		})
		if release != nil {
			release()
		}
		done <- err
	}()
	<-started
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("shutdown did not drain owned preparation")
	}
	if err := <-done; err == nil {
		t.Fatal("shutdown produced a generation lease")
	}
}

func TestLegacyPreparationRetainsExclusiveLoadAndMemoryFailure(t *testing.T) {
	s, _ := NewProviderScheduler(NewProviderRegistry(), 2)
	defer s.Close()
	g := NewLifecycleGate(2)
	started, finish := make(chan struct{}), make(chan struct{})
	var active atomic.Int32
	var overlap atomic.Bool
	failure := errors.New("fixture memory admission rejected")
	load := func(ctx context.Context, model string) error {
		if active.Add(1) != 1 {
			overlap.Store(true)
		}
		defer active.Add(-1)
		if model == "first" {
			close(started)
			<-finish
			return failure
		}
		return nil
	}
	first, second := make(chan error, 1), make(chan error, 1)
	go func() {
		release, _, err := s.AcquireLegacyModel(context.Background(), g, "first", 4096, nil, load)
		if release != nil {
			release()
		}
		first <- err
	}()
	<-started
	go func() {
		release, _, err := s.AcquireLegacyModel(context.Background(), g, "second", 4096, nil, load)
		if release != nil {
			release()
		}
		second <- err
	}()
	waitPreparationDemand(t, s, 2)
	close(finish)
	if err := <-first; !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if overlap.Load() {
		t.Fatal("parallel generation capacity allowed concurrent model changes")
	}
}
