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

func awaitWorkerSeal(t *testing.T, broker WorkerArtifactEndpoint, req WorkerArtifactRequest) WorkerArtifactReply {
	t.Helper()
	req.Operation = "seal_status"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		reply, err := broker.WorkerArtifact(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if reply.State != "accepted" {
			return reply
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
}

func TestWorkerArtifactsAsyncSealRetrievalAndAuthority(t *testing.T) {
	e, store, b, source := workingCopyFixture(t)
	ctx := context.Background()
	copyReq := workingCopyRequest(b, "artifact")
	w, err := e.PrepareWorker(ctx, copyReq)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(w.Context.Root, "output.txt"), []byte("sealed output"), 0600); err != nil {
		t.Fatal(err)
	}
	req := WorkerArtifactRequest{Operation: "seal", Source: b, Seal: &SealChangeSetRequest{RequestID: "capture", CopyID: w.Record.ID, Source: b, InstructionVersion: copyReq.InstructionVersion, ControlEpoch: copyReq.ControlEpoch}}
	stale := *req.Seal
	stale.ControlEpoch++
	bad := req
	bad.Seal = &stale
	if _, err = e.WorkerArtifact(ctx, bad); !errors.Is(err, ErrRevision) {
		t.Fatal("stale capture accepted", err)
	}
	if _, err = w.Endpoint.Check(ctx, w.Context.Binding); err != nil {
		t.Fatal("stale capture closed live worker", err)
	}
	w.Endpoint.dispatch.Lock()
	locked := true
	defer func() {
		if locked {
			w.Endpoint.dispatch.Unlock()
		}
	}()
	reply, err := e.WorkerArtifact(ctx, req)
	if err != nil || reply.State != "accepted" {
		t.Fatal(reply, err)
	}
	if _, err = e.Check(ctx, b); err != nil {
		t.Fatal("capture blocked root control", err)
	}
	again, err := e.WorkerArtifact(ctx, req)
	if err != nil || again.State != "accepted" {
		t.Fatal("retry restarted capture", again, err)
	}
	w.Endpoint.dispatch.Unlock()
	locked = false
	reply = awaitWorkerSeal(t, e, req)
	if reply.State != "sealed" || reply.ChangeSet == nil {
		t.Fatal(reply)
	}
	id := reply.ChangeSet.ID
	reply.ChangeSet.ID = "mutated caller snapshot"
	again, err = e.WorkerArtifact(ctx, req)
	if err != nil || again.ChangeSet.ID != id {
		t.Fatal("caller mutated cached seal", again, err)
	}
	read := WorkerArtifactRequest{Operation: "inspect", Source: b, ChangeSetID: id, Limit: 64}
	page, err := e.WorkerArtifact(ctx, read)
	if err != nil || page.Page == nil || len(page.Page.Operations) != 1 {
		t.Fatal(page, err)
	}
	read.Operation, read.Hash, read.Limit = "blob", page.Page.Operations[0].Result.SHA256, MaxPageBytes
	blob, err := e.WorkerArtifact(ctx, read)
	if err != nil || string(blob.Blob.Data) != "sealed output" {
		t.Fatal(blob, err)
	}
	store.mu.Lock()
	_, err = e.WorkerArtifact(ctx, read)
	store.mu.Unlock()
	if !errors.Is(err, ErrConflict) {
		t.Fatal("busy store blocked artifact transport", err)
	}
	foreign := read
	foreign.Source.Actor = "foreign"
	if _, err = e.WorkerArtifact(ctx, foreign); !errors.Is(err, ErrAuthority) {
		t.Fatal("foreign artifact exposed", err)
	}
	r, _ := artifactRequest(read)
	r.Token = e.Worker.Token
	response := e.supervisor(ctx, r)
	if response.Error != "" {
		t.Fatal(response.Error)
	}
	if _, err = NewIntent(b, "model", artifactOperation, r.Arguments, false); err == nil {
		t.Fatal("model acquired supervisor artifact operation")
	}
	var decoded WorkerArtifactReply
	if json.Unmarshal(response.Receipt.Result, &decoded) != nil || string(decoded.Blob.Data) != "sealed output" {
		t.Fatal("private supervisor response lost bytes")
	}
	if _, err = os.Stat(filepath.Join(source, "output.txt")); !os.IsNotExist(err) {
		t.Fatal("capture edited original")
	}
}

func TestEnrolledWorkerArtifactsUsePrivateBridge(t *testing.T) {
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
	peer := enrollment.peers[ref.Session].peer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	parent := config.Binding
	parent.Job = "parent"
	if err = peer.Admit(ctx, parent); err != nil {
		t.Fatal(err)
	}
	copyReq := workingCopyRequest(parent, "bridge-artifact")
	copyReq.Permissions = config.Permissions
	if _, err = peer.BeginWorkerPreparation(ctx, copyReq); err != nil {
		t.Fatal(err)
	}
	prepared := awaitPreparation(t, peer, copyReq)
	if prepared.State != "ready" {
		t.Fatal(prepared)
	}
	if err = os.WriteFile(filepath.Join(prepared.Context.Root, "output.txt"), []byte("bridge output"), 0600); err != nil {
		t.Fatal(err)
	}
	req := WorkerArtifactRequest{Operation: "seal", Source: parent, Seal: &SealChangeSetRequest{RequestID: "seal", CopyID: prepared.Record.ID, Source: parent, InstructionVersion: copyReq.InstructionVersion, ControlEpoch: copyReq.ControlEpoch}}
	if _, err = peer.WorkerArtifact(ctx, req); err != nil {
		t.Fatal(err)
	}
	reply := awaitWorkerSeal(t, peer, req)
	if reply.State != "sealed" {
		t.Fatal(reply)
	}
	page, err := peer.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "inspect", Source: parent, ChangeSetID: reply.ChangeSet.ID, Limit: 64})
	if err != nil || page.Page == nil || len(page.Page.Operations) != 1 {
		t.Fatal(page, err)
	}
	blob, err := peer.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "blob", Source: parent, ChangeSetID: reply.ChangeSet.ID, Hash: page.Page.Operations[0].Result.SHA256, Limit: MaxPageBytes})
	if err != nil || blob.Blob == nil || string(blob.Blob.Data) != "bridge output" {
		t.Fatal(blob, err)
	}
}
