package execution

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestEnrolledWorkerPreparationAndChildDispatch(t *testing.T) {
	enrollment, endpoint, config, pack, token := enrollmentFixture(t)
	config.WorkerCopies = WorkingCopyPolicy{Version: 1, MaxCopyBytes: 1024, MaxTotalBytes: 2048, MaxEntries: 100, MaxCopies: 2}
	endpoint.Worker.Session.config = config
	store, err := OpenWorkingCopies(filepath.Join(t.TempDir(), "copies"), config.WorkerCopies)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.WorkingCopies = store
	t.Cleanup(func() { endpoint.Close(); store.Close() })
	ref := onlineEnrollment(t, enrollment, endpoint, config, pack, token)
	serveEnrollmentFixture(t, enrollment, endpoint, config, token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	peer := enrollment.peers[ref.Session].peer
	parent := config.Binding
	parent.Job = "parent"
	if err = peer.Admit(ctx, parent); err != nil {
		t.Fatal(err)
	}
	req := workingCopyRequest(parent, "enrolled")
	req.Permissions = config.Permissions
	status, err := peer.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "accepted" {
		t.Fatal(status, err)
	}
	status = awaitPreparation(t, peer, req)
	if status.State != "ready" || status.Context == nil {
		t.Fatal(status)
	}
	child := status.Context.Binding
	if _, err = peer.Check(ctx, child); err != nil {
		t.Fatal(err)
	}
	intent, err := NewIntent(child, "info", "workspace_info", json.RawMessage(`{}`), false)
	if err != nil {
		t.Fatal(err)
	}
	response, err := peer.Call(ctx, intent)
	if err != nil || response.Error != "" || response.Receipt == nil {
		t.Fatal(response, err)
	}
	if err = peer.Admit(ctx, child); err != nil {
		t.Fatal(err)
	}
	foreign := child
	foreign.Job = "foreign"
	if err = peer.Admit(ctx, foreign); err == nil {
		t.Fatal("child root membership admitted another run")
	}
	changed := req
	changed.InstructionVersion++
	if _, err = peer.BeginWorkerPreparation(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("conflicting retry lost", err)
	}
	if err = peer.Cancel(ctx, child); err != nil {
		t.Fatal(err)
	}
	status, err = peer.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "closed" || status.Context != nil {
		t.Fatal(status, err)
	}
	if err = peer.Cancel(ctx, parent); err != nil {
		t.Fatal(err)
	}
}

func TestBridgePreparationCannotBypassApprovalOrPublishForeignContext(t *testing.T) {
	e, _, b, _ := workingCopyFixture(t)
	req := workingCopyRequest(b, "bridge")
	intent := preparationIntent(req)
	message := BridgeMessage{Version: ProtocolVersion, ID: "bridge", Kind: "worker", Binding: b, Request: &intent}
	approved := e.Worker.Session.config
	reply := DispatchBridgeMessage(context.Background(), e, approved, message)
	if reply.Error != ErrAuthority.Error() || len(e.preparations) != 0 {
		t.Fatal("unapproved worker policy", reply)
	}
	approved.WorkerCopies = DefaultWorkingCopyPolicy()
	message.Kind = "call"
	reply = DispatchBridgeMessage(context.Background(), e, approved, message)
	if reply.Error != ErrAuthority.Error() {
		t.Fatal("model route dispatched supervisor operation", reply)
	}
	message.Kind = "worker"
	if _, err := e.BeginWorkerPreparation(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	status := awaitPreparation(t, e, req)
	if status.Context == nil {
		t.Fatal(status)
	}
	p := &Peer{config: approved}
	for _, mutate := range []func(*Context){func(c *Context) { c.Binding.Actor = "foreign" }, func(c *Context) { c.Binding.Job = "foreign" }, func(c *Context) { c.Root = approved.Root }, func(c *Context) { c.WorkerCopies = DefaultWorkingCopyPolicy() }} {
		bad := *status.Context
		mutate(&bad)
		copy := status
		copy.Context = &bad
		data, _ := json.Marshal(copy)
		err := p.recordPreparedChild(message, &Response{Receipt: &Receipt{State: "complete", Result: data}})
		if !errors.Is(err, ErrAuthority) || len(p.children) != 0 {
			t.Fatal("foreign context published", err)
		}
	}
}
