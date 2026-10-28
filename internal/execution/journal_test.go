package execution

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestJournalDoesNotReplayUnknownEffects(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	j, err := OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, fresh, err := j.Claim(ctx, "operation-one", []byte("intent")); err != nil || !fresh {
		t.Fatal(fresh, err)
	}
	if err = j.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = OpenJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	r, fresh, err := j.Claim(ctx, "operation-one", []byte("intent"))
	if err != nil || fresh || r.State != "uncertain" {
		t.Fatal(r, fresh, err)
	}
	if _, _, err = j.Claim(ctx, "operation-one", []byte("changed")); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestJournalCompletionAndConcurrentAdmission(t *testing.T) {
	j, err := OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	ctx := context.Background()
	var freshCount atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, fresh, err := j.Claim(ctx, "shared", []byte("same"))
			if err != nil {
				t.Error(err)
			}
			if fresh {
				freshCount.Add(1)
			}
		}()
	}
	wg.Wait()
	if freshCount.Load() != 1 {
		t.Fatal(freshCount.Load())
	}
	if err = j.Finish(ctx, "shared", []byte("same"), Receipt{State: "complete", Result: json.RawMessage(`{"verified":true}`)}); err != nil {
		t.Fatal(err)
	}
	r, fresh, err := j.Claim(ctx, "shared", []byte("same"))
	if err != nil || fresh || r.State != "complete" || string(r.Result) != `{"verified":true}` {
		t.Fatal(r, fresh, err)
	}
	if err = j.Finish(ctx, "shared", []byte("same"), Receipt{State: "rejected"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	j.Close()
	if _, _, err = j.Claim(ctx, "not-dispatched", nil); err == nil {
		t.Fatal("closed journal admitted operation")
	}
}
