package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestInteractiveDefaultSelectsProfileWithoutLegacyTask(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("/exit\n")
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		var result any
		switch req.URL.Path {
		case "/api/v2/system":
			result = map[string]any{"product": "offgrid", "capabilities": []string{"agent-conversations-v1"}}
		case "/api/v2/agent-profiles":
			result = map[string]any{"profiles": []any{map[string]any{"id": "owned-profile", "revision": 4, "settings": map[string]any{"model": "installed-model", "name": "General", "max_iterations": 30}}}}
		case "/v1/sessions":
			var body map[string]string
			json.NewDecoder(req.Body).Decode(&body)
			result = map[string]string{"name": body["name"]}
		default:
			t.Fatal("unexpected request", req.URL)
		}
		encoded, _ := json.Marshal(result)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(encoded))}, nil
	})}
	env.Service.Client = func() (*serviceclient.Client, error) {
		return serviceclient.New("http://configured:11611", "fixture", env.HTTP)
	}
	chats := 0
	env.Service.Chat = func(_ context.Context, _ *serviceclient.Client, args []string, input io.Reader, _ io.Writer, _ bool) error {
		chats++
		if len(args) != 6 || args[0] != "chat" || args[2] != "--profile" || args[3] != "owned-profile" || args[5] != "4" {
			t.Fatal(args)
		}
		data, _ := io.ReadAll(input)
		if string(data) != "/exit\n" {
			t.Fatal("lost buffered input", string(data))
		}
		return nil
	}
	if code := Execute(context.Background(), []string{"agent"}, env); code != 0 || chats != 1 {
		t.Fatal(code, chats, errOut.String())
	}
}

func TestInteractiveModelMismatchDoesNotCreateOrSubmit(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Stdin = strings.NewReader("/exit\n")
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" {
			t.Fatal("model mismatch created state", req.Method, req.URL)
		}
		body := `{"product":"offgrid","capabilities":["agent-conversations-v1"]}`
		if req.URL.Path == "/api/v2/agent-profiles/owned" {
			body = `{"id":"owned","revision":1,"settings":{"model":"pinned-model","max_iterations":30}}`
		} else if req.URL.Path != "/api/v2/system" {
			t.Fatal(req.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	env.Service.Client = func() (*serviceclient.Client, error) { return serviceclient.New("http://fixture:11611", "", env.HTTP) }
	if code := Execute(context.Background(), []string{"agent", "chat", "--profile", "owned", "--model", "another-model"}, env); code != 2 || !strings.Contains(errOut.String(), "pinned model") {
		t.Fatal(code, errOut.String())
	}
}

func TestInitialProfileSetupRequiresConsentAndAddsNoAuthority(t *testing.T) {
	for _, answer := range []string{"n\n", "y\n"} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			creates := 0
			httpClient := &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
				body := `{"data":[{"id":"actual-model","type":"chat"},{"id":"embedding","type":"embedding"}]}`
				if req.Method == "POST" {
					creates++
					var request struct {
						Settings struct {
							Name, Model   string
							MaxIterations int `json:"max_iterations"`
							Tools         []any
							Computer      bool
						}
					}
					if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
						t.Fatal(err)
					}
					if request.Settings.Name != "General" || request.Settings.Model != "actual-model" || request.Settings.MaxIterations != 30 || len(request.Settings.Tools) != 0 || request.Settings.Computer {
						t.Fatal("changed authority", request)
					}
					body = `{"id":"new-profile","revision":1}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			client, _ := serviceclient.New("http://fixture:11611", "", httpClient)
			_, err := setupInteractiveProfile(context.Background(), client, "actual-model", bufio.NewReader(strings.NewReader(answer)), io.Discard)
			if answer == "y\n" {
				if err != nil || creates != 1 {
					t.Fatal(err, creates)
				}
			} else if err == nil || creates != 0 {
				t.Fatal(err, creates)
			}
		})
	}
}

func TestInteractiveInterruptStopsOnlyItsOwnTurn(t *testing.T) {
	for _, matching := range []bool{true, false} {
		t.Run(map[bool]string{true: "own", false: "other"}[matching], func(t *testing.T) {
			stops := 0
			transport := &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
				if req.Context().Err() != nil {
					t.Fatal("reused cancelled context for stop inspection")
				}
				id := "turn-current"
				if !matching {
					id = "other-turn"
				}
				body := map[string]any{"turn": map[string]any{"id": id, "job_id": "job-current"}, "job": map[string]any{"run_id": "job-current", "status": "running"}}
				if req.Method == "POST" {
					stops++
					if req.URL.Path != "/api/v2/jobs/job-current/cancel" {
						t.Fatal(req.URL)
					}
					body = map[string]any{"run_id": "job-current", "status": "cancelled"}
				}
				encoded, _ := json.Marshal(body)
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(encoded))}, nil
			})}
			client, _ := serviceclient.New("http://fixture:11611", "", transport)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			err := stopInteractiveTurn(ctx, client, "conversation", "turn-current", io.Discard)
			if matching {
				if err != nil || stops != 1 {
					t.Fatal(err, stops)
				}
			} else if err == nil || stops != 0 {
				t.Fatal("stopped unrelated turn", err, stops)
			}
		})
	}
}
