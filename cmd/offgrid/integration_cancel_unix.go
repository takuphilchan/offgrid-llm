//go:build !windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

func configureIntegrationCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		// Let runtimes abort their requests and detached workers before forcing
		// termination. SIGKILL first prevented OpenClaw's cleanup from running.
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		if err != nil {
			return err
		}
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		<-timer.C
		// Process.Signal checks the owned process handle, not just a recycled
		// numeric PID. Do not signal a group after that process has exited.
		if cmd.Process.Signal(syscall.Signal(0)) == nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
}

// Setpgid alone leaves interactive children in a background terminal group:
// their first read can stop with SIGTTIN. Transfer foreground ownership only
// when the caller owns the controlling terminal, and restore it on every exit.
func prepareIntegrationTerminal(cmd *exec.Cmd) (func() error, error) {
	noop := func() error { return nil }
	input, ok := cmd.Stdin.(*os.File)
	if !ok || cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		return noop, nil
	}
	fd := int(input.Fd())
	owner, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP)
	if err != nil {
		return noop, nil
	} // pipe/file, not a controlling terminal
	if owner != syscall.Getpgrp() {
		return nil, errors.New("interactive integration must be launched from the foreground terminal")
	}
	state, err := term.GetState(input.Fd())
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr.Foreground = true
	cmd.SysProcAttr.Ctty = fd // parent descriptor, as required by os/exec
	return func() error {
		// A caught SIGTTOU interrupts tcsetpgrp instead of performing it. Ignore
		// it only during the ownership/settings restoration, then restore the
		// CLI's prior disposition. The child has already exited at this point.
		ignored := signal.Ignored(syscall.SIGTTOU)
		signal.Ignore(syscall.SIGTTOU)
		if !ignored {
			defer signal.Reset(syscall.SIGTTOU)
		}
		ownershipErr := unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, owner)
		return errors.Join(ownershipErr, term.Restore(input.Fd(), state))
	}, nil
}

func integrationWasInterrupted(err error) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	return exit.ExitCode() == 130 || ok && status.Signaled() && status.Signal() == syscall.SIGINT
}
