package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/takuphilchan/offgrid-llm/internal/cli"
)

type agentMenuRow struct {
	text     string
	selected bool
}

// The command palette shares the editor's measured viewport. In narrow panes,
// the selected command gets a separate description instead of an unreadable
// fragment squeezed beside a fixed-width command column.
func agentCommandMenu(matches []cli.AgentCommandSuggestion, selected, width, height int) []agentMenuRow {
	if len(matches) == 0 {
		return nil
	}
	width = max(4, width-1)
	selected = max(0, min(selected, len(matches)-1))
	narrow := width < 60
	limit := max(1, min(5, height-4))
	if narrow {
		limit = max(1, min(3, height-6))
	}
	start := max(0, min(selected-limit/2, len(matches)-limit))
	var rows []agentMenuRow
	clean := func(s string) string { return strings.Join(strings.Fields(cli.TerminalSafe(s)), " ") }
	for i := start; i < len(matches) && i < start+limit; i++ {
		marker := "    "
		if i == selected {
			marker = "  > "
		}
		label := marker + clean(matches[i].Command)
		if !narrow {
			label = fmt.Sprintf("%s%-13s %s", marker, clean(matches[i].Command), clean(matches[i].Description))
		}
		rows = append(rows, agentMenuRow{ansi.Truncate(label, width, "..."), i == selected})
	}
	if narrow {
		lines := strings.Split(ansi.Wordwrap(clean(matches[selected].Description), width-2, ""), "\n")
		for i, line := range lines {
			if i == 2 {
				break
			}
			rows = append(rows, agentMenuRow{"  " + ansi.Truncate(line, width-2, "..."), false})
		}
	}
	rows = append(rows, agentMenuRow{"", false})
	hint := "  Up/Down select  Tab complete  Enter run  Esc close"
	if narrow {
		hint = "  Enter run  Tab fill  Esc close"
	}
	if len(matches) > limit && !narrow {
		hint = "  Up/Down for more  Tab complete  Enter run  Esc close"
	}
	rows = append(rows, agentMenuRow{ansi.Truncate(hint, width, "..."), false})
	return rows
}
