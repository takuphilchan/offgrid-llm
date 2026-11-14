package execution

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPreparedWorkerWriteRecoveryUsesExistingStorageAllocation(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("replacement backend unsupported")
	}
	e, store, b, root := workingCopyFixture(t)
	put(t, root, "input.txt", "before")
	ctx := context.Background()
	req := workingCopyRequest(b, "write-budget")
	child, err := e.PrepareWorker(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	intent := request(child.Endpoint.Worker, child.Context.Binding, "file_write", "replace", map[string]any{"path": "input.txt", "data": []byte("proposed"), "revision": digest([]byte("before"))})
	reply, err := child.Endpoint.Call(ctx, intent)
	if err != nil || reply.Receipt == nil || reply.Receipt.State != "complete" {
		t.Fatal(reply, err)
	}
	record, err := store.Measure(ctx, child.Record.ID, b)
	if err != nil || record.ArtifactReservedBytes < 2*int64(len("proposed"))+int64(len("before")) || record.MeasuredBytes <= record.ContentBytes || record.ReservedBytes != req.MaxBytes {
		t.Fatal("recovery bytes escaped existing allocation", record, err)
	}
	intent = request(child.Endpoint.Worker, child.Context.Binding, "file_write", "over-budget", map[string]any{"path": "input.txt", "data": []byte(strings.Repeat("x", 1024)), "revision": digest([]byte("proposed"))})
	reply, err = child.Endpoint.Call(ctx, intent)
	if err != nil || reply.Receipt == nil || reply.Receipt.State != "rejected" || !strings.Contains(string(reply.Receipt.Result), ErrLimit.Error()) {
		t.Fatal("worker opened new allowance", reply, err)
	}
	data, _ := os.ReadFile(filepath.Join(child.Context.Root, "input.txt"))
	if string(data) != "proposed" {
		t.Fatal("rejected write changed candidate")
	}
	data, _ = os.ReadFile(filepath.Join(root, "input.txt"))
	if string(data) != "before" {
		t.Fatal("worker edited shared original")
	}
	if err = child.Close(); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenWorkingCopies(store.directory, store.policy)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()
	after, err := loaded.Measure(ctx, child.Record.ID, b)
	if err != nil || after.ArtifactReservedBytes != record.ArtifactReservedBytes || after.MeasuredBytes != record.MeasuredBytes {
		t.Fatal("restart dropped retained bytes", record, after, err)
	}
}
