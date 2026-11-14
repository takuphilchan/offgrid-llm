package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegrationCancelBeforeBeginPreventsDelayedDispatch(t *testing.T) {
	e, store, binding, source, _, req := integrationFixture(t)
	before, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	request := WorkerArtifactRequest{Operation: "integration_cancel", Source: binding, Integrate: &req}
	reply, err := e.WorkerArtifact(context.Background(), request)
	if err != nil || reply.State != "cancelled" {
		t.Fatal(reply, err)
	}
	for _, operation := range []string{"integrate", "integration_status", "integration_cancel"} {
		request.Operation = operation
		reply, err = e.WorkerArtifact(context.Background(), request)
		if err != nil || reply.State != "cancelled" {
			t.Fatal(operation, reply, err)
		}
	}
	var count int
	if err = store.db.QueryRow(`SELECT COUNT(*) FROM worker_integrations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("cancelled intent dispatched", count, err)
	}
	after, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(after) != string(before) {
		t.Fatal("cancel changed original", err)
	}
	e.mu.Lock()
	alive := e.contexts[binding.Job] != nil && e.contexts[binding.Job].Err() == nil
	e.mu.Unlock()
	if !alive {
		t.Fatal("specific cancellation revoked the parent")
	}
}

func TestIntegrationCancellationWaitsForNativeSettlement(t *testing.T) {
	for _, global := range []bool{false, true} {
		t.Run(map[bool]string{false: "specific", true: "job_stop"}[global], func(t *testing.T) {
			e, store, binding, _, _, req := integrationFixture(t)
			store.mu.Lock()
			reply, err := e.WorkerArtifact(context.Background(), WorkerArtifactRequest{Operation: "integrate", Source: binding, Integrate: &req})
			if err != nil || reply.State != "accepted" {
				store.mu.Unlock()
				t.Fatal(reply, err)
			}
			e.mu.Lock()
			done := e.seals[integrationIntent(req).OperationID].done
			e.mu.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			if global {
				err = e.Cancel(ctx, binding)
			} else {
				_, err = e.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "integration_cancel", Source: binding, Integrate: &req})
			}
			cancel()
			store.mu.Unlock()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("acknowledged before native settlement", err)
			}
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("cancelled integration did not settle")
			}
			e.mu.Lock()
			alive := e.contexts[binding.Job] != nil && e.contexts[binding.Job].Err() == nil
			e.mu.Unlock()
			if alive == global {
				t.Fatal("incorrect parent lifetime after cancellation", alive)
			}
		})
	}
}
