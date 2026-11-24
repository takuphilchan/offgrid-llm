package execution

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestHostsScopeAdmissionReceiptsAndRevocation(t *testing.T) {
	w, b, root := workerFixture(t)
	e := NewLocalEndpoint(w)
	t.Cleanup(func() { e.Close() })
	h := NewHosts()
	ctx := context.Background()
	config, err := w.Session.Snapshot(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.RegisterApproved(ctx, config, e); err != nil {
		t.Fatal(err)
	}
	ref := reference(b)
	for _, scope := range [][2]string{{"bob", b.Workspace}, {b.Actor, "another-workspace"}} {
		if len(h.List(ctx, scope[0], scope[1])) != 0 {
			t.Fatal("cross-owner disclosure")
		}
		if _, err = h.Resolve(ctx, scope[0], scope[1], ref); err == nil {
			t.Fatal("cross-owner authority")
		}
	}
	data, _ := json.Marshal(map[string]any{"path": "exact.txt", "data": []byte("café 日本語")})
	if _, err = h.Call(ctx, b.Actor, b.Workspace, "new-job", "write", ref, "file_write", data); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
	if err = h.Admit(ctx, b.Actor, b.Workspace, "new-job", ref); err != nil {
		t.Fatal(err)
	}
	first, err := h.Call(ctx, b.Actor, b.Workspace, "new-job", "write", ref, "file_write", data)
	if err != nil || first.Receipt == nil || first.Receipt.State != "complete" {
		t.Fatal(first, err)
	}
	if err = os.WriteFile(filepath.Join(root, "exact.txt"), []byte("changed by user"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err := h.Call(ctx, b.Actor, b.Workspace, "new-job", "write", ref, "file_write", data)
	if err != nil || string(first.Receipt.Result) != string(again.Receipt.Result) {
		t.Fatal(again, err)
	}
	actual, _ := os.ReadFile(filepath.Join(root, "exact.txt"))
	if string(actual) != "changed by user" {
		t.Fatal("duplicate operation replayed")
	}
	changed, _ := json.Marshal(map[string]any{"path": "other.txt", "data": []byte("changed")})
	conflict, err := h.Call(ctx, b.Actor, b.Workspace, "new-job", "write", ref, "file_write", changed)
	if err != nil || conflict.Error != ErrConflict.Error() {
		t.Fatal(conflict, err)
	}
	if err = h.Revoke(b.Actor, b.Workspace, ref); err != nil {
		t.Fatal(err)
	}
	if len(h.List(ctx, b.Actor, b.Workspace)) != 0 {
		t.Fatal("revoked host listed")
	}
	if _, err = h.Call(ctx, b.Actor, b.Workspace, "new-job", "next", ref, "file_list", []byte(`{}`)); err == nil {
		t.Fatal("revoked dispatch")
	}
}

func TestHostOperationIdentityCannotAliasColonSeparatedIDs(t *testing.T) {
	w, b, _ := workerFixture(t)
	e := NewLocalEndpoint(w)
	t.Cleanup(func() { e.Close() })
	h := NewHosts()
	ctx := context.Background()
	config, _ := w.Session.Snapshot(ctx, b)
	if err := h.RegisterApproved(ctx, config, e); err != nil {
		t.Fatal(err)
	}
	ref := reference(b)
	for _, pair := range [][2]string{{"job:a", "b"}, {"job", "a:b"}} {
		if err := h.Admit(ctx, b.Actor, b.Workspace, pair[0], ref); err != nil {
			t.Fatal(err)
		}
		// Use a portable filename while preserving the ambiguous identity pair.
		args, _ := json.Marshal(map[string]any{"path": pair[0][0:1] + pair[1][0:1] + ".txt", "data": []byte("x")})
		got, err := h.Call(ctx, b.Actor, b.Workspace, pair[0], pair[1], ref, "file_write", args)
		if err != nil || got.Error != "" || got.Receipt.State != "complete" {
			t.Fatal(got, err)
		}
	}
}

func TestLocalEndpointAdmissionDoesNotAcceptForgedExistingJob(t *testing.T) {
	w, b, _ := workerFixture(t)
	e := NewLocalEndpoint(w)
	t.Cleanup(func() { e.Close() })
	if err := e.Admit(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	b.Actor = "bob"
	if err := e.Admit(context.Background(), b); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
}

func TestLocalEndpointBackgroundJobOutlivesStartRequestButCannotRestartAfterCancel(t *testing.T) {
	for _, unproven := range []bool{false, true} {
		name := "native cleanup"
		if unproven {
			name = "unproven cleanup"
		}
		t.Run(name, func(t *testing.T) { testBackgroundJobCancellation(t, unproven) })
	}
}

func testBackgroundJobCancellation(t *testing.T, forceUnproven bool) {
	w, b, _ := workerFixture(t)
	if forceUnproven {
		// Exercise the real owned process but withhold its cleanup proof. This
		// deterministically covers revoked receipt authority on every platform.
		w.Session.startProcess = func(command *exec.Cmd) (func() error, func() error, func() error, error) {
			stop, wait, release, err := startOwned(command)
			if err != nil {
				return stop, wait, release, err
			}
			return stop, wait, func() error { return errors.Join(release(), ErrHostUnavailable) }, nil
		}
	}
	unproven := forceUnproven || runtime.GOOS == "darwin"
	w.Session.config.Permissions.Processes = true
	w.AuthorizeProcess = func(context.Context, Binding, ProcessSpec, bool) error { return nil } // isolated fixture approval, not production policy
	e := NewLocalEndpoint(w)
	t.Cleanup(func() { e.Close() })
	if err := e.Admit(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	requestctx, endRequest := context.WithCancel(ctx)
	spec := ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessFixtureHelper$", "--", "offgrid-stream-fixture"}}
	r := request(w, b, "process_start", "background-once", spec)
	response, err := e.Call(requestctx, r)
	endRequest()
	if err != nil || response.Receipt == nil || response.Receipt.State != "dispatched" {
		t.Fatal(response, err)
	}
	var process ProcessResult
	if err = json.Unmarshal(response.Receipt.Result, &process); err != nil {
		t.Fatal(err)
	}
	actual, err := w.Processes.Inspect(ctx, b, process.Handle)
	if err != nil || actual.State != "running" {
		t.Fatal("request completion killed background job", actual, err)
	}
	if err = e.Cancel(ctx, b); unproven {
		if !errors.Is(err, ErrHostUnavailable) || w.Processes.cleanupConfirmed() {
			t.Fatal("cancellation invented descendant retirement", err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	// Cancellation waits for the owned supervisor. Public inspection requires
	// live authority and can already be revoked by receipt settlement below.
	w.Processes.mu.Lock()
	owned := w.Processes.processes[process.Handle]
	w.Processes.mu.Unlock()
	if owned == nil || owned.binding != b {
		t.Fatal("cancelled command lost its exact owner")
	}
	select {
	case <-owned.done:
	default:
		t.Fatal("stop acknowledged before supervisor termination")
	}
	actual = owned.snapshot()
	if actual.State != "cancelled" || actual.ExitCode == nil {
		t.Fatal("stop acknowledged before termination", actual)
	}
	receiptDone := make(chan struct{})
	go func() { w.finishing.Wait(); close(receiptDone) }()
	select {
	case <-receiptDone:
	case <-ctx.Done():
		t.Fatal("cancelled command receipt did not settle", ctx.Err())
	}
	_, inspectErr := w.Processes.Inspect(ctx, b, process.Handle)
	wantReceipt := "complete"
	if unproven {
		wantReceipt = "uncertain"
		if !errors.Is(inspectErr, ErrAuthority) || ProcessDescendantsSettled(actual) || ProcessDescendantState(actual) != "unknown" {
			t.Fatal("unproven cleanup retained inspection authority or invented settlement", actual, inspectErr)
		}
	} else if inspectErr != nil || !ProcessDescendantsSettled(actual) {
		t.Fatal("verified cancellation lost its terminal observation", actual, inspectErr)
	}
	receipt, receiptErr := w.Journal.Inspect(ctx, r)
	var persisted ProcessResult
	if receiptErr != nil || receipt.State != wantReceipt || json.Unmarshal(receipt.Result, &persisted) != nil || persisted.Handle != actual.Handle || persisted.State != "cancelled" || persisted.ExitCode == nil || *persisted.ExitCode != *actual.ExitCode || ProcessDescendantsSettled(persisted) != !unproven {
		t.Fatal("cancellation lost its durable terminal outcome", receipt, persisted, receiptErr)
	}
	if err = e.Admit(ctx, b); !errors.Is(err, ErrAuthority) {
		t.Fatal("cancelled job resurrected", err)
	}
	if _, err = e.Call(ctx, r); err == nil {
		t.Fatal("new dispatch after stop")
	}
}

type invalidReplyEndpoint struct {
	HostEndpoint
	receipt *Receipt
	failure string
}

func (e invalidReplyEndpoint) Call(_ context.Context, r Request) (Response, error) {
	return Response{Version: 1, ID: r.ID, Receipt: e.receipt, Error: e.failure}, nil
}

func TestHostRejectsMalformedReceiptsWithoutProjectingSuccess(t *testing.T) {
	for _, value := range []invalidReplyEndpoint{
		{receipt: &Receipt{State: "done", Result: json.RawMessage(`{}`)}},
		{receipt: &Receipt{State: "complete"}},
		{receipt: &Receipt{State: "complete", Result: json.RawMessage(`{"x":1,"x":2}`)}},
		{receipt: &Receipt{State: "complete", Result: json.RawMessage(`{}`)}, failure: "failed"},
		{receipt: &Receipt{State: "dispatched", Result: json.RawMessage(`{}`)}},
	} {
		w, b, _ := workerFixture(t)
		endpoint := NewLocalEndpoint(w)
		t.Cleanup(func() { endpoint.Close() })
		value.HostEndpoint = endpoint
		h := NewHosts()
		ctx := context.Background()
		config, _ := w.Session.Snapshot(ctx, b)
		if err := h.RegisterApproved(ctx, config, value); err != nil {
			t.Fatal(err)
		}
		ref := reference(b)
		if err := h.Admit(ctx, b.Actor, b.Workspace, "job", ref); err != nil {
			t.Fatal(err)
		}
		if _, err := h.Call(ctx, b.Actor, b.Workspace, "job", "read", ref, "file_list", []byte(`{}`)); !errors.Is(err, ErrProtocol) {
			t.Fatal("invalid receipt accepted", value.receipt, err)
		}
	}
}
