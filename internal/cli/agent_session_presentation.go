package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
)

type agentSessionKey struct{}
type agentSessionPresentation struct {
	Profile      agents.AgentProfile
	Style        PresentationStyle
	ReadLine     func(*bufio.Reader, io.Writer, PresentationStyle, []AgentCommandSuggestion) (string, error)
	ClearDisplay func(io.Writer) bool
	Default      func(string, io.Writer)
	Switch       func(string, io.Writer) error
}

type AgentCommandSuggestion struct{ Command, Description string }

func AgentCommandSuggestions() []AgentCommandSuggestion {
	result := make([]AgentCommandSuggestion, 0, len(agentSlashCommands))
	// The short palette leads with actions people use during a conversation.
	// The complete command catalog remains available through /help all.
	for _, name := range []string{"help", "agent", "status", "skills", "clear"} {
		for _, command := range agentSlashCommands {
			if command.Name == name {
				result = append(result, AgentCommandSuggestion{"/" + command.Name, command.Description})
				break
			}
		}
	}
	for _, command := range agentSlashCommands {
		if command.Name == "help" || command.Name == "agent" || command.Name == "status" || command.Name == "skills" || command.Name == "clear" {
			continue
		}
		result = append(result, AgentCommandSuggestion{"/" + command.Name, command.Description})
	}
	return result
}

// Session output is explicitly marked, not detected by scraping rendered text.
// Preserve the underlying terminal for animation and color detection.
type agentSessionWriter struct{ io.Writer }

func (agentSessionWriter) AgentSessionOutput() bool { return true }
func AgentSessionOutput(w io.Writer) bool {
	v, ok := w.(interface{ AgentSessionOutput() bool })
	return ok && v.AgentSessionOutput()
}
func AgentOutputTarget(w io.Writer) io.Writer {
	if v, ok := w.(agentSessionWriter); ok {
		return v.Writer
	}
	return w
}

type agentSlashCommand struct{ Name, Group, Description string }

var agentSlashCommands = []agentSlashCommand{
	{"help", "Explore", "Show session commands"},
	{"skills", "Explore", "Inspect this profile's attached skills"},
	{"agents", "Explore", "Inspect configured collaborators"},
	{"tools", "Explore", "Inspect configured tool attachments"},
	{"agent", "Session", "Choose an agent for a new conversation"},
	{"default", "Session", "Remember this agent; /default show or /default clear"},
	{"session", "Session", "Profile, model and exact resume command"},
	{"clear", "Session", "Clear display; saved conversation and context remain"},
	{"status", "Work", "Inspect the latest saved work"},
	{"pause", "Work", "Pause new scheduling"},
	{"stop", "Work", "Stop work; completed actions remain"},
	{"resume", "Work", "Explicitly resume a safe checkpoint"},
	{"recover", "Work", "Reconcile saved conversation bookkeeping"},
	{"exit", "Session", "Leave chat; saved work is retained"},
}

func agentSessionHeader(w io.Writer, p agentSessionPresentation) {
	s := p.Style
	name := p.Profile.Settings.Name
	if name == "" {
		name = "Agent"
	}
	fmt.Fprintf(w, "\n  %sOffGrid%s  -  %s\n", s.Bold, s.Reset, terminalLine(name))
	if p.Profile.Settings.Model != "" {
		fmt.Fprintf(w, "  %s%s%s\n", s.Muted, terminalLine(p.Profile.Settings.Model), s.Reset)
	}
	fmt.Fprintln(w, "\n  Ask a question or describe a task. Type / for commands.")
}

func agentSessionPrompt(w io.Writer, style PresentationStyle) {
	prompt := style.Chevron
	if prompt == "" {
		prompt = ">"
	}
	fmt.Fprintf(w, "\n%s%s%s ", style.Primary, prompt, style.Reset)
}

func ReadAgentPlainLine(reader *bufio.Reader, w io.Writer, style PresentationStyle) (string, error) {
	agentSessionPrompt(w, style)
	return readConversationLine(reader)
}

// Local commands inspect the pinned profile only. They do not attach resources,
// grant authority, create jobs, or change the profile used by the conversation.
func renderAgentSessionCommand(w io.Writer, text, name string, p agentSessionPresentation) bool {
	command, _, _ := strings.Cut(text, " ")
	if command == "/image" || command == "/images" || command == "/remove-image" {
		fmt.Fprintln(w, "\nAgent image execution is not supported yet. Use ordinary Chat: offgrid run MODEL --image PATH, with an installed vision model and matching projector. No image was attached or sent.")
		return true
	}
	s := p.Style
	heading := func(title string) { fmt.Fprintf(w, "\n%s%s%s\n", s.Bold, title, s.Reset) }
	if p.Profile.Settings.Name == "" && (text == "/skills" || text == "/agents" || text == "/tools") {
		fmt.Fprintln(w, "\nPinned profile details are unavailable in this entry point.\nUse offgrid agent chat --profile ID --profile-revision N to inspect attached resources.")
		return true
	}
	switch text {
	case "/clear":
		if p.ClearDisplay != nil && p.ClearDisplay(w) {
			agentSessionHeader(w, p)
		} else {
			fmt.Fprintln(w, "Display clearing is available in an interactive terminal. Saved conversation and context are unchanged.")
		}
	case "/", "/help", "/help all":
		heading("Commands")
		presentation := Presenter{Stdout: w, Style: s}
		commands := agentSlashCommands
		if text != "/help all" {
			commands = []agentSlashCommand{
				{"agent", "", "Choose another agent"},
				{"status", "", "Inspect current work"},
				{"skills", "", "View attached skills"},
				{"clear", "", "Clear the screen"},
				{"exit", "", "Leave; keep saved work"},
			}
		}
		if text == "/help all" {
			for _, group := range []string{"Explore", "Session", "Work"} {
				var rows [][]string
				for _, command := range commands {
					if command.Group == group {
						rows = append(rows, []string{"/" + command.Name, command.Description})
					}
				}
				presentation.ModernSection(group, rows)
			}
		} else {
			var rows []presentationRow
			for _, command := range commands {
				rows = append(rows, presentationRow{"/" + command.Name, command.Description})
			}
			presentation.rows(rows)
		}
		if text != "/help all" {
			presentation.Note("All commands and recovery controls: /help all")
		}
	case "/session":
		heading("Current session")
		fmt.Fprintf(w, "  Conversation: %s\n  Profile: %s (revision %d)\n", TerminalSafe(name), TerminalSafe(p.Profile.ID), p.Profile.Revision)
		if p.Profile.Settings.Model != "" {
			fmt.Fprintf(w, "  Model: %s\n", TerminalSafe(p.Profile.Settings.Model))
		}
		if p.Profile.Settings.MaxIterations > 0 {
			fmt.Fprintf(w, "  Budget: %d decisions per turn\n", p.Profile.Settings.MaxIterations)
		}
		fmt.Fprintf(w, "\nResume after a saved turn:\n  offgrid agent chat --resume %s\n", TerminalSafe(name))
		fmt.Fprintf(w, "Before the first turn, also pass --profile %s --profile-revision %d.\n", TerminalSafe(p.Profile.ID), p.Profile.Revision)
	case "/skills":
		heading("Attached skills")
		for _, ref := range p.Profile.Settings.Skills {
			fmt.Fprintf(w, "  %s (revision %d)\n", TerminalSafe(ref.ID), ref.Revision)
		}
		if len(p.Profile.Settings.Skills) == 0 {
			fmt.Fprintln(w, "  No skills attached to this profile revision.")
		}
		fmt.Fprintln(w, "\nManage skills: offgrid agent skills help\nAttaching a skill creates a new profile revision; this session stays pinned.")
	case "/agents":
		heading("Configured collaborators")
		for _, id := range p.Profile.Settings.Delegates {
			fmt.Fprintf(w, "  %s\n", TerminalSafe(id))
		}
		if len(p.Profile.Settings.Delegates) == 0 {
			fmt.Fprintln(w, "  No collaborators configured for this profile revision.")
		}
		fmt.Fprintln(w, "\nThe parent delegates within the task's permissions and budget.\nInspect child activity with /status. Manage profiles: offgrid agent profiles help")
	case "/tools":
		heading("Configured tool attachments")
		for _, ref := range p.Profile.Settings.Tools {
			fmt.Fprintf(w, "  %s / %s\n", TerminalSafe(ref.Provider), TerminalSafe(ref.Tool))
		}
		if len(p.Profile.Settings.Tools) == 0 {
			fmt.Fprintln(w, "  No additional tool attachments.")
		}
		fmt.Fprintln(w, "\nBuilt-in tools depend on the current task and granted access.\nConfigured attachments alone do not establish permission or readiness.")
	default:
		return false
	}
	return true
}

func knownAgentSlashCommand(text string) bool {
	if text == "/cancel" {
		return true
	}
	if text == "/help all" {
		return true
	}
	for _, command := range agentSlashCommands {
		if text == "/"+command.Name {
			return true
		}
	}
	return !strings.HasPrefix(text, "/")
}

func sessionPresentation(ctx context.Context, profileID string, revision int) agentSessionPresentation {
	if p, ok := ctx.Value(agentSessionKey{}).(agentSessionPresentation); ok {
		return p
	}
	return agentSessionPresentation{Profile: agents.AgentProfile{ID: profileID, Revision: revision}}
}
