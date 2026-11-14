package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

func testEndpointWorkerPreparation(t *testing.T, binary, digest string) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root, private := t.TempDir(), t.TempDir()
	b := execution.Binding{Actor: "fixture", Workspace: "workspace", Host: "host", Session: "prepare", Grant: "grant", Revision: 1, Job: "parent"}
	config := execution.Context{Version: 1, Binding: b, Backend: "local", Root: root, Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}, WorkerCopies: execution.DefaultWorkingCopyPolicy()}
	grant, state := filepath.Join(private, "grant.json"), filepath.Join(private, "state")
	data, _ := json.Marshal(config)
	if err := os.WriteFile(grant, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := execution.ProtectPrivatePath(grant); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "input.txt"), []byte("input"), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := execution.StartStdio(ctx, binary, digest, grant, state)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	req := execution.WorkingCopyRequest{RequestID: "prepare-once", Parent: b, WorkerRunID: "child", InstructionVersion: 1, Permissions: config.Permissions, MaxBytes: 1024}
	status, err := e.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "accepted" {
		t.Fatal(status, err)
	}
	for status.State == "accepted" {
		status, err = e.InspectWorkerPreparation(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatal(ctx.Err())
		}
		time.Sleep(time.Millisecond)
	}
	if status.State != "ready" || status.Context == nil || status.Record == nil {
		t.Fatal(status)
	}
	child := status.Context.Binding
	again, err := e.BeginWorkerPreparation(ctx, req)
	if err != nil || again.Context == nil || *again.Context != *status.Context {
		t.Fatal("retry changed worker", again, err)
	}
	changed := req
	changed.ControlEpoch++
	if _, err = e.BeginWorkerPreparation(ctx, changed); !errors.Is(err, execution.ErrConflict) {
		t.Fatal("changed retry", err)
	}
	if _, err = e.Check(ctx, child); err != nil {
		t.Fatal(err)
	}
	intent, err := execution.NewIntent(child, "write", "file_write", json.RawMessage(`{"path":"output.txt","data":"bmV3"}`), false)
	if err != nil {
		t.Fatal(err)
	}
	response, err := e.Call(ctx, intent)
	if err != nil || response.Error != "" || response.Receipt == nil || response.Receipt.State != "complete" {
		t.Fatal(response, err)
	}
	if _, err = os.Stat(filepath.Join(root, "output.txt")); !os.IsNotExist(err) {
		t.Fatal("original edited", err)
	}
	if err = e.Cancel(ctx, child); err != nil {
		t.Fatal(err)
	}
	response, err = e.InspectReceipt(ctx, intent)
	if err != nil || response.Receipt == nil || response.Receipt.State != "complete" {
		t.Fatal("lost child receipt", response, err)
	}
	seal := execution.WorkerArtifactRequest{Operation: "seal", Source: b, Seal: &execution.SealChangeSetRequest{RequestID: "capture", CopyID: again.Record.ID, Source: b, InstructionVersion: req.InstructionVersion, ControlEpoch: req.ControlEpoch}}
	captured, err := e.WorkerArtifact(ctx, seal)
	if err != nil {
		t.Fatal(err)
	}
	for captured.State == "accepted" {
		statusRequest := seal
		statusRequest.Operation = "seal_status"
		captured, err = e.WorkerArtifact(ctx, statusRequest)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond)
	}
	if captured.State != "sealed" || captured.ChangeSet == nil {
		t.Fatal(captured)
	}
	setID := captured.ChangeSet.ID
	if err = e.Close(); err != nil {
		t.Fatal(err)
	}
	e, err = execution.StartStdio(ctx, binary, digest, grant, state)
	if err != nil {
		t.Fatal(err)
	}
	status, err = e.BeginWorkerPreparation(ctx, req)
	if err != nil || status.State != "interrupted" || status.Context != nil {
		t.Fatal("restart restored grant", status, err)
	}
	if _, err = e.Check(ctx, child); err == nil {
		t.Fatal("restart adopted child authority")
	}
	if data, err = os.ReadFile(filepath.Join(again.Context.Root, "output.txt")); err != nil || string(data) != "new" {
		t.Fatal("restart lost artifacts", string(data), err)
	}
	recovered, err := e.WorkerArtifact(ctx, seal)
	if err != nil || recovered.State != "sealed" || recovered.ChangeSet.ID != setID {
		t.Fatal("restart repeated or lost capture", recovered, err)
	}
	page, err := e.WorkerArtifact(ctx, execution.WorkerArtifactRequest{Operation: "inspect", Source: b, ChangeSetID: setID, Limit: 64})
	if err != nil || page.Page == nil || len(page.Page.Operations) != 1 {
		t.Fatal(page, err)
	}
	blob, err := e.WorkerArtifact(ctx, execution.WorkerArtifactRequest{Operation: "blob", Source: b, ChangeSetID: setID, Hash: page.Page.Operations[0].Result.SHA256, Limit: execution.MaxPageBytes})
	if err != nil || blob.Blob == nil || string(blob.Blob.Data) != "new" {
		t.Fatal("restart lost sealed output", blob, err)
	}
}
