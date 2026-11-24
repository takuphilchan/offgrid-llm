package execution

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestHostProtectionBrowserCannotOverlapExistingExecutionJournal(t *testing.T) {
	registry := protectionTestRegistry(t)
	root, parent := protectionTestRoot(t), protectionTestRoot(t)
	journal := protectionTestDirectory(t, parent, "journal")
	config := protectionTestConfig(root, "journal-owner")
	if _, err := registry.ReserveExecution(context.Background(), config, journal); err != nil {
		t.Fatal(err)
	}
	for _, browser := range []string{parent, journal, filepath.Join(journal, "future-browser")} {
		if _, err := registry.RegisterBrowserRoot(context.Background(), browser); !errors.Is(err, ErrHostProtectionConflict) {
			t.Fatalf("execution journal exposed to browser allocation: %v", err)
		}
	}
}

func TestHostProtectionPreservesIndependentTrustedProcessGrant(t *testing.T) {
	registry := protectionTestRegistry(t)
	config := protectionTestConfig(protectionTestRoot(t), "process-only-authority")
	config.Permissions = Permissions{Read: true, Processes: true}
	// Native host UI has its own access modes. A direct locally approved worker
	// has always supported a process grant without granting file_write tools.
	lease, err := registry.ReserveExecution(context.Background(), config, protectionTestRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if lease.record.Permissions != config.Permissions {
		t.Fatal("registry broadened the grant")
	}
	if err = lease.CancelUnactivated(context.Background()); err != nil {
		t.Fatal(err)
	}
}
