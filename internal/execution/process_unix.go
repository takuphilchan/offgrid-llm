//go:build linux || darwin

package execution

import (
	"os/exec"
	"sync"
	"syscall"
)

func startOwned(command *exec.Cmd) (func() error, func() error, func(), error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return nil, nil, nil, err
	}
	var mu sync.Mutex
	reaped := false
	stop := func() error {
		mu.Lock()
		defer mu.Unlock()
		if reaped {
			return nil
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	wait := func() error {
		// Observe exit without reaping first. Reaping and group cancellation are
		// serialized, so cancellation can never signal a reused numeric PGID.
		observed := observeProcessExit(command.Process.Pid)
		mu.Lock()
		defer mu.Unlock()
		// Shell-daemonized descendants are not persistent process handles. Clean
		// up this group before reaping its leader and releasing its identity.
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		err := command.Wait()
		reaped = true
		if observed != nil {
			return observed
		}
		return err
	}
	return stop, wait, func() {}, nil
}
