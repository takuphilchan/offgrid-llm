package execution

import (
	"context"
	"errors"
	"io"
	"time"
)

// ServeProtected binds a native admission reservation to its exact existing
// worker. Only the endpoint's own revocation and descendant cleanup can release
// it; caller cancellation, EOF and process exit alone are not cleanup evidence.
func (w *Worker) ServeProtected(ctx context.Context, input io.ReadCloser, output io.Writer, lease *HostProtectionLease) error {
	return w.serveProtected(ctx, input, output, lease, false)
}

// ServeProtectedRecoverable exports cleanup authority only on private startup
// to the original pinned native supervisor, before accepting any dispatch.
func (w *Worker) ServeProtectedRecoverable(ctx context.Context, input io.ReadCloser, output io.Writer, lease *HostProtectionLease) error {
	return w.serveProtected(ctx, input, output, lease, true)
}

func (w *Worker) serveProtected(ctx context.Context, input io.ReadCloser, output io.Writer, lease *HostProtectionLease, recoverable bool) error {
	handoffAccepted := !recoverable
	if lease == nil || w.Session == nil || w.Journal == nil || w.Processes == nil {
		return ErrAuthority
	}
	lease.mu.Lock()
	if lease.activated || lease.released || lease.record.Binding == nil || lease.record.Journal == nil {
		lease.mu.Unlock()
		return ErrAuthority
	}
	config := w.Session.config
	root, rootErr := locateHostProtection(config.Root)
	journal, journalErr := locateHostProtection(w.Journal.directory)
	if rootErr != nil || journalErr != nil || root != lease.record.Root || journal != *lease.record.Journal || config.Binding != *lease.record.Binding || config.Backend != lease.record.Backend || config.Permissions != lease.record.Permissions {
		lease.mu.Unlock()
		return ErrHostProtectionChanged
	}
	if err := ctx.Err(); err != nil {
		lease.mu.Unlock()
		return err
	}
	if recoverable {
		startup, err := protectedStartup(w, lease)
		if err != nil {
			lease.mu.Unlock()
			return err
		}
		w.protectedStartup = startup
		w.protectedAdmission = func(reader io.Reader) error {
			timer := time.AfterFunc(10*time.Second, func() { _ = input.Close() })
			defer timer.Stop()
			if err := acceptProtectionAcknowledgment(reader, lease.id, w.Token); err != nil {
				return err
			}
			handoffAccepted = true
			return nil
		}
	}
	lease.activated = true
	lease.mu.Unlock()
	return w.serve(ctx, input, output, func(cleanup error) error {
		if !handoffAccepted || cleanup != nil || !w.Session.revoked.Load() || !w.Processes.cleanupConfirmed() {
			return errors.Join(ErrHostProtectionUncertain, cleanup)
		}
		// Cleanup publication must survive the dispatch context being cancelled.
		settled, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return lease.releaseVerified(settled)
	})
}
