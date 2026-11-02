package rag

import (
	"os/exec"
	"syscall"
)

func configureParserProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
