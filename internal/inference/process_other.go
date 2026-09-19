//go:build !windows

package inference

import "os/exec"

func configureBackgroundProcess(_ *exec.Cmd) {}
