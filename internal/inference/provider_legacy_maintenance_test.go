package inference

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLegacyMaintenanceRequiresSchedulerLeaseAndAttachedOwner(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	if _, err := scheduler.WithLegacyMaintenance(context.Background(), func(LegacyMaintenance) error { return nil }); err == nil {
		t.Fatal("unattached cache was accepted as a process owner")
	}
	cache := NewModelCache(1, 0, t.TempDir())
	if _, err := scheduler.AttachLlamaCache(cache, NewMockEngine()); err != nil {
		cache.StopMonitor()
		t.Fatal(err)
	}
	held, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	called := false
	if _, err := scheduler.WithLegacyMaintenance(ctx, func(LegacyMaintenance) error { called = true; return nil }); !errors.Is(err, context.DeadlineExceeded) || called {
		t.Fatalf("maintenance bypassed active generation: called=%v err=%v", called, err)
	}
	held()
	if _, err := scheduler.WithLegacyMaintenance(context.Background(), func(owner LegacyMaintenance) error {
		called = true
		if err := owner.UnloadModel("absent"); err != nil {
			return err
		}
		return owner.DisconnectEngine()
	}); err != nil || !called {
		t.Fatalf("attached owner could not perform exclusive maintenance: called=%v err=%v", called, err)
	}
}

func TestLegacyMaintenanceHoldsProcessOwnerThroughCallback(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewModelCache(1, 0, t.TempDir())
	if _, err := scheduler.AttachLlamaCache(cache, NewMockEngine()); err != nil {
		cache.StopMonitor()
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	maintained := make(chan error, 1)
	go func() {
		_, err := scheduler.WithLegacyMaintenance(context.Background(), func(LegacyMaintenance) error {
			close(entered)
			<-release
			return nil
		})
		maintained <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("maintenance callback was not admitted")
	}
	closed := make(chan error, 1)
	go func() { closed <- scheduler.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("scheduler retired the process during maintenance: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	if err := <-maintained; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}

func TestLegacyMaintenanceReportsEngineUnloadSeparatelyFromQueue(t *testing.T) {
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	cache := NewModelCache(1, 0, t.TempDir())
	engine := &delayedUnloadEngine{MockEngine: NewMockEngine()}
	if _, err := scheduler.AttachLlamaCache(cache, engine); err != nil {
		cache.StopMonitor()
		t.Fatal(err)
	}
	if err := engine.Load(context.Background(), "fixture.gguf", LoadOptions{}); err != nil {
		t.Fatal(err)
	}
	if !engine.IsLoaded() {
		t.Fatal("fixture engine did not load")
	}
	timing, err := scheduler.WithLegacyMaintenance(context.Background(), func(owner LegacyMaintenance) error {
		if !engine.IsLoaded() {
			t.Fatal("engine was retired before the maintenance callback")
		}
		if owner.engine != engine || !owner.engine.IsLoaded() {
			t.Fatalf("maintenance received a different engine: owner=%T loaded=%v", owner.engine, owner.engine.IsLoaded())
		}
		err := owner.DisconnectEngine()
		return err
	})
	if err != nil || engine.IsLoaded() || timing.Unload <= 0 {
		t.Fatalf("engine retirement was not accounted by scheduler: timing=%+v loaded=%v err=%v", timing, engine.IsLoaded(), err)
	}
}

type delayedUnloadEngine struct{ *MockEngine }

func (e *delayedUnloadEngine) Unload() error {
	time.Sleep(2 * time.Millisecond)
	return e.MockEngine.Unload()
}
