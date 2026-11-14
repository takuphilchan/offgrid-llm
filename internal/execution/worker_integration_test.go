package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegrationPrivateProtocolIsAsyncAndObservationOnly(t *testing.T) {
	e, store, binding, source, _, req := integrationFixture(t)
	ctx := context.Background()
	request := WorkerArtifactRequest{Operation: "integration_status", Source: binding, Integrate: &req}
	if reply, err := e.WorkerArtifact(ctx, request); err != nil || reply.State != "uncertain" || reply.Integration != nil {
		t.Fatal("missing receipt was not uncertain", reply, err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM worker_integrations`).Scan(&count); err != nil || count != 0 {
		t.Fatal("observation dispatched", count, err)
	}
	store.mu.Lock()
	request.Operation = "integrate"
	started := time.Now()
	reply, err := e.WorkerArtifact(ctx, request)
	if err != nil || reply.State != "accepted" || time.Since(started) > time.Second {
		store.mu.Unlock()
		t.Fatal("integration admission blocked", reply, err)
	}
	request.Operation = "integration_status"
	reply, err = e.WorkerArtifact(ctx, request)
	if err != nil || reply.State != "accepted" {
		store.mu.Unlock()
		t.Fatal("status blocked", reply, err)
	}
	// The source session remains inspectable while native integration waits.
	if _, err = e.Worker.Session.Snapshot(ctx, binding); err != nil {
		store.mu.Unlock()
		t.Fatal(err)
	}
	store.mu.Unlock()
	deadline := time.Now().Add(10 * time.Second)
	for reply.State == "accepted" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		reply, err = e.WorkerArtifact(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
	}
	if reply.State != "integrated" || reply.Integration == nil || reply.Integration.Integrated != 5 {
		t.Fatal(reply)
	}
	id := reply.Integration.ID
	// Mutating a returned projection cannot change the cached receipt.
	reply.Integration.Files[0].Path = "forged"
	again, err := e.WorkerArtifact(ctx, request)
	if err != nil || again.Integration.Files[0].Path == "forged" {
		t.Fatal("receipt alias", again, err)
	}
	page, err := e.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "integration", Source: binding, IntegrationID: id, Limit: 1})
	if err != nil || page.Integration == nil || page.Integration.NextOffset == nil {
		t.Fatal(page, err)
	}
	readIntent, err := artifactRequest(WorkerArtifactRequest{Operation: "integration", Source: binding, IntegrationID: id, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := dispatchWorkerArtifact(ctx, e, &readIntent)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeArtifactResponse(encoded)
	if err != nil || decoded.Integration == nil || decoded.Integration.ID != id || len(decoded.Integration.Files) != 1 {
		t.Fatal("private wire lost integration receipt", decoded, err)
	}
	writeCandidateFile(t, source, "input.txt", "after completion")
	e.mu.Lock()
	delete(e.seals, integrationIntent(req).OperationID)
	e.mu.Unlock()
	request.Operation = "integrate"
	again, err = e.WorkerArtifact(ctx, request)
	if err != nil || again.State != "integrated" {
		t.Fatal("lost in-memory receipt repeated integration", again, err)
	}
	data, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "after completion" {
		t.Fatal("replayed mutation", string(data), err)
	}
	intent, _ := artifactRequest(request)
	if _, err = NewIntent(binding, "model", artifactOperation, intent.Arguments, false); err == nil {
		t.Fatal("model acquired private integration")
	}
	foreign := request
	foreign.Source.Actor = "foreign"
	if _, err = e.WorkerArtifact(ctx, foreign); err == nil {
		t.Fatal("foreign integration accepted")
	}
}

func TestIntegrationLostOuterReceiptUsesPerFileEvidence(t *testing.T) {
	e, store, binding, source, _, req := integrationFixture(t)
	ctx := context.Background()
	intent := integrationIntent(req)
	if _, _, err := e.Worker.Journal.Claim(ctx, intent.OperationID, intent.Payload()); err != nil {
		t.Fatal(err)
	}
	page, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
	if err != nil {
		t.Fatal(err)
	}
	writeCandidateFile(t, source, "input.txt", "new user edit")
	reply, err := e.WorkerArtifact(ctx, WorkerArtifactRequest{Operation: "integrate", Source: binding, Integrate: &req})
	if err != nil || reply.Integration == nil || reply.Integration.ID != page.ID || reply.State != "integrated" {
		t.Fatal("lost ack discarded file receipts", reply, err)
	}
	data, err := os.ReadFile(filepath.Join(source, "input.txt"))
	if err != nil || string(data) != "new user edit" {
		t.Fatal("lost ack replayed writes", string(data), err)
	}
}

func TestIntegrationFileDirectoryConversionsAndManifestOrdering(t *testing.T) {
	setup := func(root string) {
		writeCandidateFile(t, root, "old-dir/child.txt", "old child")
		writeCandidateFile(t, root, "old-file", "old file")
		writeCandidateFile(t, root, "old-dir-sibling", "unchanged sibling")
	}
	mutate := func(root string) {
		for _, name := range []string{"old-dir/child.txt", "old-dir", "old-file"} {
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				t.Fatal(err)
			}
		}
		writeCandidateFile(t, root, "old-dir", "directory became file")
		writeCandidateFile(t, root, "old-file/new.txt", "file became directory")
	}
	e, store, binding, source, _, seal := integrationCandidateFixtureWith(t, setup, mutate)
	ctx := context.Background()
	set, err := store.PrepareIntegrationCandidate(ctx, seal)
	if err != nil {
		t.Fatal(err)
	}
	req := IntegrateChangeSetRequest{RequestID: "convert", Source: binding, ChangeSetID: set.ID, OperationsDigest: set.OperationsDigest, InstructionVersion: set.InstructionVersion, ControlEpoch: set.ControlEpoch, MaxReservedBytes: 4096}
	page, err := store.IntegrateChangeSet(ctx, e.Worker.Session, req)
	if err != nil || page.State != "integrated" {
		t.Fatal(page, err)
	}
	for name, want := range map[string]string{"old-dir": "directory became file", "old-file/new.txt": "file became directory", "old-dir-sibling": "unchanged sibling"} {
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil || string(data) != want {
			t.Fatal(name, string(data), err)
		}
	}
	if _, err = store.IntegrateChangeSet(ctx, nil, req); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("missing supervisor accepted", err)
	}
}
