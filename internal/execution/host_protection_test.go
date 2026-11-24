package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func protectionTestBase(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if CheckPrivateAncestry(directory) == nil {
		return directory
	}
	// Some Windows TEMP ACLs grant other principals directory replacement. Use
	// newly created isolated state under a checked account ancestor; never repair
	// the machine's TEMP or any existing account directory for a test.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	if err = CheckPrivateAncestry(home); err != nil {
		t.Fatalf("no safe isolated test parent: %v", err)
	}
	directory, err = os.MkdirTemp(home, ".offgrid-host-protection-test-")
	if err != nil {
		t.Fatal(err)
	}
	if err = ProtectPrivatePath(directory); err != nil {
		t.Fatal(err)
	}
	owned, err := os.Lstat(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		current, err := os.Lstat(directory)
		if os.IsNotExist(err) {
			return
		}
		relative, relErr := filepath.Rel(home, directory)
		if err != nil || relErr != nil || filepath.IsAbs(relative) || strings.Contains(relative, string(filepath.Separator)) || !strings.HasPrefix(relative, ".offgrid-host-protection-test-") || !os.SameFile(current, owned) {
			t.Errorf("test cleanup ownership changed: %v", err)
			return
		}
		if err = os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	return directory
}

func protectionTestDirectory(t *testing.T, parent, name string) string {
	t.Helper()
	directory := filepath.Join(parent, name)
	if err := EnsurePrivateDirectory(directory); err != nil {
		t.Fatal(err)
	}
	return directory
}
func protectionTestRegistry(t *testing.T) *HostProtectionRegistry {
	t.Helper()
	temporary := protectionTestBase(t)
	parent := protectionTestDirectory(t, temporary, "account")
	return &HostProtectionRegistry{directory: filepath.Join(parent, "host-protection")}
}
func protectionTestRoot(t *testing.T) string {
	t.Helper()
	temporary := protectionTestBase(t)
	return protectionTestDirectory(t, temporary, "private")
}
func protectionTestConfig(root, session string) Context {
	return Context{Version: ProtocolVersion, Binding: Binding{Actor: "actor", Workspace: "workspace", Host: "host", Session: session, Grant: "grant", Revision: 1, Job: BootstrapJob}, Backend: "local", Root: root, Directory: ".", Permissions: Permissions{Read: true, Write: true}}
}

func TestHostProtectionInvalidInputsDoNotCreateState(t *testing.T) {
	registry := protectionTestRegistry(t)
	if _, err := registry.RegisterBrowserRoot(context.Background(), "relative"); !errors.Is(err, ErrPath) {
		t.Fatal(err)
	}
	if _, err := registry.RegisterBrowserRoot(context.Background(), filepath.Dir(registry.directory)); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal(err)
	}
	if _, err := registry.ReserveExecution(context.Background(), Context{}, "relative"); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := registry.RegisterBrowserRoot(cancelled, protectionTestRoot(t)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(registry.directory))
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid input wrote state: %v %v", entries, err)
	}
}

func TestHostProtectionBrowserRegistrationAndReplacement(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root := protectionTestRoot(t)
	first, err := registry.RegisterBrowserRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.RegisterBrowserRoot(ctx, filepath.Join(root, "."))
	if err != nil || second.ID() != first.ID() {
		t.Fatalf("registration not idempotent: %v", err)
	}
	if err = first.Finalize(ctx); err != nil {
		t.Fatal(err)
	}
	old := root + "-retained"
	if err = os.Rename(root, old); err != nil {
		t.Fatal(err)
	}
	if err = EnsurePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, root); !errors.Is(err, ErrHostProtectionChanged) {
		t.Fatalf("adopted replacement: %v", err)
	}
	if err = first.Finalize(ctx); !errors.Is(err, ErrHostProtectionChanged) {
		t.Fatalf("accepted changed root: %v", err)
	}
}

func TestHostProtectionPendingRootNeedsOriginalCapability(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	parent := protectionTestRoot(t)
	root := filepath.Join(parent, "browser")
	claim, err := registry.RegisterBrowserRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	reopened := &HostProtectionRegistry{directory: registry.directory}
	if _, err = reopened.RegisterBrowserRoot(ctx, root); !errors.Is(err, ErrHostProtectionUncertain) {
		t.Fatalf("reopened pending reservation: %v", err)
	}
	if err = EnsurePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.RegisterBrowserRoot(ctx, root); !errors.Is(err, ErrHostProtectionUncertain) {
		t.Fatalf("adopted created object after losing capability: %v", err)
	}
	forged := *claim
	forged.capability = ""
	if err = forged.Finalize(ctx); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
	if err = claim.Finalize(ctx); err != nil {
		t.Fatal(err)
	}
	if err = claim.Finalize(ctx); err != nil {
		t.Fatalf("finalization retry failed: %v", err)
	}
	registered, err := reopened.RegisterBrowserRoot(ctx, root)
	if err != nil || registered.ID() != claim.ID() {
		t.Fatalf("finalized registration lost: %v", err)
	}
}

func TestHostProtectionPendingAncestorReplacementCannotFinalize(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	parent := protectionTestRoot(t)
	root := filepath.Join(parent, "browser")
	claim, err := registry.RegisterBrowserRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(parent, parent+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err = EnsurePrivateDirectory(parent); err != nil {
		t.Fatal(err)
	}
	if err = EnsurePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err = claim.Finalize(ctx); !errors.Is(err, ErrHostProtectionChanged) {
		t.Fatalf("adopted replacement ancestor: %v", err)
	}
}

func TestHostProtectionExecutionConflictAndCrashRetention(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	workspace := protectionTestRoot(t)
	root := filepath.Join(workspace, "browser")
	config := protectionTestConfig(workspace, "first")
	lease, err := registry.ReserveExecution(ctx, config, protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	reopened := &HostProtectionRegistry{directory: registry.directory}
	if _, err = reopened.RegisterBrowserRoot(ctx, root); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatalf("lost unresolved execution after reopening: %v", err)
	}
	if _, err = reopened.ReserveExecution(ctx, config, protectionTestRoot(t)); !errors.Is(err, ErrHostProtectionUncertain) {
		t.Fatalf("reopened execution authority: %v", err)
	}
	forged := &HostProtectionLease{registry: lease.registry, id: lease.id, capability: protectionHash("different capability")}
	if err = forged.releaseVerified(ctx); !errors.Is(err, ErrAuthority) {
		t.Fatal(err)
	}
	if err = lease.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if err = lease.releaseVerified(ctx); err != nil {
		t.Fatal("release retry", err)
	}
	claim, err := reopened.RegisterBrowserRoot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = EnsurePrivateDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err = claim.Finalize(ctx); err != nil {
		t.Fatal(err)
	}
	for _, selected := range []string{workspace, root, protectionTestDirectory(t, root, "child")} {
		if _, err = reopened.ReserveExecution(ctx, protectionTestConfig(selected, "next"), protectionTestRoot(t)); !errors.Is(err, ErrHostProtectionConflict) {
			t.Fatalf("accepted overlapping execution: %s %v", selected, err)
		}
	}
	if _, err = reopened.ReserveExecution(ctx, protectionTestConfig(protectionTestRoot(t), "disjoint"), protectionTestRoot(t)); err != nil {
		t.Fatal("disjoint rooted admission", err)
	}
}

func TestHostProtectionCancelUnactivatedCannotRefundActivatedLease(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root := protectionTestRoot(t)
	lease, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "unactivated"), protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if err = lease.CancelUnactivated(ctx); err != nil {
		t.Fatal(err)
	}
	if err = lease.CancelUnactivated(ctx); err != nil {
		t.Fatal("cancellation retry", err)
	}
	if !lease.released || lease.activated {
		t.Fatal("wrong cancellation state")
	}
	active, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "activated"), protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	active.mu.Lock()
	active.activated = true
	active.mu.Unlock()
	if err = active.CancelUnactivated(ctx); !errors.Is(err, ErrAuthority) {
		t.Fatal("cancel bypassed activation", err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, filepath.Join(root, "browser")); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal("activated lease lost", err)
	}
	if err = active.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionProcessAuthorityRecordedWithoutContainmentClaim(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	config := protectionTestConfig(protectionTestRoot(t), "process-authority")
	config.Permissions.Processes = true
	lease, err := registry.ReserveExecution(ctx, config, protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	err = registry.transaction(ctx, func(store *hostProtectionTransaction) error {
		row := store.records[lease.ID()]
		if row.Backend != "local" || !row.Permissions.Processes || row.State != "reserved" || row.CapabilityHash != protectionHash(lease.capability) || row.CapabilityHash == lease.capability {
			t.Fatalf("incorrect authority evidence: %+v", row)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionAccountPathIgnoresDesktopProfile(t *testing.T) {
	first, err := AccountHostProtectionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_DESKTOP_HOME", t.TempDir())
	second, err := AccountHostProtectionRegistry()
	if err != nil || first.directory != second.directory {
		t.Fatalf("profile changed account registry: %v", err)
	}
}

func TestHostProtectionCreatesOnlyMissingAccountParents(t *testing.T) {
	base := protectionTestBase(t)
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", filepath.Join(base, "missing-config"))
	case "darwin":
		t.Setenv("HOME", base)
	default:
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "missing-config"))
	}
	registry, err := AccountHostProtectionRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(context.Background(), protectionTestRoot(t)); err != nil {
		t.Fatal(err)
	}
	for path := registry.directory; path != base; path = filepath.Dir(path) {
		if path == filepath.Dir(path) {
			t.Fatal("test account state escaped isolated base")
		}
		if err = CheckPrivatePath(path); err != nil {
			t.Fatalf("new account ancestor not private: %s %v", path, err)
		}
	}
}

func TestHostProtectionMovedActiveRootCannotEscapeConflict(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root := protectionTestRoot(t)
	lease, err := registry.ReserveExecution(ctx, protectionTestConfig(root, "moved"), protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	moved := root + "-moved"
	if err = os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, moved); !errors.Is(err, ErrHostProtectionUncertain) {
		t.Fatalf("moving active root bypassed exclusion: %v", err)
	}
	if err = lease.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, moved); err != nil {
		t.Fatal(err)
	}
}

func TestHostProtectionConfirmedCloseAllowsFreshMatchingLease(t *testing.T) {
	ctx := context.Background()
	registry := protectionTestRegistry(t)
	root := protectionTestRoot(t)
	journal := protectionTestRoot(t)
	config := protectionTestConfig(root, "explicit-restart")
	first, err := registry.ReserveExecution(ctx, config, journal)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	second, err := registry.ReserveExecution(ctx, config, journal)
	if err != nil {
		t.Fatalf("verified close prevented explicit matching restart: %v", err)
	}
	if first.ID() == second.ID() || first.capability == second.capability {
		t.Fatal("restart reused authority")
	}
	if err = first.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.RegisterBrowserRoot(ctx, filepath.Join(root, "browser")); !errors.Is(err, ErrHostProtectionConflict) {
		t.Fatal("old capability released new owner", err)
	}
	if err = second.releaseVerified(ctx); err != nil {
		t.Fatal(err)
	}
	changed := config
	changed.Permissions.Write = false
	if _, err = registry.ReserveExecution(ctx, changed, journal); !errors.Is(err, ErrHostProtectionChanged) {
		t.Fatal("same session rebound permissions", err)
	}
	changed = config
	changed.Root = protectionTestRoot(t)
	if _, err = registry.ReserveExecution(ctx, changed, journal); !errors.Is(err, ErrHostProtectionChanged) {
		t.Fatal("same session rebound root", err)
	}
	if _, err = registry.ReserveExecution(ctx, config, protectionTestRoot(t)); !errors.Is(err, ErrHostProtectionChanged) {
		t.Fatal("same session rebound journal", err)
	}
}
