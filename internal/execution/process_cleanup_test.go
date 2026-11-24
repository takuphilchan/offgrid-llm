package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessCleanupRejectedLaunchHasNoUnsettledOwner(t *testing.T) {
	s, b, root := fixture(t)
	s.config.Permissions.Processes = true
	m := NewProcessManager(s)
	if _, err := m.Start(context.Background(), b, ProcessSpec{Program: filepath.Join(root, "does-not-exist")}); err == nil {
		t.Fatal("invalid fixture launched")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if !m.cleanupConfirmed() {
		t.Fatal("proven pre-launch rejection invented an unowned process")
	}
}

func TestProcessCleanupUnprovenHistoryCannotBecomeProof(t *testing.T) {
	s, b, _ := fixture(t)
	s.config.Permissions.Processes = true
	m := NewProcessManager(s)
	defer m.Close()
	for i := 0; i < 128; i++ {
		done := make(chan struct{})
		close(done)
		handle := fmt.Sprint(i)
		m.processes[handle] = &ownedProcess{done: done, cancel: func() {}, state: "exited"}
		m.order = append(m.order, handle)
	}
	_, err := m.Start(context.Background(), b, ProcessSpec{Program: os.Args[0], Arguments: []string{"-test.run=^TestProcessFixtureHelper$"}})
	if !errors.Is(err, ErrLimit) {
		t.Fatal("unsettled owner did not retain admission capacity", err)
	}
	if err := m.Close(); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("unknown descendants were acknowledged as closed", err)
	}
	if m.cleanupConfirmed() {
		t.Fatal("bounded history erased cleanup uncertainty")
	}
}
