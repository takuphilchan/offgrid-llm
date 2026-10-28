//go:build !linux && !darwin && !windows

package execution

import "os/exec"

func startOwned(*exec.Cmd) (func() error, func() error, func(), error) {
	return nil, nil, nil, ErrUnsupported
}
