//go:build !linux && !darwin && !windows

package execution

import "os/exec"

func startOwned(*exec.Cmd) (func() error, func() error, func() error, error) {
	return nil, nil, nil, ErrUnsupported
}

func nativeCleanupVerified(error) bool { return false }
