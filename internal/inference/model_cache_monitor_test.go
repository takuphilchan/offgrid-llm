package inference

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

func TestModelCacheStopMonitorWaitsAndIsIdempotent(t *testing.T) {
	cache := NewModelCache(1, 0, t.TempDir())
	var stopped sync.WaitGroup
	for range 4 {
		stopped.Add(1)
		go func() {
			defer stopped.Done()
			cache.StopMonitor()
		}()
	}
	stopped.Wait()
	if err := cache.monitorCtx.Err(); err == nil {
		t.Fatal("monitor context remains live after StopMonitor")
	}
	select {
	case <-cache.monitorDone:
	default:
		t.Fatal("StopMonitor returned before monitor exit")
	}
}

func TestModelCacheCrashRestartRequiresAdmissionBeforeCleanup(t *testing.T) {
	cache := NewModelCache(1, 0, t.TempDir())
	defer cache.StopMonitor()
	dead := &ModelInstance{ModelID: "fixture", Cmd: &exec.Cmd{Process: &os.Process{Pid: 99999999}}, Port: 18001, ModelPath: "missing.gguf"}
	cache.mu.Lock()
	cache.instances[dead.ModelID] = dead
	cache.mu.Unlock()
	var admissions int
	cache.SetRestartAdmission(func(context.Context) (func(), error) {
		admissions++
		return nil, errors.New("provider is busy")
	})
	cache.checkAndRestartCrashed(context.Background())
	cache.mu.RLock()
	retained := cache.instances[dead.ModelID] == dead
	cache.mu.RUnlock()
	if admissions != 1 || !retained {
		t.Fatalf("crash recovery bypassed admission: attempts=%d retained=%v", admissions, retained)
	}
}

func TestModelCacheCrashRestartWaitsForProviderLease(t *testing.T) {
	cache := NewModelCache(1, 0, t.TempDir())
	defer cache.StopMonitor()
	dead := &ModelInstance{ModelID: "fixture", Cmd: &exec.Cmd{Process: &os.Process{Pid: 99999999}}, Port: 18002, ModelPath: "missing.gguf"}
	cache.mu.Lock()
	cache.instances[dead.ModelID] = dead
	cache.mu.Unlock()
	scheduler, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	cache.SetRestartAdmission(func(ctx context.Context) (func(), error) {
		release, _, err := scheduler.AcquireLegacyMaintenance(ctx)
		return release, err
	})
	held, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	cache.checkAndRestartCrashed(ctx)
	cache.mu.RLock()
	retained := cache.instances[dead.ModelID] == dead
	cache.mu.RUnlock()
	if !retained {
		t.Fatal("monitor mutated crashed process while provider generation held the lease")
	}
}
