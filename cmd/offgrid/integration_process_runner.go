package main

import (
	"context"
	"errors"
	"os/exec"
)

// Integration processes are prepared as argv by the Hermes/OpenClaw adapters.
// Keeping dispatch behind this hook lets command tests inspect a proposed
// process without running a host installer or installed agent executable.
var runIntegrationProcess = runOwnedIntegrationProcess

func runOwnedIntegrationProcess(command *exec.Cmd) error {
	restore, err := prepareIntegrationTerminal(command)
	if err != nil {
		return err
	}
	err = command.Run()
	restoreErr := restore()
	if integrationWasInterrupted(err) {
		err = context.Canceled
	}
	return errors.Join(err, restoreErr)
}
