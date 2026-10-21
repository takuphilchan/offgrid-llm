package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func awaitComputerCheckWaiters(t *testing.T, group *computerCheckFlights, key computerCheckKey, n int) {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		group.mu.Lock()
		f := group.active[key]
		ok := f != nil && f.waiters == n
		group.mu.Unlock()
		if ok {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("check did not reach %d observers", n)
		case <-tick.C:
		}
	}
}

func receiveComputerCheck(t *testing.T, ch <-chan computerModelCheck) computerModelCheck {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("check did not finish")
		return computerModelCheck{}
	}
}

func TestComputerModelCheckSharedCancellationAndNoResultCache(t *testing.T) {
	var group computerCheckFlights
	key := computerCheckKey{model: "fixture", driver: "browser", budget: time.Second}
	ready := make(chan struct{})
	var calls atomic.Int32
	run := func(ctx context.Context) computerModelCheck {
		calls.Add(1)
		select {
		case <-ready:
		case <-ctx.Done():
			return cancelledComputerCheck(key.model, ctx.Err())
		}
		value := true
		return computerModelCheck{Model: key.model, Passed: true, Runtime: &inference.ToolRuntimeInfo{Build: "fixture", SupportsTools: &value}}
	}
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	first, second := make(chan computerModelCheck, 1), make(chan computerModelCheck, 1)
	go func() { first <- group.do(firstCtx, nil, key, run) }()
	go func() { second <- group.do(context.Background(), nil, key, run) }()
	awaitComputerCheckWaiters(t, &group, key, 2)
	cancelFirst()
	if result := receiveComputerCheck(t, first); result.Passed || result.Code != "computer_model_check_cancelled" {
		t.Fatalf("wrong cancellation: %+v", result)
	}
	awaitComputerCheckWaiters(t, &group, key, 1)
	close(ready)
	if result := receiveComputerCheck(t, second); !result.Passed {
		t.Fatalf("other observer cancelled: %+v", result)
	}
	if calls.Load() != 1 {
		t.Fatal("concurrent callers repeated check")
	}
	if result := group.do(context.Background(), nil, key, run); !result.Passed || calls.Load() != 2 {
		t.Fatal("completed success was cached")
	}
}

func TestComputerModelCheckAbandonedCompletionCannotDeleteReplacement(t *testing.T) {
	var group computerCheckFlights
	key := computerCheckKey{model: "fixture"}
	oldCancelled, allowOldFinish, allowNewFinish := make(chan struct{}), make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan computerModelCheck, 1)
	go func() {
		first <- group.do(ctx, nil, key, func(ctx context.Context) computerModelCheck {
			<-ctx.Done()
			close(oldCancelled)
			<-allowOldFinish
			return computerModelCheck{Code: "old"}
		})
	}()
	awaitComputerCheckWaiters(t, &group, key, 1)
	group.mu.Lock()
	old := group.active[key]
	group.mu.Unlock()
	cancel()
	receiveComputerCheck(t, first)
	select {
	case <-oldCancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("last observer did not cancel work")
	}
	second := make(chan computerModelCheck, 1)
	go func() {
		second <- group.do(context.Background(), nil, key, func(context.Context) computerModelCheck {
			<-allowNewFinish
			return computerModelCheck{Passed: true, Code: "new"}
		})
	}()
	awaitComputerCheckWaiters(t, &group, key, 1)
	close(allowOldFinish)
	select {
	case <-old.done:
	case <-time.After(3 * time.Second):
		t.Fatal("old worker did not finish")
	}
	group.mu.Lock()
	replacement := group.active[key]
	group.mu.Unlock()
	if replacement == nil || replacement == old {
		t.Fatal("old completion deleted replacement")
	}
	close(allowNewFinish)
	if result := receiveComputerCheck(t, second); !result.Passed || result.Code != "new" {
		t.Fatalf("wrong result: %+v", result)
	}
}

func TestComputerModelCheckLifetimeAndObserverIsolation(t *testing.T) {
	var group computerCheckFlights
	key := computerCheckKey{model: "fixture"}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan computerModelCheck, 1)
	go func() {
		result <- group.do(context.Background(), ctx, key, func(ctx context.Context) computerModelCheck {
			<-ctx.Done()
			return cancelledComputerCheck(key.model, ctx.Err())
		})
	}()
	awaitComputerCheckWaiters(t, &group, key, 1)
	cancel()
	if value := receiveComputerCheck(t, result); value.Passed || !value.Retryable {
		t.Fatal("shutdown did not cancel check")
	}
	value := true
	original := computerModelCheck{Runtime: &inference.ToolRuntimeInfo{Build: "fixture", SupportsTools: &value}}
	a, b := cloneComputerCheck(original), cloneComputerCheck(original)
	a.Runtime.Build = "mutated"
	*a.Runtime.SupportsTools = false
	if b.Runtime.Build != "fixture" || !*b.Runtime.SupportsTools {
		t.Fatal("shared result pointers alias caller state")
	}
	cancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	group.do(cancelled, nil, key, func(context.Context) computerModelCheck { t.Error("cancelled caller started work"); return original })
}

func TestComputerModelCheckShutdownDrainsDetachedWorker(t *testing.T) {
	var group computerCheckFlights
	key := computerCheckKey{model: "fixture"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finish := make(chan struct{})
	defer close(finish)
	result := make(chan computerModelCheck, 1)
	go func() {
		result <- group.do(ctx, nil, key, func(ctx context.Context) computerModelCheck {
			<-ctx.Done()
			<-finish
			return cancelledComputerCheck(key.model, ctx.Err())
		})
	}()
	awaitComputerCheckWaiters(t, &group, key, 1)
	cancel()
	receiveComputerCheck(t, result)
	deadline, stop := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer stop()
	if err := group.shutdown(deadline); err == nil {
		t.Fatal("shutdown ignored detached running worker")
	}
	value := group.do(context.Background(), nil, key, func(context.Context) computerModelCheck {
		t.Error("new check admitted after shutdown")
		return computerModelCheck{Passed: true}
	})
	if value.Passed || !value.Retryable {
		t.Fatal("shutdown did not reject new checks")
	}
}

type blockedProbeEngine struct {
	probeEngine
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (e *blockedProbeEngine) ChatCompletion(ctx context.Context, req *api.ChatCompletionRequest) (*api.ChatCompletionResponse, error) {
	if e.calls.Add(1) == 1 {
		close(e.started)
		select {
		case <-e.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return e.probeEngine.ChatCompletion(ctx, req)
}

func TestComputerModelCheckDeduplicatesServicePreflight(t *testing.T) {
	gate := inference.NewLifecycleGate(1)
	release, err := gate.AcquireWithContext(context.Background(), "fixture", 8192, func(context.Context, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	release()
	engine := &blockedProbeEngine{probeEngine: probeEngine{mode: "good"}, started: make(chan struct{}), release: make(chan struct{})}
	s := &Server{config: &config.Config{MaxContextSize: 8192}, engine: engine, inferenceLifecycle: gate}
	first, second := make(chan computerModelCheck, 1), make(chan computerModelCheck, 1)
	go func() {
		first <- s.checkComputerModelDriverWithBudget(context.Background(), "fixture", "", time.Second)
	}()
	select {
	case <-engine.started:
	case <-time.After(3 * time.Second):
		t.Fatal("probe did not start")
	}
	go func() {
		second <- s.checkComputerModelDriverWithBudget(context.Background(), "fixture", "browser", time.Second)
	}()
	key := computerCheckKey{model: "fixture", driver: "browser", contextWindow: 8192, budget: time.Second}
	awaitComputerCheckWaiters(t, &s.computerModelChecks, key, 2)
	close(engine.release)
	if !receiveComputerCheck(t, first).Passed || !receiveComputerCheck(t, second).Passed {
		t.Fatal("shared synthetic check failed")
	}
	if engine.calls.Load() != 2 || gate.Status().Active != 0 {
		t.Fatal("duplicate probe or leaked inference lease")
	}
	if !s.checkComputerModelDriverWithBudget(context.Background(), "fixture", "browser", time.Second).Passed || engine.calls.Load() != 4 {
		t.Fatal("later task reused stale success")
	}
}
