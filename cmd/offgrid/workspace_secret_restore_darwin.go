package main

import (
	"os"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sys/unix"
)

func restoreWorkspaceTerminal(input *os.File, state *term.State) error {
	// Darwin TIOCSETA marks pending input for retyping when canonical mode is
	// restored. Protected input must instead be discarded atomically with the
	// mode transition; TIOCSETAF avoids introducing PENDIN after our preflush.
	return unix.IoctlSetTermios(int(input.Fd()), unix.TIOCSETAF, &state.Termios)
}
