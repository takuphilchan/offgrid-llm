package execution

import (
	"context"
	"errors"
	"testing"
)

func TestWorkerPreparationStopBeforeBeginLeavesDurableTombstone(t *testing.T) {
	e, _, binding, _ := workingCopyFixture(t)
	req := workingCopyRequest(binding, "stopped-before-begin")
	ctx := context.Background()
	status, err := e.CancelWorkerPreparation(ctx, req)
	if err != nil || status.State != "cancelled" {
		t.Fatal(status, err)
	}
	status, err = e.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "cancelled" || status.Context != nil || status.Record != nil {
		t.Fatal("late begin revived preparation", status, err)
	}
	changed := req
	changed.ControlEpoch++
	if _, err = e.CancelWorkerPreparation(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed cancellation identity accepted", err)
	}
	e.mu.Lock()
	count := len(e.prepared)
	e.mu.Unlock()
	if count != 0 {
		t.Fatal("cancelled request created a worker")
	}
	// Reconstructing the endpoint loses live authority and maps, but its same
	// private journal must still reject replay of the cancelled preparation.
	restarted := NewLocalEndpoint(e.Worker)
	restarted.WorkingCopies = e.WorkingCopies
	if err = restarted.Admit(ctx, binding); err != nil {
		t.Fatal(err)
	}
	status, err = restarted.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "cancelled" {
		t.Fatal("restart revived preparation", status, err)
	}
	restarted.Close()
}

func TestStopPreparedWorkerBeforeServiceRegistrationPreservesParent(t *testing.T) {
	e, _, binding, _ := workingCopyFixture(t)
	ctx := context.Background()
	config, err := e.Check(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture store was installed explicitly by its test supervisor.
	config.WorkerCopies = e.WorkingCopies.policy
	e.Worker.Session.mu.Lock()
	e.Worker.Session.config.WorkerCopies = config.WorkerCopies
	e.Worker.Session.mu.Unlock()
	hosts := NewHosts()
	if err = hosts.RegisterApproved(ctx, config, e); err != nil {
		t.Fatal(err)
	}
	ref := reference(binding)
	if err = hosts.Admit(ctx, binding.Actor, binding.Workspace, binding.Job, ref); err != nil {
		t.Fatal(err)
	}
	req := workingCopyRequest(binding, "unregistered-child")
	if _, err = hosts.BeginPreparedWorker(ctx, binding.Actor, binding.Workspace, ref, req); err != nil {
		t.Fatal(err)
	}
	status := awaitPreparation(t, e, req)
	if status.State != "ready" {
		t.Fatal(status)
	}
	if err = hosts.StopPreparedWorker(ctx, binding.Actor, binding.Workspace, ref, req); err != nil {
		t.Fatal(err)
	}
	if _, err = hosts.AdmitPreparedWorker(ctx, binding.Actor, binding.Workspace, ref, req); err == nil {
		t.Fatal("stopped worker admitted")
	}
	if _, err = hosts.Call(ctx, binding.Actor, binding.Workspace, binding.Job, "parent-inspect", ref, "workspace_info", []byte(`{}`)); err != nil {
		t.Fatal("worker stop revoked parent", err)
	}
}
