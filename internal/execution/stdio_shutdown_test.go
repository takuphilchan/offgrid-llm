package execution

import (
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

func TestForcedWorkerShutdownCannotClaimConfirmedCleanup(t *testing.T) {
	input, send := io.Pipe()
	output, receive := io.Pipe()
	defer input.Close()
	defer receive.Close()
	var killed atomic.Bool
	e := &StdioEndpoint{input: send, output: output, done: make(chan struct{}), cleanupResult: make(chan error, 1)}
	e.stop = func() error {
		killed.Store(true)
		e.cleanupResult <- nil
		close(e.done)
		return nil
	}
	if err := e.Close(); !errors.Is(err, ErrHostUnavailable) || !killed.Load() {
		t.Fatal("forced worker death was reported as command cleanup", err)
	}
	if err := e.Close(); !errors.Is(err, ErrHostUnavailable) {
		t.Fatal("repeated Close lost uncertainty", err)
	}
}
