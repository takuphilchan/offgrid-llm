package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func workingCopyFixture(t *testing.T) (*LocalEndpoint, *WorkingCopies, Binding, string) {
	t.Helper()
	w, b, root := workerFixture(t)
	w.Session.config.Permissions = Permissions{Read: true, Write: true, Processes: true}
	w.AuthorizeProcess = TrustedProcessPolicy
	store, err := OpenWorkingCopies(filepath.Join(t.TempDir(), "private"), WorkingCopyPolicy{Version: 1, MaxTotalBytes: 2048, MaxCopyBytes: 1024, MaxEntries: 100})
	if err != nil {
		t.Fatal(err)
	}
	e := NewLocalEndpoint(w)
	e.WorkingCopies = store
	if err = e.Admit(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close(); store.Close() })
	return e, store, b, root
}

func workingCopyRequest(b Binding, id string) WorkingCopyRequest {
	return WorkingCopyRequest{RequestID: id, Parent: b, WorkerRunID: "run-" + id, InstructionVersion: 2, ControlEpoch: 3, Permissions: Permissions{Read: true, Write: true, Processes: true}, MaxBytes: 1024}
}

func TestWorkingCopyIndependentInputsSessionsAndStorage(t *testing.T) {
	e, store, b, root := workingCopyFixture(t)
	for name, body := range map[string]string{"dirty.txt": "current dirty bytes", "untracked.txt": "not tracked", ".git/index": "untouched index", ".gitignore": "untracked.txt\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	req := workingCopyRequest(b, "one")
	req.RequiredPaths = []string{"dirty.txt", "untracked.txt"}
	one, err := e.PrepareWorker(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.PrepareWorker(ctx, req)
	if err != nil || again != one {
		t.Fatal("retry did not preserve the live worker", err)
	}
	changed := req
	changed.ControlEpoch++
	if _, err = e.PrepareWorker(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("changed retry accepted", err)
	}
	two, err := e.PrepareWorker(ctx, workingCopyRequest(b, "two"))
	if err != nil {
		t.Fatal(err)
	}
	if one.Context.Root == two.Context.Root || one.Context.Binding.Session == two.Context.Binding.Session || one.Context.Binding.Grant == two.Context.Binding.Grant {
		t.Fatal("workers share environment or grant")
	}
	for _, worker := range []*PreparedWorker{one, two} {
		intent := request(worker.Endpoint.Worker, worker.Context.Binding, "file_write", "same-call", map[string]any{"path": "independent.txt", "data": []byte(worker.Context.Binding.Job)})
		response, callErr := worker.Endpoint.Call(ctx, intent)
		if callErr != nil || response.Error != "" || response.Receipt == nil || response.Receipt.State != "complete" {
			t.Fatal("independent journal dispatch failed", response, callErr)
		}
		retained, readErr := worker.Endpoint.Worker.Journal.Inspect(ctx, intent)
		if readErr != nil || retained.State != "complete" {
			t.Fatal("missing independent receipt", retained, readErr)
		}
		for name, expected := range map[string]string{"dirty.txt": "current dirty bytes", "untracked.txt": "not tracked"} {
			data, err := os.ReadFile(filepath.Join(worker.Context.Root, name))
			if err != nil || string(data) != expected {
				t.Fatal(name, string(data), err)
			}
		}
		if _, err = os.Lstat(filepath.Join(worker.Context.Root, ".git")); !os.IsNotExist(err) {
			t.Fatal("copied Git administration", err)
		}
		if err = PrivateStateOutsideRoot(worker.Context.Root, filepath.Join(store.directory, worker.Record.ID, "journal"), store.directory); err != nil {
			t.Fatal("runtime state is worker-visible", err)
		}
	}
	manifest, err := store.Manifest(ctx, one.Record.ID, b)
	if err != nil || len(manifest.Exclusions) != 1 || manifest.Exclusions[0].Path != ".git" || manifest.InstructionVersion != 2 || manifest.ControlEpoch != 3 {
		t.Fatal(manifest, err)
	}
	if _, err = e.PrepareWorker(ctx, workingCopyRequest(b, "three")); !errors.Is(err, ErrLimit) {
		t.Fatal("overcommitted storage", err)
	}
	if err = os.WriteFile(filepath.Join(one.Context.Root, "dirty.txt"), []byte("worker edit"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, expected := range map[string]string{filepath.Join(root, "dirty.txt"): "current dirty bytes", filepath.Join(two.Context.Root, "dirty.txt"): "current dirty bytes", filepath.Join(root, ".git/index"): "untouched index"} {
		data, err := os.ReadFile(name)
		if err != nil || string(data) != expected {
			t.Fatal("worker edit changed another environment", name, string(data), err)
		}
	}
	foreign := one.Context.Binding
	foreign.Job = "unassigned"
	if one.Endpoint.Admit(ctx, foreign) == nil {
		t.Fatal("child grant admitted an unrelated run")
	}
	if _, err = one.Endpoint.PrepareWorker(ctx, workingCopyRequest(one.Context.Binding, "nested")); err == nil {
		t.Fatal("recursive worker creation enabled")
	}
	if err = one.Release(ctx); err != nil {
		t.Fatal(err)
	}
	released, err := store.Inspect(ctx, one.Record.ID, b)
	if err != nil || released.State != "released" || released.ReservedBytes != 0 {
		t.Fatal(released, err)
	}
	if _, err = e.PrepareWorker(ctx, workingCopyRequest(b, "three")); err != nil {
		t.Fatal("released bytes unavailable", err)
	}
}

func TestWorkingCopyRejectsAuthorityMissingInputsAndLinks(t *testing.T) {
	e, _, b, root := workingCopyFixture(t)
	for _, kind := range []string{"foreign", "missing", "git", "budget"} {
		req := workingCopyRequest(b, kind)
		switch kind {
		case "foreign":
			req.Parent.Actor = "another"
		case "missing":
			req.RequiredPaths = []string{"absent.txt"}
		case "git":
			req.RequiredPaths = []string{".git/index"}
		case "budget":
			req.MaxBytes = 1025
		}
		if _, err := e.PrepareWorker(context.Background(), req); err == nil {
			t.Fatal("invalid preparation accepted", kind)
		}
	}
	target := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(target, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "link")); err != nil {
		t.Skip("this host cannot create the disposable symlink fixture:", err)
	}
	if _, err := e.PrepareWorker(context.Background(), workingCopyRequest(b, "link")); err == nil || !strings.Contains(err.Error(), "links or special files") {
		t.Fatal("unsupported link was copied", err)
	}
}

func TestWorkingCopyCopyRevalidatesContents(t *testing.T) {
	e, store, b, root := workingCopyFixture(t)
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	req := workingCopyRequest(b, "changed")
	source, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	manifest, err := inventoryWorkingCopy(context.Background(), source, req, store.policy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "file"), []byte("after!"), 0600); err != nil {
		t.Fatal(err)
	}
	target, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err = copyWorkingTree(context.Background(), source, target, manifest); !errors.Is(err, ErrRevision) {
		t.Fatal("changed source accepted", err)
	}
	e.Worker.Session.Revoke()
	if _, err = e.PrepareWorker(context.Background(), req); err == nil {
		t.Fatal("revoked source copied")
	}
}

func TestWorkingCopyIndependentProcessOwnershipAndParentCancellation(t *testing.T) {
	e, _, b, _ := workingCopyFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	one, err := e.PrepareWorker(ctx, workingCopyRequest(b, "one"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := e.PrepareWorker(ctx, workingCopyRequest(b, "two"))
	if err != nil {
		t.Fatal(err)
	}
	spec := ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessFixtureHelper$", "--", "offgrid-stream-fixture"}}
	p1, err := one.Endpoint.Worker.Processes.Start(ctx, one.Context.Binding, spec)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := two.Endpoint.Worker.Processes.Start(ctx, two.Context.Binding, spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = one.Endpoint.Worker.Processes.Inspect(ctx, one.Context.Binding, p2.Handle); err == nil {
		t.Fatal("worker inspected a sibling process")
	}
	if _, err = one.Endpoint.Worker.Processes.Stop(ctx, one.Context.Binding, p1.Handle); err != nil {
		t.Fatal(err)
	}
	other, err := two.Endpoint.Worker.Processes.Inspect(ctx, two.Context.Binding, p2.Handle)
	if err != nil || other.State != "running" {
		t.Fatal("stopping one worker stopped its sibling", other, err)
	}
	if err = e.Cancel(ctx, b); err != nil {
		t.Fatal(err)
	}
	for _, child := range []*PreparedWorker{one, two} {
		select {
		case <-child.done:
		default:
			t.Fatal("parent Stop acknowledged before child shutdown")
		}
	}
	if _, err = two.Endpoint.Check(ctx, two.Context.Binding); err == nil {
		t.Fatal("parent cancellation left child authority alive")
	}
	if reflect.DeepEqual(one.Context.Binding, two.Context.Binding) {
		t.Fatal("copied grant")
	}
}
