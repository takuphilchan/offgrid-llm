package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

type endpointFixtureTransport func(*http.Request) (*http.Response, error)

func (f endpointFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type noEndpointFixtureProcess struct{}

func (noEndpointFixtureProcess) Run(context.Context, cli.CommandRequest) error {
	return errors.New("no host process authorized by this transport fixture")
}

func TestLegacyAgentClientKeepsSelectedOriginAndCredentials(t *testing.T) {
	t.Setenv("OFFGRID_SERVER_URL", "http://selected.invalid:12345")
	t.Setenv("OFFGRID_API_KEY", "selected-fixture-key")
	previous := httpClient
	defer func() { httpClient = previous }()
	requests := 0
	httpClient = &http.Client{Transport: endpointFixtureTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.URL.Host != "selected.invalid:12345" || r.Header.Get("Authorization") != "Bearer selected-fixture-key" {
			t.Error("legacy listing redirected selected authority")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	client, err := legacyAgentEnvironment(cli.Environment{}).Service.Client()
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(context.Background(), "GET", "/api/v2/jobs", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if requests != 1 {
		t.Fatal("legacy request was omitted or repeated", requests)
	}
}

func TestInteractiveAgentRetainsSelectedAuthenticatedService(t *testing.T) {
	// Both transports are in-memory. A regression must never reach the user's
	// actual localhost service or create a profile, conversation or host grant.
	previous := httpClient
	defer func() { httpClient = previous }()
	legacyRequests := 0
	response := func() *http.Response {
		return &http.Response{StatusCode: 401, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"code":"unauthenticated"}}`))}
	}
	httpClient = &http.Client{Transport: endpointFixtureTransport(func(r *http.Request) (*http.Response, error) { legacyRequests++; return response(), nil })}
	for _, args := range [][]string{{"offgrid", "agent"}, {"offgrid", "agent", "chat"}, {"offgrid", "agent", "--model", "explicit-model"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			selectedRequests := 0
			transport := &http.Client{Transport: endpointFixtureTransport(func(r *http.Request) (*http.Response, error) {
				selectedRequests++
				if r.URL.Host != "selected.invalid:12345" || r.Header.Get("Authorization") != "Bearer selected-fixture-key" || r.Method != "GET" {
					t.Error("selected endpoint, credentials or read-only discovery changed")
				}
				return response(), nil
			})}
			client, err := serviceclient.New("http://selected.invalid:12345", "selected-fixture-key", transport)
			if err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			env := cli.Environment{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &stderr, Getenv: func(string) string { return "" }, Setenv: func(string, string) error { return nil }, HTTP: transport, LongHTTP: transport, Exec: noEndpointFixtureProcess{}, Service: cli.ServiceDependencies{Client: func() (*serviceclient.Client, error) { return client, nil }}}
			if code := runRoot(context.Background(), args, env); code == 0 || selectedRequests != 1 || legacyRequests != 0 {
				t.Fatalf("interactive discovery ignored selected service: code=%d selected=%d legacy=%d", code, selectedRequests, legacyRequests)
			}
		})
	}
}
