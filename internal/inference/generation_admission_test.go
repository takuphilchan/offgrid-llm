package inference

import (
	"context"
	"errors"
	"testing"
	"time"
)

func awaitGenerationQueue(t *testing.T, q *generationAdmission, n int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		q.mu.Lock()
		count := q.counts[0] + q.counts[1]
		q.mu.Unlock()
		if count == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("generation queue did not reach", n)
}

func TestGenerationAdmissionFairnessAcrossClassesActorsAndWork(t *testing.T) {
	if !schedulingScope(context.Background()).Background {
		t.Fatal("unlabelled internal work gained foreground priority")
	}
	q := &generationAdmission{capacity: 1, maxWait: 32}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, err := q.acquire(ctx, ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	type admitted struct {
		id      string
		release func()
		err     error
	}
	results := make(chan admitted, 12)
	scopes := []SchedulingScope{
		{Actor: "a", Work: "x"}, {Actor: "a", Work: "x"}, {Actor: "a", Work: "x"},
		{Actor: "a", Work: "y"}, {Actor: "a", Work: "y"},
		{Actor: "b", Work: "z"}, {Actor: "b", Work: "z"}, {Actor: "c", Work: "z"},
		{Actor: "d", Work: "background", Background: true}, {Actor: "d", Work: "background", Background: true},
	}
	for i, scope := range scopes {
		go func(scope SchedulingScope) {
			release, err := q.acquire(WithSchedulingScope(ctx, scope), ctx)
			results <- admitted{scope.Actor + "/" + scope.Work, release, err}
		}(scope)
		awaitGenerationQueue(t, q, i+1)
	}
	held()
	for _, want := range []string{"a/x", "b/z", "c/z", "d/background", "a/y", "b/z", "a/x", "d/background", "a/y", "a/x"} {
		select {
		case got := <-results:
			if got.err != nil {
				t.Fatal(got.err)
			}
			got.release()
			got.release() // Idempotent release cannot create additional slots.
			if got.id != want {
				t.Fatalf("admitted %s, want %s", got.id, want)
			}
		case <-ctx.Done():
			t.Fatal("admission starved", want)
		}
	}
	awaitGenerationQueue(t, q, 0)
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.active != 0 || len(q.classes[0])+len(q.classes[1]) != 0 {
		t.Fatal("queue leaked resources")
	}
}

func TestGenerationAdmissionCancellationCapacityAndShutdown(t *testing.T) {
	q := &generationAdmission{capacity: 1, maxWait: 1}
	shutdown, closeQueue := context.WithCancel(context.Background())
	defer closeQueue()
	held, err := q.acquire(context.Background(), shutdown)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		release, err := q.acquire(ctx, shutdown)
		if release != nil {
			release()
		}
		done <- err
	}()
	awaitGenerationQueue(t, q, 1)
	if _, err := q.acquire(context.Background(), shutdown); !errors.Is(err, ErrInferenceQueueFull) {
		t.Fatal("unbounded queue", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	awaitGenerationQueue(t, q, 0)
	go func() {
		release, err := q.acquire(context.Background(), shutdown)
		if release != nil {
			release()
		}
		done <- err
	}()
	awaitGenerationQueue(t, q, 1)
	closeQueue()
	if err := <-done; err == nil {
		t.Fatal("shutdown admitted queued generation")
	}
	awaitGenerationQueue(t, q, 0)
	held()
	if _, err := q.acquire(context.Background(), shutdown); err == nil {
		t.Fatal("closed queue reopened")
	}
}

func TestProviderQueuePriorityPrecedesFamilyAndRuntimeResources(t *testing.T) {
	s, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, _, err := s.acquireResources(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	type admission struct {
		native  bool
		release func()
		err     error
	}
	done := make(chan admission, 2)
	for i, native := range []bool{true, false} {
		go func(native bool) {
			release, _, err := s.acquireResources(WithSchedulingScope(ctx, SchedulingScope{Actor: "actor", Background: native}), native)
			done <- admission{native, release, err}
		}(native)
		awaitGenerationQueue(t, s.admission, i+1)
	}
	s.mu.Lock()
	waiting, legacy := s.nativeWait, s.legacyRuns
	s.mu.Unlock()
	if waiting != 0 || legacy != 1 {
		t.Fatal("queued work occupied provider-family resources", waiting, legacy)
	}
	status := s.AdmissionStatus()
	if status.Admitted != 1 || status.ForegroundQueued != 1 || status.BackgroundQueued != 1 || status.Policy != schedulingPolicyVersion {
		t.Fatal("queue inspection disagrees with admission", status)
	}
	held()
	for _, wantNative := range []bool{false, true} {
		got := <-done
		if got.err != nil {
			t.Fatal(got.err)
		}
		got.release()
		if got.native != wantNative {
			t.Fatal("family affinity bypassed shared priority")
		}
	}
}

func TestSharedModelPreparationRetainsBackgroundSchedulingScope(t *testing.T) {
	s, err := NewProviderScheduler(NewProviderRegistry(), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	held, _, err := s.acquireResources(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	done := make(chan error, 1)
	go func() {
		release, _, err := s.AcquireLegacyModel(WithSchedulingScope(ctx, SchedulingScope{Actor: "alice", Work: "work", Background: true}), NewLifecycleGate(1), "model", 4096, nil, func(context.Context, string) error { return nil })
		if release != nil {
			release()
		}
		done <- err
	}()
	awaitGenerationQueue(t, s.admission, 1)
	s.admission.mu.Lock()
	actors := s.admission.classes[1]
	correct := len(actors) == 1 && actors[0].id == "alice" && actors[0].works[0].id == "work"
	s.admission.mu.Unlock()
	if !correct {
		t.Fatal("background model load lost owner or priority")
	}
	held()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestGenerationAdmissionCancellationRacesWithRelease(t *testing.T) {
	for i := 0; i < 100; i++ {
		q := &generationAdmission{capacity: 1, maxWait: 2}
		held, err := q.acquire(context.Background(), context.Background())
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			release, _ := q.acquire(ctx, context.Background())
			if release != nil {
				release()
				release()
			}
			close(done)
		}()
		awaitGenerationQueue(t, q, 1)
		go cancel()
		held()
		<-done
		q.mu.Lock()
		clean := q.active == 0 && q.counts[0]+q.counts[1] == 0
		q.mu.Unlock()
		if !clean {
			t.Fatal("cancellation leaked or duplicated admission")
		}
	}
}
