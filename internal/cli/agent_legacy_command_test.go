package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestLegacyAgentRoutesThroughInjectedBoundary(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("test prompt\n/exit\n")
	dataDir := t.TempDir()
	env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ServerPort: 12999, DataDir: dataDir} }
	chats := 0
	env.LegacyAgent = LegacyAgentDependencies{
		SubmitTask: func(_ context.Context, args []string) error {
			chats++
			t.Fatal("legacy independent task submission must not be used by interactive chat")
			return nil
		},
		MCP: MCPDependencies{ReadFile: os.ReadFile, MkdirAll: os.MkdirAll, WriteFile: os.WriteFile, Now: time.Now},
	}
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "localhost:12999" {
			t.Fatalf("host=%s", req.URL.Host)
		}
		body := ""
		switch req.URL.Path {
		case "/v1/agents/tasks":
			if req.Header.Get("Authorization") != "Bearer fixture-key" {
				t.Fatal("missing task authentication")
			}
			body = `[{"id":"task-1","status":"completed","prompt":"fixture task"}]`
		case "/v1/agents/workflows":
			body = `[{"name":"fixture workflow","steps":[{}]}]`
		case "/v1/agents/tools":
			body = `{"count":1,"tools":[{"name":"read_file","description":"fixture","type":"builtin"}]}`
		default:
			t.Fatalf("path=%s", req.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	env.Service.Client = func() (*serviceclient.Client, error) {
		return serviceclient.New("http://localhost:12999", "fixture-key", env.HTTP)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"agent", "templates"}, "Agent Templates"},
		{[]string{"agent", "list"}, "fixture task"},
		{[]string{"agent", "workflows"}, "fixture workflow"},
		{[]string{"agent", "tools"}, "Read file contents"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 0 || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", tc.args, code, out.String(), errOut.String())
		}
	}
	for _, args := range [][]string{{"agent", "mcp", "list"}, {"agent", "help"}} {
		if code := Execute(context.Background(), args, env); code != 0 {
			t.Fatalf("%v: code=%d", args, code)
		}
	}
	if chats != 0 || !strings.Contains(out.String(), "Agent workspace") || !strings.Contains(out.String(), "--detach") || !strings.Contains(out.String(), "--workspace-folder") {
		t.Fatalf("interactive/help result chats=%d stdout=%q", chats, out.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"agent", "chat", "--bad"}, env); code != 2 || chats != 0 || !strings.Contains(errOut.String(), "Unknown or incomplete option") {
		t.Fatalf("invalid chat: code=%d chats=%d stderr=%q", code, chats, errOut.String())
	}
}

func TestInteractiveAgentUsesCanonicalConversationAndExplicitProfileModel(t *testing.T) {
	for _, resume := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "resume"}[resume], func(t *testing.T) {
			var out, errOut bytes.Buffer
			env := testEnvironment(&out, &errOut, &fakeRunner{})
			env.Stdin = strings.NewReader("list this folder\nnow read its README\n/exit\n")
			session := "saved-session"
			created, submitted, workspaceLookups := 0, 0, 0
			var latest any = map[string]any{"turn": nil}
			seen := map[string]bool{}
			env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
				if req.URL.Host != "configured-service:12999" || req.Header.Get("Authorization") != "Bearer fixture-key" {
					t.Fatalf("wrong endpoint/auth: %s", req.URL)
				}
				var body any
				switch {
				case req.URL.Path == "/api/v2/execution/workspaces":
					workspaceLookups++
					body = map[string]any{"protocol": 1, "workspaces": []any{map[string]any{"reference": map[string]any{"host": "host", "session": "session", "grant": "grant", "revision": 3}, "root": "/fixture/project", "directory": ".", "backend": "local"}}}
				case req.URL.Path == "/api/v2/system":
					body = map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
				case strings.HasPrefix(req.URL.Path, "/api/v2/agent-profiles/"):
					body = map[string]any{"id": "profile-one", "revision": 7, "settings": map[string]any{"name": "General", "model": "explicit-model", "max_iterations": 30}}
				case req.URL.Path == "/v1/sessions" && req.Method == "POST":
					var data map[string]string
					if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
						t.Fatal(err)
					}
					if data["model_id"] != "explicit-model" {
						t.Fatal(data)
					}
					session = data["name"]
					created++
					body = map[string]any{"name": session}
				case req.URL.Path == "/v1/sessions/"+session+"/turn":
					body = latest
				case req.URL.Path == "/v1/sessions/"+session+"/generate":
					var data map[string]any
					if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
						t.Fatal(err)
					}
					if data["model_id"] != "explicit-model" {
						t.Fatal("lost model override", data)
					}
					ref, ok := data["execution"].(map[string]any)
					if !ok || ref["session"] != "session" || ref["revision"] != float64(3) {
						t.Fatal("lost execution binding", data)
					}
					id, _ := data["request_id"].(string)
					if id == "" || seen[id] {
						t.Fatal("duplicate/empty turn", id)
					}
					seen[id] = true
					submitted++
					job := "job-" + id
					latest = map[string]any{"turn": map[string]any{"id": id, "job_id": job}, "job": map[string]any{"run_id": job, "status": "completed", "output": "fixture result"}}
					body = latest
				case req.URL.Path == "/v1/sessions/"+session:
					body = map[string]any{"name": session, "model_id": "explicit-model", "created_at": "2026-10-25T00:00:00Z", "updated_at": "2026-10-25T00:00:00Z", "messages": []any{}}
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL)
				}
				encoded, _ := json.Marshal(body)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(encoded)), Header: http.Header{}}, nil
			})}
			env.Service.Client = func() (*serviceclient.Client, error) {
				return serviceclient.New("http://configured-service:12999", "fixture-key", env.HTTP)
			}
			env.LegacyAgent.SubmitTask = func(context.Context, []string) error { t.Fatal("submitted independent job"); return nil }
			args := []string{"agent", "chat", "--profile", "profile-one", "--profile-revision", "7", "--model", "explicit-model", "--workspace", "session"}
			if resume {
				args = append(args, "--resume", session)
			}
			if code := Execute(context.Background(), args, env); code != 0 {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), errOut.String())
			}
			expectedCreates := 1
			if resume {
				expectedCreates = 0
			}
			if created != expectedCreates || submitted != 2 || workspaceLookups != 1 || strings.Count(out.String(), "OffGrid  /  Agent chat") != 1 || !strings.Contains(out.String(), "explicit-model") {
				t.Fatalf("created=%d turns=%d output=%s", created, submitted, out.String())
			}
		})
	}
}

func TestInteractiveOptionsAreValidatedBeforeRequests(t *testing.T) {
	for _, args := range [][]string{{"--profile-revision", "1"}, {"--model", "a", "--model", "b"}, {"--resume", "../escape"}, {"--profile"}, {"--unknown", "value"}} {
		if _, err := parseInteractiveAgent(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
}
