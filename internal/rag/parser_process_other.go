//go:build !windows

package rag

import "os/exec"

func configureParserProcess(_ *exec.Cmd) {}
