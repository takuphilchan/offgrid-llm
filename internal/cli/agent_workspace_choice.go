package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ChooseWorkspaceAccessPlain preserves legacy answers for plain terminals and
// tests. Empty input and EOF never grant access; invalid input can be corrected.
func ChooseWorkspaceAccessPlain(input *bufio.Reader, out io.Writer, _ PresentationStyle, remember bool) (string, error) {
	fmt.Fprintln(out, "  1  Allow once")
	if remember {
		fmt.Fprintln(out, "  2  Remember this folder for this account and service")
	}
	fmt.Fprintln(out, "  0  Cancel")
	for {
		fmt.Fprint(out, "Choose [0]: ")
		line, err := input.ReadString('\n')
		if err != nil {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "1", "y", "yes":
			return "y", nil
		case "2", "remember":
			if remember {
				return "remember", nil
			}
		case "", "0", "n", "no", "cancel":
			return "n", nil
		}
		fmt.Fprintln(out, "Choose a listed option, or press Enter to cancel.")
	}
}
