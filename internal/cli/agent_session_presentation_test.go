package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

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
	err = RunConversationChat(ctx, client, []string{"chat", "conversation", "--profile", profile.ID, "--profile-revision", "7"}, strings.NewReader("/\n/help\n/skills\n/agents\n/tools\n/session\n/unknown\n/exit\n"), &out, false, nil)
	if err != nil || gets != 2 {
		t.Fatal(err, gets, out.String())
	}
	for _, want := range []string{"bumblebee", "local-model", "reviewed-skill (revision 3)", "researcher", "owned-provider / lookup", "No message was submitted", "--resume conversation --profile profile-fixture --profile-revision 7"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal("missing", want, out.String())
		}
	}
	if strings.Count(out.String(), "OffGrid  /  Agent chat") != 1 || strings.Contains(out.String(), "\x1b") {
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
