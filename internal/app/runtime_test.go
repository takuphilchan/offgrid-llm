package app

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestCleanupStackReleasesEachAcquisitionInReverseOrder(t *testing.T) {
	for failAt := 0; failAt <= 4; failAt++ {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			var stack cleanupStack
			released := make([]int, 0)
			cause := errors.New("acquire failed")
			cleanupFailure := errors.New("cleanup failed")
			for i := 0; i < failAt; i++ {
				index := i
				stack.add(fmt.Sprintf("resource-%d", index), func() error {
					released = append(released, index)
					if index == 0 {
						return cleanupFailure
					}
					return nil
				})
			}
			err := stack.rollback(cause)
			if !errors.Is(err, cause) {
				t.Fatalf("rollback lost primary error: %v", err)
			}
			if got := errors.Is(err, cleanupFailure); got != (failAt > 0) {
				t.Fatalf("cleanup error retained = %v, error = %v", got, err)
			}
			want := make([]int, 0, failAt)
			for i := failAt - 1; i >= 0; i-- {
				want = append(want, i)
			}
			if !reflect.DeepEqual(released, want) {
				t.Fatalf("release order = %v, want %v", released, want)
			}
			if err := stack.close(); err != nil || len(released) != failAt {
				t.Fatalf("rollback repeated release: err=%v releases=%v", err, released)
			}
		})
	}
}

func TestCleanupStackPromotesCancellationBeforeProviderShutdown(t *testing.T) {
	var stack cleanupStack
	var calls []string
	for _, name := range []string{"provider scheduler", "runtime context", "workspace ownership"} {
		owner := name
		stack.add(owner, func() error { calls = append(calls, owner); return nil })
	}
	if err := stack.promote("provider scheduler"); err != nil {
		t.Fatal(err)
	}
	if err := stack.promote("runtime context"); err != nil {
		t.Fatal(err)
	}
	if err := stack.close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"runtime context", "provider scheduler", "workspace ownership"}) {
		t.Fatalf("unsafe inference shutdown order: %v", calls)
	}
	if err := stack.promote("missing"); err == nil {
		t.Fatal("missing cleanup owner was accepted")
	}
}

func TestCleanupStackRetainsModelAndWorkspaceAfterProviderDrainFailure(t *testing.T) {
	var stack cleanupStack
	var calls []string
	blocked := errors.New("provider drain uncertain")
	stack.add("workspace", func() error { calls = append(calls, "workspace"); return nil })
	stack.add("model cache", func() error { calls = append(calls, "model cache"); return nil })
	stack.addBarrier("provider scheduler", func() error { calls = append(calls, "provider scheduler"); return blocked })
	stack.add("runtime context", func() error { calls = append(calls, "runtime context"); return nil })
	if err := stack.close(); !errors.Is(err, blocked) {
		t.Fatalf("provider drain failure was lost: %v", err)
	}
	if !reflect.DeepEqual(calls, []string{"runtime context", "provider scheduler"}) {
		t.Fatalf("provider failure released resources under an active lease: %v", calls)
	}
	if len(stack.steps) != 3 {
		t.Fatalf("remaining owners were discarded: %v", stack.steps)
	}
}

func TestRuntimeCloseIsIdempotentAndJoinsCleanupErrors(t *testing.T) {
	var calls []string
	first := errors.New("first")
	second := errors.New("second")
	r := &Runtime{}
	r.owned.add("first", func() error { calls = append(calls, "first"); return first })
	r.owned.add("second", func() error { calls = append(calls, "second"); return second })
	err := r.Close()
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("close lost cleanup errors: %v", err)
	}
	if again := r.Close(); again != err {
		t.Fatalf("second close returned different error: %v", again)
	}
	if !reflect.DeepEqual(calls, []string{"second", "first"}) {
		t.Fatalf("cleanup calls = %v", calls)
	}
}
