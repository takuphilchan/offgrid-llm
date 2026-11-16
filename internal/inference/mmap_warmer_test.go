package inference

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/resource"
)

func warmFixture(t *testing.T) (*MmapWarmer, string) {
	t.Helper()
	w := NewMmapWarmer(t.TempDir())
	w.memory = func() (*resource.SystemResources, error) {
		return &resource.SystemResources{TotalRAM: 8192, AvailableRAM: 4096}, nil
	}
	path := filepath.Join(w.modelsDir, "model.gguf")
	if err := os.WriteFile(path, make([]byte, 3<<20), 0600); err != nil {
		t.Fatal(err)
	}
	return w, path
}

func TestWarmerPartialReadBudgetAndRecentReadReuse(t *testing.T) {
	w, path := warmFixture(t)
	budget := int64(1 << 20)
	status, err := w.warm(path, 0, &budget, time.Now().Add(time.Second))
	if err != nil || budget != 0 || status.BytesRead != 1<<20 || status.Complete || status.InPageCache || w.IsWarmed(path) {
		t.Fatalf("%+v %v", status, err)
	}
	budget = 1 << 20
	_, err = w.warm(path, 0, &budget, time.Now().Add(time.Second))
	if err != nil || budget != 1<<20 {
		t.Fatal("recent speculative read repeated", budget, err)
	}
	w.InvalidateCache(path)
	status, err = w.FastWarmModel(path)
	if err != nil || !status.Complete || status.BytesRead != 3<<20 || !w.IsWarmed(path) {
		t.Fatalf("%+v %v", status, err)
	}
	status.Complete = false
	if !w.GetStatus(path).Complete {
		t.Fatal("caller mutated cache")
	}
}

func TestWarmerStopsOnPressureAndDoesNotBypassUnknownMemory(t *testing.T) {
	w, path := warmFixture(t)
	for _, sample := range []*resource.SystemResources{nil, {TotalRAM: 8192, AvailableRAM: 512}, {TotalRAM: 8192, AvailableRAM: 7000, CommitMeasured: true, AvailableCommitMB: 0}} {
		w.memory = func() (*resource.SystemResources, error) { return sample, nil }
		if status, err := w.FastWarmModel(path); err == nil || status != nil {
			t.Fatal("pressure bypassed", status, err)
		}
	}
	w.memory = func() (*resource.SystemResources, error) { return nil, errors.New("unavailable") }
	if _, err := w.WarmModel(path); err == nil {
		t.Fatal("unmeasured memory accepted")
	}
	var calls int
	w.memory = func() (*resource.SystemResources, error) {
		calls++
		available := int64(4096)
		if calls > 2 {
			available = 100
		}
		return &resource.SystemResources{TotalRAM: 8192, AvailableRAM: available}, nil
	}
	status, err := w.WarmModel(path)
	if err == nil || status.BytesRead != 1<<20 || status.Complete {
		t.Fatalf("pressure did not interrupt: %+v %v", status, err)
	}
}

func TestWarmerPauseResumeCancelsOldOperationAndAsyncFloodIsBounded(t *testing.T) {
	w, path := warmFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	w.memory = func() (*resource.SystemResources, error) {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return &resource.SystemResources{TotalRAM: 8192, AvailableRAM: 4096}, nil
	}
	w.WarmOnDemand(path)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	for i := 0; i < 10000; i++ {
		w.WarmModelsAsync([]string{path})
	}
	if calls.Load() != 1 {
		t.Fatal("multiple workers admitted")
	}
	w.Pause()
	w.Resume()
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for w.busy.Load() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if w.busy.Load() || w.IsWarmed(path) {
		t.Fatal("cancelled warm leaked or reported completion")
	}
	status, err := w.WarmModel(path)
	if err != nil || !status.Complete {
		t.Fatalf("admission not released: %+v %v", status, err)
	}
}

func TestWarmerDeadlineReadFailureAndBoundedHistory(t *testing.T) {
	w, path := warmFixture(t)
	budget := int64(1 << 20)
	status, err := w.warm(path, 0, &budget, time.Now().Add(-time.Second))
	if err == nil || status.BytesRead != 0 || status.Complete {
		t.Fatal(status, err)
	}
	// Truncate after stat/open, at the first pressure observation.
	w.memory = func() (*resource.SystemResources, error) {
		if err := os.Truncate(path, 0); err != nil {
			t.Fatal(err)
		}
		return &resource.SystemResources{TotalRAM: 8192, AvailableRAM: 4096}, nil
	}
	status, err = w.warm(path, 0, &budget, time.Now().Add(time.Second))
	if err == nil || !strings.Contains(err.Error(), "read failed") || status.Complete {
		t.Fatalf("truncation ignored: %+v %v", status, err)
	}
	for i := 0; i < 100; i++ {
		w.remember(WarmStatus{ModelPath: fmt.Sprint(i), WarmedAt: time.Now()})
	}
	if len(w.GetAllStatus()) != warmHistoryLimit || w.GetStatus("0") != nil {
		t.Fatal("history is unbounded")
	}
}

func TestWarmAllStopsAfterBoundedNumberOfFiles(t *testing.T) {
	w, _ := warmFixture(t)
	for i := 0; i < 45; i++ {
		if err := os.WriteFile(filepath.Join(w.modelsDir, fmt.Sprintf("%02d.gguf", i)), []byte("small test model"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	statuses, err := w.WarmAllModels()
	if err != nil || len(statuses) != warmHistoryLimit {
		t.Fatal(len(statuses), err)
	}
}
