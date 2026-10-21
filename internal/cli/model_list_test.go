package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteModelListUsesInjectedAuthenticatedService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Errorf("unexpected service request: %s %s auth=%q", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"fixture-model","size":1024}]}`))
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	env := testEnvironment(&stdout, &stderr, &fakeRunner{})
	env.ServerURL = server.URL
	env.Getenv = func(key string) string {
		if key == "OFFGRID_API_KEY" {
			return "fixture-key"
		}
		return ""
	}
	if code := Execute(context.Background(), []string{"--json", "list"}, env); code != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"name":"fixture-model"`) || !strings.Contains(stdout.String(), `"server":"`+server.URL+`"`) || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestModelListHelpAndInvalidUsageAvoidService(t *testing.T) {
	var stdout, stderr bytes.Buffer
	p := Presenter{Stdout: &stdout, Stderr: &stderr}
	if err := RunList(context.Background(), []string{"--help"}, p, "", "", nil); err != nil || !strings.Contains(stdout.String(), "offgrid list") {
		t.Fatalf("help err=%v stdout=%q", err, stdout.String())
	}
	stdout.Reset()
	if err := RunList(context.Background(), []string{"--bad"}, p, "", "", nil); err == nil || stdout.Len() != 0 {
		t.Fatalf("invalid usage err=%v stdout=%q", err, stdout.String())
	}
}
