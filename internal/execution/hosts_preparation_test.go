package execution

import (
	"context"
	"errors"
	"testing"
)

func TestHostRegistryPreparedChildrenScopeAndLifetime(t *testing.T) {
	e, store, b, _ := workingCopyFixture(t)
	e.Worker.Session.config.WorkerCopies = store.policy
	h := NewHosts()
	ctx := context.Background()
	config := e.Worker.Session.config
	if err := h.RegisterApproved(ctx, config, e); err != nil {
		t.Fatal(err)
	}
	ref := reference(b)
	if err := h.Admit(ctx, b.Actor, b.Workspace, b.Job, ref); err != nil {
		t.Fatal(err)
	}
	prepare := func(id string) (WorkingCopyRequest, Resource) {
		t.Helper()
		req := workingCopyRequest(b, id)
		if _, err := h.BeginPreparedWorker(ctx, b.Actor, b.Workspace, ref, req); err != nil {
			t.Fatal(err)
		}
		if status := awaitPreparation(t, e, req); status.State != "ready" {
			t.Fatal(status)
		}
		if _, err := h.InspectPreparedWorker(ctx, b.Actor, b.Workspace, ref, req); err != nil {
			t.Fatal(err)
		}
		if len(h.entries) != 1 && id == "one" {
			t.Fatal("inspection registered child")
		}
		resource, err := h.AdmitPreparedWorker(ctx, b.Actor, b.Workspace, ref, req)
		if err != nil {
			t.Fatal(err)
		}
		again, err := h.AdmitPreparedWorker(ctx, b.Actor, b.Workspace, ref, req)
		if err != nil || again != resource {
			t.Fatal("admission was not idempotent", again, err)
		}
		return req, resource
	}
	oneReq, one := prepare("one")
	twoReq, two := prepare("two")
	if len(h.List(ctx, b.Actor, b.Workspace)) != 1 {
		t.Fatal("private child exposed as general host selection")
	}
	if err := h.Admit(ctx, b.Actor, b.Workspace, "foreign-job", one.Reference); !errors.Is(err, ErrAuthority) {
		t.Fatal("child admitted foreign run", err)
	}
	if _, err := h.BeginPreparedWorker(ctx, "foreign", b.Workspace, ref, oneReq); err == nil {
		t.Fatal("foreign actor prepared child")
	}
	call := func(req WorkingCopyRequest, resource Resource) error {
		response, err := h.Call(ctx, b.Actor, b.Workspace, req.WorkerRunID, "info", resource.Reference, "workspace_info", []byte(`{}`))
		if err == nil && response.Error != "" {
			return errors.New(response.Error)
		}
		return err
	}
	if err := call(oneReq, one); err != nil {
		t.Fatal(err)
	}
	if err := h.Revoke(b.Actor, b.Workspace, one.Reference); err != nil {
		t.Fatal(err)
	}
	if err := call(twoReq, two); err != nil {
		t.Fatal("revoking child closed sibling transport", err)
	}
	if _, err := h.Resolve(ctx, b.Actor, b.Workspace, ref); err != nil {
		t.Fatal("revoking child closed root", err)
	}
	if err := h.Cancel(ctx, b.Actor, b.Workspace, b.Job, ref); err != nil {
		t.Fatal(err)
	}
	if err := call(twoReq, two); err == nil {
		t.Fatal("root Stop left child admission open")
	}
	if err := h.Admit(ctx, b.Actor, b.Workspace, b.Job, ref); err == nil {
		t.Fatal("root Stop admission reopened")
	}
	if _, err := h.BeginPreparedWorker(ctx, b.Actor, b.Workspace, ref, workingCopyRequest(b, "late")); err == nil {
		t.Fatal("stopped root prepared child")
	}
	if err := h.Cancel(ctx, b.Actor, b.Workspace, b.Job, ref); err != nil {
		t.Fatal("Stop retry lost acknowledgement", err)
	}
}

func TestHostRegistryRevocationRemovesPreparedDescendants(t *testing.T) {
	e, store, b, _ := workingCopyFixture(t)
	e.Worker.Session.config.WorkerCopies = store.policy
	h := NewHosts()
	ctx := context.Background()
	ref := reference(b)
	if err := h.RegisterApproved(ctx, e.Worker.Session.config, e); err != nil {
		t.Fatal(err)
	}
	if err := h.Admit(ctx, b.Actor, b.Workspace, b.Job, ref); err != nil {
		t.Fatal(err)
	}
	req := workingCopyRequest(b, "one")
	if _, err := h.BeginPreparedWorker(ctx, b.Actor, b.Workspace, ref, req); err != nil {
		t.Fatal(err)
	}
	awaitPreparation(t, e, req)
	resource, err := h.AdmitPreparedWorker(ctx, b.Actor, b.Workspace, ref, req)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Revoke(b.Actor, b.Workspace, ref); err != nil {
		t.Fatal(err)
	}
	if len(h.entries) != 0 {
		t.Fatal("orphan registrations retained")
	}
	if _, err = h.Resolve(ctx, b.Actor, b.Workspace, resource.Reference); err == nil {
		t.Fatal("orphan child resolved")
	}
}

func TestHostRegistryStopBeforeFirstAdmissionFencesDispatch(t *testing.T) {
	e, _, b, _ := workingCopyFixture(t)
	h := NewHosts()
	ctx := context.Background()
	ref := reference(b)
	if err := h.RegisterApproved(ctx, e.Worker.Session.config, e); err != nil {
		t.Fatal(err)
	}
	if err := h.CancelIfAdmitted(ctx, b.Actor, b.Workspace, "not-yet-admitted", ref); err != nil {
		t.Fatal(err)
	}
	if err := h.Admit(ctx, b.Actor, b.Workspace, "not-yet-admitted", ref); !errors.Is(err, ErrAuthority) {
		t.Fatal("late admission bypassed Stop", err)
	}
	if err := h.CancelIfAdmitted(ctx, b.Actor, b.Workspace, "not-yet-admitted", ref); err != nil {
		t.Fatal("cleanup retry", err)
	}
}
