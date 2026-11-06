package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/takuphilchan/offgrid-llm/internal/agentcontext"
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestSessionCommandsNeverSubmitOrAcquireAuthority(t *testing.T) {
	gets := 0
	client, err := serviceclient.New("http://fixture:11611", "", &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" {
			t.Fatal("command mutated state", req.Method, req.URL)
		}
		var body string
		switch req.URL.Path {
		case "/api/v2/system":
			body = `{"product":"offgrid","capabilities":["agent-conversations-v1"]}`
		case "/v1/sessions/conversation":
			body = `{"name":"conversation","created_at":"2026-10-26T00:00:00Z","updated_at":"2026-10-26T00:00:00Z"}`
		default:
			t.Fatal("local command accessed unrelated service state", req.URL)
		}
		gets++
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	profile := agents.AgentProfile{ID: "profile-fixture", Revision: 7, Settings: agents.ProfileSettings{
		Name: "bumblebee", Model: "local-model", MaxIterations: 30,
		Skills: []agentcontext.SkillReference{{ID: "reviewed-skill", Revision: 3}}, Delegates: []string{"researcher"},
		Tools: []agents.ProfileToolReference{{Provider: "owned-provider", Tool: "lookup"}},
	}}
	ctx := context.WithValue(context.Background(), agentSessionKey{}, agentSessionPresentation{Profile: profile})
	var out bytes.Buffer
	err = RunConversationChat(ctx, client, []string{"chat", "conversation", "--profile", profile.ID, "--profile-revision", "7"}, strings.NewReader("/\n/help\n/skills\n/agents\n/tools\n/session\n/clear\n/unknown\n/exit\n"), &out, false, nil)
	if err != nil || gets != 2 {
		t.Fatal(err, gets, out.String())
	}
	if strings.Contains(out.String(), "\x1b") || !strings.Contains(out.String(), "Saved conversation and context are unchanged") {
		t.Fatal("clear changed plain output or history", out.String())
	}
	for _, want := range []string{"bumblebee", "local-model", "reviewed-skill (revision 3)", "researcher", "owned-provider / lookup", "No message was submitted", "--resume conversation", "--profile profile-fixture --profile-revision 7"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing", want, out.String())
		}
	}
	if strings.Count(out.String(), "OffGrid  -  bumblebee") != 1 || strings.Contains(out.String(), "\x1b") {
		t.Fatal("duplicate banner or nonterminal escape", out.String())
	}
}

func TestSessionOutputRetainsTerminalAndEscapesResourceNames(t *testing.T) {
	var out bytes.Buffer
	w := agentSessionWriter{&out}
	if !AgentSessionOutput(w) || AgentSessionOutput(&out) || AgentOutputTarget(w) != &out {
		t.Fatal("writer lost identity")
	}
	p := agentSessionPresentation{Profile: agents.AgentProfile{Settings: agents.ProfileSettings{Name: "agent\x1b[2J", Model: "model\x1b]0;title\a"}}}
	agentSessionHeader(w, p)
	if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "\a") {
		t.Fatal("terminal injection", out.String())
	}
	for _, c := range AgentCommandSuggestions() {
		if !knownAgentSlashCommand(c.Command) {
			t.Fatal("picker and parser diverged", c)
		}
	}
}

func TestClearUsesDisplayAdapterWithoutChangingProfile(t *testing.T) {
	var out bytes.Buffer
	clears := 0
	p := agentSessionPresentation{Profile: agents.AgentProfile{ID: "profile-fixture", Revision: 7, Settings: agents.ProfileSettings{Name: "Bumblebee", Model: "local-model"}}, ClearDisplay: func(io.Writer) bool { clears++; return true }}
	if !renderAgentSessionCommand(&out, "/clear", "saved-conversation", p) || clears != 1 {
		t.Fatal("display adapter not called")
	}
	if p.Profile.Revision != 7 || !strings.Contains(out.String(), "Bumblebee") || !strings.Contains(out.String(), "local-model") {
		t.Fatal("session changed or header missing", out.String())
	}
}

func TestSessionHelpHasShortDefaultAndCompleteOnDemand(t *testing.T) {
	var short, full bytes.Buffer
	p := agentSessionPresentation{}
	if !renderAgentSessionCommand(&short, "/help", "saved", p) || !renderAgentSessionCommand(&full, "/help all", "saved", p) {
		t.Fatal("help command was not recognized")
	}
	if !strings.Contains(short.String(), "/help all") || !strings.Contains(short.String(), "/agent") || strings.Contains(short.String(), "/recover") {
		t.Fatal("default help is not focused", short.String())
	}
	if !strings.Contains(full.String(), "/recover") || !strings.Contains(full.String(), "/default") || !knownAgentSlashCommand("/help all") {
		t.Fatal("complete help is unavailable", full.String())
	}
	if suggestions := AgentCommandSuggestions(); len(suggestions) < 5 || suggestions[0].Command != "/help" || suggestions[1].Command != "/agent" || suggestions[2].Command != "/status" {
		t.Fatal("unexpected command palette order", suggestions)
	}
}

func TestSessionHelpReflowsAndGroupsRecoveryCommands(t *testing.T) {
	for _, width := range []int{32, 48, 80, 120} {
		var out bytes.Buffer
		p := agentSessionPresentation{Style: PresentationStyle{Columns: width}}
		renderAgentSessionCommand(&out, "/help all", "saved", p)
		for _, group := range []string{"Explore", "Session", "Work"} {
			if !strings.Contains(out.String(), group) {
				t.Fatal("missing group", group)
			}
		}
		for _, line := range strings.Split(out.String(), "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("%d columns: %q", width, line)
			}
		}
	}
}
