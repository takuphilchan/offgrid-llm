package execution

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestParentCancelWaitsForEveryDurableChildOutcome(t *testing.T) {
	endpoint, store, binding, _ := workingCopyFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var children []*PreparedWorker
	for _, name := range []string{"first", "second"} {
		child, err := endpoint.PrepareWorker(ctx, workingCopyRequest(binding, name))
		if err != nil {
			t.Fatal(err)
		}
		// Deterministic native ownership loss. Real native descendant fixtures
		// separately qualify how a platform enters this state.
		child.Endpoint.Worker.Processes.cleanupUnproven = true
		children = append(children, child)
	}
	endpoint.Worker.Processes.cleanupUnproven = true
	store.mu.Lock()
	locked := true
	defer func() {
		if locked {
			store.mu.Unlock()
		}
	}()
	result := make(chan error, 1)
	go func() { result <- endpoint.Cancel(ctx, binding) }()
	for _, child := range children {
		select {
		case <-child.Endpoint.Worker.Session.revokedSignal:
		case <-ctx.Done():
			t.Fatal("child shutdown did not start")
		}
		// Revocation occurs inside the once block: wait until native resource
		// closure finishes while the durable write remains deliberately blocked.
		child.once.Do(func() { t.Error("child resource closure was not started") })
		select {
		case <-child.done:
			t.Fatal("child acknowledged completion before its durable outcome")
		default:
		}
	}
	select {
	case err := <-result:
		t.Fatal("parent returned before all durable child outcomes", err)
	default:
	}
	store.mu.Unlock()
	locked = false
	select {
	case err := <-result:
		if !errors.Is(err, ErrHostUnavailable) {
			t.Fatal("parent lost unresolved cleanup error", err)
		}
	case <-ctx.Done():
		t.Fatal("parent failed to acknowledge persisted child outcomes")
	}
	for _, child := range children {
		record, err := store.Inspect(ctx, child.Record.ID, binding)
		if err != nil || record.State != "uncertain" || record.ReservedBytes != child.Record.ReservedBytes {
			t.Fatal("parent acknowledged before retained uncertainty", record, err)
		}
		owner, err := store.copyOwnership(ctx, child.Record.ID)
		if err != nil || !owner.InUse || owner.Detached {
			t.Fatal("parent refunded unresolved child ownership", owner, err)
		}
	}
}
