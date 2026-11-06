package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

var ErrAgentSelectionCancelled = errors.New("agent selection cancelled")

// MatchAgentProfiles searches presentation metadata, never instructions or grants.
func MatchAgentProfiles(profiles []agents.AgentProfile, query string) []agents.AgentProfile {
	query = strings.ToLower(strings.TrimSpace(query))
	result := []agents.AgentProfile{}
	for _, p := range profiles {
		if strings.Contains(strings.ToLower(p.Settings.Name+" "+p.ID+" "+p.Settings.Model+" "+p.Settings.Description), query) {
			result = append(result, p)
		}
	}
	return result
}

func PickAgentProfilePlain(input *bufio.Reader, out io.Writer, style PresentationStyle, profiles []agents.AgentProfile) (agents.AgentProfile, error) {
	presentation := Presenter{Stdout: out, Style: style}
	presentation.Heading("Choose agent")
	visible := profiles
	show := func() {
		fmt.Fprintln(out)
		for i, p := range visible {
			fmt.Fprintf(out, "  %d. %s\n", i+1, terminalLine(p.Settings.Name))
			if p.Settings.Description != "" {
				fmt.Fprintf(out, "     %s\n", terminalLine(p.Settings.Description))
			}
			fmt.Fprintf(out, "     Model %s  |  Revision %d\n", terminalLine(p.Settings.Model), p.Revision)
		}
	}
	show()
	for {
		fmt.Fprint(out, "\nChoose a profile number, name or ID (type to filter, /all, /cancel): ")
		line, err := readConversationLine(input)
		if err != nil {
			return agents.AgentProfile{}, err
		}
		query := strings.TrimSpace(line)
		if query == "/cancel" || query == "/exit" {
			return agents.AgentProfile{}, ErrAgentSelectionCancelled
		}
		if query == "/all" || query == "" {
			visible = profiles
			show()
			continue
		}
		if n, err := strconv.Atoi(query); err == nil {
			if n > 0 && n <= len(visible) {
				return visible[n-1], nil
			}
			fmt.Fprintln(out, "Choose a listed profile number or search by name. Nothing has started.")
			continue
		}
		var exact []agents.AgentProfile
		for _, p := range profiles {
			if p.ID == query {
				return p, nil
			}
			if strings.EqualFold(p.Settings.Name, query) {
				exact = append(exact, p)
			}
		}
		if len(exact) == 1 {
			return exact[0], nil
		}
		visible = MatchAgentProfiles(profiles, query)
		if len(visible) == 0 {
			fmt.Fprintln(out, "No matching agents. Try another search or /all.")
		} else {
			show()
		}
	}
}
