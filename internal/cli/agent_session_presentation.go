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
	Profile  agents.AgentProfile
	Style    PresentationStyle
	ReadLine func(*bufio.Reader, io.Writer, PresentationStyle, []AgentCommandSuggestion) (string, error)
}

type AgentCommandSuggestion struct{ Command, Description string }

func AgentCommandSuggestions() []AgentCommandSuggestion {
	result := make([]AgentCommandSuggestion, 0, len(agentSlashCommands))
	for _, command := range agentSlashCommands {
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
	{"session", "Session", "Profile, model and exact resume command"},
	{"status", "Work", "Inspect the latest saved work"},
	{"pause", "Work", "Pause new scheduling"},
	{"stop", "Work", "Stop work; completed actions remain"},
	{"resume", "Work", "Explicitly resume a safe checkpoint"},
	{"recover", "Work", "Reconcile saved conversation bookkeeping"},
	{"exit", "Session", "Leave chat; saved work is retained"},
}

func agentSessionHeader(w io.Writer, p agentSessionPresentation) {
	s := p.Style
	fmt.Fprintf(w, "\n%sOffGrid%s  /  Agent chat\n", s.Bold, s.Reset)
	if p.Profile.Settings.Name != "" {
		fmt.Fprintf(w, "  %s%s%s\n", s.Bold, TerminalSafe(p.Profile.Settings.Name), s.Reset)
	}
	if p.Profile.Settings.Model != "" {
		fmt.Fprintf(w, "  %s%s%s\n", s.Muted, TerminalSafe(p.Profile.Settings.Model), s.Reset)
	}
	fmt.Fprintf(w, "\n  Ask a question or describe a task.\n  %s/ commands  /session details  /exit leave%s\n\n", s.Muted, s.Reset)
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
	s := p.Style
	heading := func(title string) { fmt.Fprintf(w, "\n%s%s%s\n", s.Bold, title, s.Reset) }
	if p.Profile.Settings.Name == "" && (text == "/skills" || text == "/agents" || text == "/tools") {
		fmt.Fprintln(w, "\nPinned profile details are unavailable in this entry point.\nUse offgrid agent chat --profile ID --profile-revision N to inspect attached resources.")
		return true
	}
	switch text {
	case "/", "/help":
		heading("Session commands")
		for _, group := range []string{"Explore", "Work", "Session"} {
			fmt.Fprintf(w, "\n  %s%s%s\n", s.Muted, group, s.Reset)
			for _, command := range agentSlashCommands {
				if command.Group == group {
					fmt.Fprintf(w, "  /%s\n    %s\n", command.Name, command.Description)
				}
			}
		}
		fmt.Fprintln(w, "\nUse commands at the input prompt. Ctrl+C interrupts foreground work.")
	case "/session":
		heading("Current session")
		fmt.Fprintf(w, "  Conversation: %s\n  Profile: %s (revision %d)\n", TerminalSafe(name), TerminalSafe(p.Profile.ID), p.Profile.Revision)
		if p.Profile.Settings.Model != "" {
			fmt.Fprintf(w, "  Model: %s\n", TerminalSafe(p.Profile.Settings.Model))
		}
		if p.Profile.Settings.MaxIterations > 0 {
			fmt.Fprintf(w, "  Budget: %d decisions per turn\n", p.Profile.Settings.MaxIterations)
		}
		fmt.Fprintf(w, "\nResume this conversation:\n  offgrid agent chat --resume %s --profile %s --profile-revision %d\n", TerminalSafe(name), TerminalSafe(p.Profile.ID), p.Profile.Revision)
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
