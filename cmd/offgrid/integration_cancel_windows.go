package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func configureIntegrationCancellation(cmd *exec.Cmd) {
	// Cancel only this owned invocation and its descendants; do not stop a
	// separately running gateway or use an executable-name process match.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		stop := exec.Command("taskkill.exe", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		stop.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := stop.Run(); err != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

func prepareIntegrationTerminal(*exec.Cmd) (func() error, error) {
	return func() error { return nil }, nil
}

func integrationWasInterrupted(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && (exit.ExitCode() == 130 || uint32(exit.ExitCode()) == 0xc000013a)
}
