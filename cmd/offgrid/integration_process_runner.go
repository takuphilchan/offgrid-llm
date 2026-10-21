package main

import "os/exec"

// Integration processes are prepared as argv by the Hermes/OpenClaw adapters.
// Keeping dispatch behind this hook lets command tests inspect a proposed
// process without running a host installer or installed agent executable.
var runIntegrationProcess = func(command *exec.Cmd) error { return command.Run() }
