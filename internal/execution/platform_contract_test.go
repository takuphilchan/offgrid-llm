package execution

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
)

// assertDarwinCleanupRefusal checks the unsupported native boundary rather than
// treating a generic cleanup success scenario as Darwin qualification. Callers
// still execute every assertion that precedes that boundary.
func assertDarwinCleanupRefusal(t *testing.T, store *WorkingCopies, binding Binding, before WorkingCopyRecord, err error) bool {
	t.Helper()
	if runtime.GOOS != "darwin" || !errors.Is(err, ErrUnsupported) {
		return false
	}
	record, inspectErr := store.Inspect(context.Background(), before.ID, binding)
	if inspectErr != nil || record.State != "uncertain" || record.ReservedBytes < before.ReservedBytes || record.ReservedBytes == 0 {
		t.Fatal("unsupported cleanup lost its reservation", record, inspectErr)
	}
	owner, ownerErr := store.copyOwnership(context.Background(), before.ID)
	if ownerErr != nil || owner.InUse || owner.Detached || owner.Empty || owner.Identity == "" {
		t.Fatal("unsupported cleanup invented settled deletion", owner, ownerErr)
	}
	root, rootErr := store.ownedCopyRoot(before.ID, owner.Identity)
	if rootErr != nil {
		t.Fatal("unsupported cleanup lost the exact owned directory", rootErr)
	}
	defer root.Close()
	if info, statErr := root.Stat("content"); statErr != nil || !info.IsDir() {
		t.Fatal("unsupported cleanup removed copied content", statErr)
	}
	t.Log("Darwin refused unqualified quarantine and retained the owned copy and reservation; successful cleanup remains unqualified")
	return true
}

func assertDarwinWorkerUnsettled(t *testing.T, store *WorkingCopies, binding Binding, child *PreparedWorker, err error) bool {
	t.Helper()
	if runtime.GOOS != "darwin" {
		return false
	}
	if !errors.Is(err, ErrHostUnavailable) || child.Endpoint.Worker.Processes.cleanupConfirmed() {
		t.Fatal("Darwin invented descendant retirement", err)
	}
	owner, ownerErr := store.copyOwnership(context.Background(), child.Record.ID)
	if ownerErr != nil || !owner.InUse || owner.Detached || owner.Quarantine != "" {
		t.Fatal("unproven descendants lost their lease", owner, ownerErr)
	}
	record, inspectErr := store.Inspect(context.Background(), child.Record.ID, binding)
	if inspectErr != nil || record.State != "uncertain" || record.ReservedBytes < child.Record.ReservedBytes {
		t.Fatal("unproven descendants released capacity", record, inspectErr)
	}
	if _, statErr := os.Stat(child.Context.Root); statErr != nil {
		t.Fatal("unproven descendants lost the retained copy", statErr)
	}
	return true
}
