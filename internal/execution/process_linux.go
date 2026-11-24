package execution

import (
	"context"
	"errors"
	"os/exec"
	"sync"

	"github.com/takuphilchan/offgrid-llm/internal/processowner"
)

var linuxNamespaceRuntime = sync.OnceValues(func() (processowner.Runtime, error) {
	return processowner.SystemRuntime(context.Background())
})

func startOwned(command *exec.Cmd) (func() error, func() error, func() error, error) {
	runtime, err := linuxNamespaceRuntime()
	if err != nil {
		return nil, nil, nil, err
	}
	return startOwnedRuntime(command, runtime)
}

func startOwnedRuntime(command *exec.Cmd, runtime processowner.Runtime) (func() error, func() error, func() error, error) {
	owner, err := processowner.StartRuntime(context.Background(), command, runtime)
	if err != nil {
		if errors.Is(err, processowner.ErrStopUncertain) {
			err = errors.Join(errProcessCleanupUncertain, err)
		}
		if errors.Is(err, processowner.ErrStarted) {
			err = &MutationError{Effects: "unknown", Cause: err}
		}
		return nil, nil, nil, err
	}
	release := func() error {
		if err := owner.Close(); err != nil {
			return errors.Join(errProcessCleanupUncertain, err)
		}
		return nil
	}
	return owner.Stop, owner.Wait, release, nil
}

func nativeCleanupVerified(err error) bool { return err == nil }
