//go:build !windows

package inference

import (
	"os"
	"os/exec"
	"syscall"
)

func configureBackgroundProcess(_ *exec.Cmd) {}

func startOwnedProcess(cmd *exec.Cmd) error { return cmd.Start() }

func ownedProcessAlive(process *os.Process) bool {
	return process != nil && process.Signal(syscall.Signal(0)) == nil
}
