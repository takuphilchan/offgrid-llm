package inference

import (
	"context"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/processowner"
	"net"
	"os/exec"
)

type providerNamespaceLauncher = processowner.Launcher

// Preserve inference's uncertainty contract while sharing native lifetime
// ownership with execution. No provider/HTTP authority lives in processowner.
type providerNamespace struct {
	owner *processowner.Namespace
	done  <-chan struct{}
}

func providerNamespaceError(err error) error {
	if errors.Is(err, processowner.ErrStopUncertain) {
		return errors.Join(errProviderStopUncertain, err)
	}
	return err
}
func startProviderNamespace(ctx context.Context, cmd *exec.Cmd, launcher providerNamespaceLauncher) (*providerNamespace, error) {
	owner, err := processowner.Start(ctx, cmd, launcher)
	if err != nil {
		return nil, providerNamespaceError(err)
	}
	return &providerNamespace{owner: owner, done: owner.Done()}, nil
}
func (job *providerNamespace) close() error { return providerNamespaceError(job.owner.Close()) }
func (job *providerNamespace) verifyConnection(ctx context.Context, conn net.Conn) error {
	return job.owner.VerifyConnection(ctx, conn)
}
