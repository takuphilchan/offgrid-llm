package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func choiceProfiles() []agents.AgentProfile {
	return []agents.AgentProfile{
		{ID: "profile-a", Revision: 2, Settings: agents.ProfileSettings{Name: "Researcher", Description: "Find evidence", Model: "model-a"}},
		{ID: "profile-b", Revision: 8, Settings: agents.ProfileSettings{Name: "Writer", Description: "Draft and revise", Model: "model-b"}},
	}
}

type switchLease struct {
	fixtureExecutionLease
	closeError error
}

func (l *switchLease) Close() error { l.closed++; return l.closeError }

func TestAgentSwitchClosesOwnedLeaseBeforeNewConversation(t *testing.T) {
	for _, closeFails := range []bool{false, true} {
		t.Run(fmt.Sprint(closeFails), func(t *testing.T) {
			var out, errOut bytes.Buffer
			env := testEnvironment(&out, &errOut, &fakeRunner{})
			env.Stdin = strings.NewReader("2\n/exit\n")
			lease := &switchLease{fixtureExecutionLease: fixtureExecutionLease{resource: execution.Resource{Reference: execution.Reference{Host: "host", Session: "old-session", Grant: "grant", Revision: 1}}, done: make(chan struct{})}}
			if closeFails {
				lease.closeError = errors.New("fixture close failed")
			}
			creates, chats, opens := 0, 0, 0
			names := map[string]bool{}
			client := continuityClient(t, func(r *http.Request) any {
				switch r.URL.Path {
				case "/api/v2/system":
					return map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
				case "/api/v2/agent-profiles/profile-a":
					return choiceProfiles()[0]
				case "/api/v2/agent-profiles":
					return map[string]any{"profiles": choiceProfiles()}
				case "/v1/sessions":
					if r.Method != "POST" {
						t.Fatal(r.Method)
					}
					var body map[string]string
					json.NewDecoder(r.Body).Decode(&body)
					creates++
					if names[body["name"]] {
						t.Fatal("reused conversation")
					}
					names[body["name"]] = true
					if creates > 1 && lease.closed != 1 {
						t.Fatal("new conversation before lease closed")
					}
					return map[string]string{"name": body["name"]}
				case "/v1/sessions/previous/turn":
					return map[string]any{"turn": nil}
				default:
					t.Fatal(r.Method, r.URL)
					return nil
				}
			})
			env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
			env.Execution.Open = func(_ context.Context, _ *serviceclient.Client, _ string, _ execution.Permissions, _ func(execution.LocalApproval) bool) (ExecutionLease, error) {
				opens++
				return lease, nil
			}
			env.Service.Chat = func(ctx context.Context, _ *serviceclient.Client, args []string, input io.Reader, _ io.Writer, _ bool) error {
				chats++
				if chats == 1 {
					return sessionPresentation(ctx, "", 0).Switch("previous", &out)
				}
				if strings.Contains(strings.Join(args, " "), "--workspace") || args[3] != "profile-b" {
					t.Fatal("inherited authority or wrong profile", args)
				}
				remaining, _ := io.ReadAll(input)
				if string(remaining) != "/exit\n" {
					t.Fatal(string(remaining))
				}
				return nil
			}
			err := runInteractiveAgent(context.Background(), []string{"--profile", "profile-a", "--workspace-folder", "fixture"}, env)
			want := 2
			if closeFails {
				want = 1
				if err == nil {
					t.Fatal("ignored lease close failure")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if creates != want || chats != want || opens != 1 || lease.closed != 1 {
				t.Fatal(creates, chats, opens, lease.closed, err)
			}
		})
	}
}

func TestAgentSwitchProtectsTerminalUnknownEffects(t *testing.T) {
	if !agentSwitchBlocked(LinkedTurnResponse{Job: &AgentRunSnapshot{Status: "failed", UncertainCallID: "pending"}}) {
		t.Fatal("uncertain effects bypassed")
	}
	if !agentSwitchBlocked(LinkedTurnResponse{RecoveryRequired: true}) {
		t.Fatal("recovery bypassed")
	}
}

func TestResumeLegacyNeedsExplicitProfile(t *testing.T) {
	client := continuityClient(t, func(r *http.Request) any {
		return json.RawMessage(`{"name":"empty","created_at":"2026-11-01T00:00:00Z","updated_at":"2026-11-01T00:00:00Z"}`)
	})
	if _, err := resolveAgentResume(context.Background(), client, interactiveAgentOptions{resume: "empty"}); err == nil {
		t.Fatal("invented identity")
	}
	if _, err := resolveAgentResume(context.Background(), client, interactiveAgentOptions{resume: "empty", profile: "profile-a"}); err != nil {
		t.Fatal(err)
	}
}

func TestAgentSwitchRechecksWorkAfterPicker(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("2\n/exit\n")
	creates := 0
	client := continuityClient(t, func(r *http.Request) any {
		switch r.URL.Path {
		case "/api/v2/system":
			return map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
		case "/api/v2/agent-profiles/profile-a":
			return choiceProfiles()[0]
		case "/api/v2/agent-profiles":
			return map[string]any{"profiles": choiceProfiles()}
		case "/v1/sessions":
			creates++
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			return map[string]string{"name": body["name"]}
		case "/v1/sessions/old/turn":
			return map[string]any{"job": map[string]string{"run_id": "new-work", "status": "running"}}
		default:
			t.Fatal(r.Method, r.URL)
			return nil
		}
	})
	env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
	env.Service.Chat = func(ctx context.Context, _ *serviceclient.Client, _ []string, input io.Reader, _ io.Writer, _ bool) error {
		err := sessionPresentation(ctx, "", 0).Switch("old", &out)
		if err == nil || !strings.Contains(err.Error(), "Work changed") {
			t.Fatal("switch ignored concurrent work", err)
		}
		remaining, _ := io.ReadAll(input)
		if string(remaining) != "/exit\n" {
			t.Fatal(string(remaining))
		}
		return nil
	}
	if err := runInteractiveAgent(context.Background(), []string{"--profile", "profile-a"}, env); err != nil {
		t.Fatal(err)
	}
	if creates != 1 {
		t.Fatal("started new conversation after race", creates)
	}
}

func TestProfileSelectionRecoversWithoutConsumingPrompt(t *testing.T) {
	for _, input := range []string{"0\n99\n2\nhello\n", "no match\nwriter\nhello\n", "draft\n1\nhello\n", "profile-b\nhello\n"} {
		reader := bufio.NewReader(strings.NewReader(input))
		var out bytes.Buffer
		profile, err := PickAgentProfilePlain(reader, &out, PresentationStyle{}, choiceProfiles())
		remaining, _ := io.ReadAll(reader)
		if err != nil || profile.ID != "profile-b" || string(remaining) != "hello\n" {
			t.Fatal(profile, err, string(remaining), out.String())
		}
	}
	profiles := choiceProfiles()
	profiles[1].Settings.Name = "Researcher"
	reader := bufio.NewReader(strings.NewReader("Researcher\n2\nhello\n"))
	p, err := PickAgentProfilePlain(reader, io.Discard, PresentationStyle{}, profiles)
	if err != nil || p.ID != "profile-b" {
		t.Fatal("ambiguous name selected implicitly", p, err)
	}
	for _, text := range []string{"/cancel\n", "/exit\n"} {
		_, err := PickAgentProfilePlain(bufio.NewReader(strings.NewReader(text)), io.Discard, PresentationStyle{}, profiles)
		if !errors.Is(err, ErrAgentSelectionCancelled) {
			t.Fatal(err)
		}
	}
}

func continuityClient(t *testing.T, response func(*http.Request) any) *serviceclient.Client {
	t.Helper()
	client, err := serviceclient.New("http://fixture:11611", "fixture", &http.Client{Transport: localRoundTrip(func(r *http.Request) (*http.Response, error) {
		data, _ := json.Marshal(response(r))
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data))}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestProfileModelNarrowsCatalogAndRejectsMissingMatch(t *testing.T) {
	client := continuityClient(t, func(r *http.Request) any {
		if r.Method != "GET" || r.URL.Path != "/api/v2/agent-profiles" {
			t.Fatal(r.Method, r.URL)
		}
		return map[string]any{"profiles": choiceProfiles()}
	})
	p, err := selectInteractiveProfile(context.Background(), client, interactiveAgentOptions{model: "model-b"}, bufio.NewReader(strings.NewReader("hello\n")), io.Discard, PresentationStyle{})
	if err != nil || p.ID != "profile-b" {
		t.Fatal(p, err)
	}
	_, err = selectInteractiveProfile(context.Background(), client, interactiveAgentOptions{model: "missing"}, nil, io.Discard, PresentationStyle{})
	if err == nil {
		t.Fatal("missing model allowed")
	}
}

type memoryAgentDefaults struct {
	values map[AgentDefaultScope]*AgentDefault
	err    error
	saves  int
}

func (s *memoryAgentDefaults) Load(scope AgentDefaultScope) (*AgentDefault, error) {
	return s.values[scope], s.err
}
func (s *memoryAgentDefaults) Save(scope AgentDefaultScope, value *AgentDefault) error {
	if s.err != nil {
		return s.err
	}
	s.saves++
	if s.values == nil {
		s.values = map[AgentDefaultScope]*AgentDefault{}
	}
	s.values[scope] = value
	return nil
}

func TestAgentDefaultsScopeAndRevision(t *testing.T) {
	workspace, actor, revision := "workspace-a", "alice", 2
	archived := false
	client := continuityClient(t, func(r *http.Request) any {
		if r.Method != "GET" {
			t.Fatal("default made service mutation", r.URL)
		}
		switch r.URL.Path {
		case "/api/v2/system":
			return map[string]any{"product": "offgrid", "workspace_id": workspace}
		case "/v1/users/me":
			return map[string]any{"auth_required": true, "authenticated": true, "user": map[string]string{"id": actor}}
		case "/api/v2/agent-profiles/profile-a":
			p := choiceProfiles()[0]
			p.Revision = revision
			p.Archived = archived
			return p
		default:
			t.Fatal(r.URL)
			return nil
		}
	})
	store := &memoryAgentDefaults{}
	var out bytes.Buffer
	env := Environment{Stdout: &out, Service: ServiceDependencies{AgentDefaults: store}}
	agentDefaultCommand(context.Background(), client, store, choiceProfiles()[0], "/default", &out)
	if store.saves != 1 {
		t.Fatal(out.String())
	}
	get := func() interactiveAgentOptions {
		return rememberedAgent(context.Background(), client, env, interactiveAgentOptions{})
	}
	if o := get(); o.profile != "profile-a" || o.revision != 2 {
		t.Fatal(o, out.String())
	}
	workspace = "workspace-b"
	if o := get(); o.profile != "" {
		t.Fatal("cross-workspace default", o)
	}
	workspace = "workspace-a"
	actor = "bob"
	if o := get(); o.profile != "" {
		t.Fatal("cross-actor default", o)
	}
	actor = "alice"
	revision = 3
	if o := get(); o.profile != "" {
		t.Fatal("silently upgraded revision", o)
	}
	revision = 2
	archived = true
	if o := get(); o.profile != "" {
		t.Fatal("archived default", o)
	}
	archived = false
	store.err = errors.New("corrupt")
	if o := get(); o.profile != "" {
		t.Fatal("corrupt default", o)
	}
	store.err = nil
	o := rememberedAgent(context.Background(), client, env, interactiveAgentOptions{profile: "explicit", revision: 5})
	if o.profile != "explicit" {
		t.Fatal(o)
	}
	o = rememberedAgent(context.Background(), client, env, interactiveAgentOptions{model: "model-b"})
	if o.profile != "" {
		t.Fatal(o)
	}
	agentDefaultCommand(context.Background(), client, store, choiceProfiles()[0], "/default clear", &out)
	if get().profile != "" {
		t.Fatal("clear retained default")
	}
}

func TestAgentDefaultRequiresVerifiedIdentity(t *testing.T) {
	for _, account := range []string{`{}`, `{"auth_required":true}`, `{"authenticated":true,"user":{}}`, `{"auth_required":false,"guest":true}`, `{"auth_required":false}`} {
		t.Run(account, func(t *testing.T) {
			client := continuityClient(t, func(r *http.Request) any {
				if r.URL.Path == "/api/v2/system" {
					return map[string]string{"product": "offgrid", "workspace_id": "w"}
				}
				return json.RawMessage(account)
			})
			scope, err := agentDefaultScope(context.Background(), client)
			if account == `{"auth_required":false}` {
				if err != nil || scope.Actor != "local-admin" {
					t.Fatal(scope, err)
				}
			} else if err == nil {
				t.Fatal("unverified actor accepted", scope)
			}
		})
	}
}

func TestResumeUsesSavedRevisionAndRejectsConflicts(t *testing.T) {
	client := continuityClient(t, func(r *http.Request) any {
		if r.Method != "GET" || r.URL.Path != "/v1/sessions/saved" {
			t.Fatal(r.URL)
		}
		return json.RawMessage(`{"name":"saved","created_at":"2026-11-01T00:00:00Z","updated_at":"2026-11-01T00:00:00Z","turn":{"agent":{"id":"profile-a","revision":2},"model":"model-a","status":"completed"}}`)
	})
	o, err := resolveAgentResume(context.Background(), client, interactiveAgentOptions{resume: "saved"})
	if err != nil || o.profile != "profile-a" || o.revision != 2 {
		t.Fatal(o, err)
	}
	for _, options := range []interactiveAgentOptions{{resume: "saved", profile: "profile-b"}, {resume: "saved", profile: "profile-a", revision: 3}, {resume: "saved", model: "model-b"}} {
		if _, err := resolveAgentResume(context.Background(), client, options); err == nil {
			t.Fatal("conflict allowed", options)
		}
	}
}

func TestAgentSwitchPreservesConversationWhenBlockedOrCancelled(t *testing.T) {
	for _, status := range []string{"running", "interrupted", "outcome_unknown", "completed", "cancelled", "failed"} {
		t.Run(status, func(t *testing.T) {
			calls := 0
			var out bytes.Buffer
			client := continuityClient(t, func(r *http.Request) any {
				if r.Method != "GET" {
					t.Fatal("switch submitted or changed work", r.Method, r.URL)
				}
				switch r.URL.Path {
				case "/api/v2/system":
					return map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
				case "/v1/sessions/saved":
					return json.RawMessage(`{"name":"saved","created_at":"2026-11-01T00:00:00Z","updated_at":"2026-11-01T00:00:00Z"}`)
				case "/v1/sessions/saved/turn":
					return map[string]any{"job": map[string]string{"run_id": "run-old", "status": status}}
				default:
					t.Fatal(r.URL)
					return nil
				}
			})
			ctx := context.WithValue(context.Background(), agentSessionKey{}, agentSessionPresentation{Profile: choiceProfiles()[0], Switch: func(string, io.Writer) error { calls++; return ErrAgentSelectionCancelled }})
			err := RunConversationChat(ctx, client, []string{"chat", "saved", "--profile", "profile-a", "--profile-revision", "2"}, strings.NewReader("/agent\n/exit\n"), &out, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if status == "completed" || status == "cancelled" || status == "failed" {
				want = 1
			}
			if calls != want || !strings.Contains(out.String(), "Conversation saved") {
				t.Fatal(calls, out.String())
			}
		})
	}
}
