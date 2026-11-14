package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func awaitPreparation(t *testing.T, e WorkerPreparationEndpoint, req WorkingCopyRequest) WorkerPreparationStatus {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		status, err := e.InspectWorkerPreparation(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if status.State != "accepted" && status.State != "cancelling" {
			return status
		}
		select {
		case <-time.After(time.Millisecond):
		case <-ctx.Done():
			t.Fatal("preparation did not settle")
		}
	}
}

func TestWorkerPreparationAsyncCancellationAndRetry(t *testing.T) {
	e, _, b, _ := workingCopyFixture(t)
	req := workingCopyRequest(b, "cancel")
	ctx := context.Background()
	e.preparing.Lock()
	locked := true
	defer func() {
		if locked {
			e.preparing.Unlock()
		}
	}()
	status, err := e.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "accepted" {
		t.Fatal(status, err)
	}
	for i := 0; i < 3; i++ {
		status, err = e.BeginWorkerPreparation(ctx, req)
		if err != nil || status.State != "accepted" {
			t.Fatal(status, err)
		}
	}
	changed := req
	changed.ControlEpoch++
	if _, err = e.BeginWorkerPreparation(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry", err)
	}
	if _, err = e.InspectWorkerPreparation(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed inspection", err)
	}
	if _, err = e.Check(ctx, b); err != nil {
		t.Fatal("preparation blocked control", err)
	}
	status, err = e.CancelWorkerPreparation(ctx, req)
	if err != nil || status.State != "cancelling" {
		t.Fatal(status, err)
	}
	e.preparing.Unlock()
	locked = false
	status = awaitPreparation(t, e, req)
	if status.State != "cancelled" || status.Context != nil {
		t.Fatal(status)
	}
	status, err = e.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "cancelled" {
		t.Fatal("cancelled work revived", status, err)
	}
	if len(e.prepared) != 0 {
		t.Fatal("cancelled preparation published child")
	}
}

func TestWorkerPreparationParentStopWaitsForQuiescence(t *testing.T) {
	e, _, b, _ := workingCopyFixture(t)
	req := workingCopyRequest(b, "stop")
	e.preparing.Lock()
	_, err := e.BeginWorkerPreparation(context.Background(), req)
	if err != nil {
		e.preparing.Unlock()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	err = e.Cancel(ctx, b)
	cancel()
	e.preparing.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("Stop falsely acknowledged unsettled preparation", err)
	}
	if status := awaitPreparation(t, e, req); status.State != "cancelled" {
		t.Fatal(status)
	}
	if err = e.Cancel(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if _, err = e.BeginWorkerPreparation(context.Background(), workingCopyRequest(b, "late")); !errors.Is(err, ErrAuthority) {
		t.Fatal("stopped parent dispatched", err)
	}
}

func TestWorkerPreparationRoutesOnlyOwnedChildAndRetainsReceipt(t *testing.T) {
	e, _, b, root := workingCopyFixture(t)
	if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	req := workingCopyRequest(b, "ready")
	if _, err := e.BeginWorkerPreparation(ctx, req); err != nil {
		t.Fatal(err)
	}
	status := awaitPreparation(t, e, req)
	if status.State != "ready" || status.Context == nil || status.Record == nil {
		t.Fatal(status)
	}
	child := status.Context.Binding
	if err := e.Admit(ctx, child); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Binding){func(b *Binding) { b.Grant = "foreign" }, func(b *Binding) { b.Job = "foreign" }, func(b *Binding) { b.Actor = "foreign" }, func(b *Binding) { b.Revision++ }} {
		bad := child
		change(&bad)
		if err := e.Admit(ctx, bad); !errors.Is(err, ErrAuthority) {
			t.Fatal("widened child binding", err)
		}
	}
	intent, err := NewIntent(child, "write", "file_write", json.RawMessage(`{"path":"output.txt","data":"bmV3"}`), false)
	if err != nil {
		t.Fatal(err)
	}
	response, err := e.Call(ctx, intent)
	if err != nil || response.Error != "" || response.Receipt == nil || response.Receipt.State != "complete" {
		t.Fatal(response, err)
	}
	if _, err = os.Stat(filepath.Join(root, "output.txt")); !os.IsNotExist(err) {
		t.Fatal("mutated original", err)
	}
	response, err = e.InspectReceipt(ctx, intent)
	if err != nil || response.Receipt == nil || response.Receipt.State != "complete" {
		t.Fatal(response, err)
	}
	durable, err := e.Worker.Journal.Inspect(ctx, preparationIntent(req))
	if err != nil {
		t.Fatal(err)
	}
	var recorded WorkerPreparationStatus
	if err = json.Unmarshal(durable.Result, &recorded); err != nil || recorded.Context != nil {
		t.Fatal("persisted live grant", recorded, err)
	}
	// Loss of the live supervisor map cannot recreate an endpoint from a receipt.
	recovered, err := recoveredPreparation(durable)
	if err != nil || recovered.State != "interrupted" || recovered.Context != nil {
		t.Fatal(recovered, err)
	}
	if err = e.Cancel(ctx, child); err != nil {
		t.Fatal(err)
	}
	status, err = e.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "closed" || status.Context != nil {
		t.Fatal("stopped child revived", status, err)
	}
	response, err = e.Call(ctx, intent)
	if err == nil && response.Error == "" {
		t.Fatal("stopped child dispatched")
	}
}

func TestWorkerPreparationReceiptFailuresAndObservation(t *testing.T) {
	for _, stage := range []string{"claim", "finish"} {
		t.Run(stage, func(t *testing.T) {
			e, _, b, _ := workingCopyFixture(t)
			req := workingCopyRequest(b, stage)
			ctx := context.Background()
			status, err := e.InspectWorkerPreparation(ctx, req)
			if err != nil || status.State != "uncertain" || len(e.preparations) != 0 {
				t.Fatal("observation dispatched", status, err)
			}
			trigger := `CREATE TRIGGER preparation_failure BEFORE INSERT ON execution_operations BEGIN SELECT RAISE(FAIL,'fixture'); END`
			if stage == "finish" {
				trigger = `CREATE TRIGGER preparation_failure BEFORE UPDATE ON execution_operations BEGIN SELECT RAISE(FAIL,'fixture'); END`
			}
			if _, err = e.Worker.Journal.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			status, err = e.BeginWorkerPreparation(ctx, req)
			if stage == "claim" {
				if err == nil || len(e.preparations) != 0 {
					t.Fatal("unrecorded command launched", status, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			status = awaitPreparation(t, e, req)
			if status.State != "uncertain" || status.Context != nil {
				t.Fatal("unrecorded grant exposed", status)
			}
			for _, child := range e.prepared {
				select {
				case <-child.done:
				default:
					t.Fatal("unrecorded child remains live")
				}
			}
		})
	}
}
