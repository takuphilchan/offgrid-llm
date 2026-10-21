package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestComputerServiceUsesInjectedAuthenticatedClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/computer/capabilities" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Errorf("unexpected request: %s %s auth=%q", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"ready":true}`))
	}))
	defer server.Close()
	makeClient := func() (*serviceclient.Client, error) {
		return serviceclient.New(server.URL, "fixture-key", server.Client())
	}
	var stdout bytes.Buffer
	if err := RunComputerService(context.Background(), []string{"status"}, makeClient, &stdout); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "{\n  \"ready\": true\n}\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestComputerServiceRejectsInvalidUsageBeforeClient(t *testing.T) {
	called := false
	makeClient := func() (*serviceclient.Client, error) {
		called = true
		return nil, nil
	}
	if err := RunComputerService(context.Background(), []string{"check"}, makeClient, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "computer check") || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestComputerRoutingUsesBoundedInjectedAdapters(t *testing.T) {
	var calls []string
	deps := ComputerDependencies{
		Setup: func(_ context.Context, id string) error { calls = append(calls, "setup:"+id); return nil },
		Control: func(_ context.Context, args []string) error {
			calls = append(calls, strings.Join(args, ":"))
			return nil
		},
		Submit: func(_ context.Context, args []string) error {
			calls = append(calls, "submit:"+strings.Join(args, ":"))
			return nil
		},
		Stdout: &bytes.Buffer{},
	}
	for _, args := range [][]string{{"setup", "run-1"}, {"stop", "run-1"}, {"run", "draft", "--model", "fixture"}} {
		if err := RunComputer(context.Background(), args, deps); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	want := []string{"setup:run-1", "cancel:run-1", "submit:draft:--model:fixture"}
	if strings.Join(calls, "|") != strings.Join(want, "|") {
		t.Fatalf("calls=%v, want %v", calls, want)
	}
	if err := RunComputer(context.Background(), []string{"setup"}, ComputerDependencies{}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing adapter error=%v", err)
	}
}
