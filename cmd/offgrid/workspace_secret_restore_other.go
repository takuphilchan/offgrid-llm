//go:build !darwin

package main

import (
	"os"

	"github.com/charmbracelet/x/term"
)

func restoreWorkspaceTerminal(input *os.File, state *term.State) error {
	return term.Restore(input.Fd(), state)
}
