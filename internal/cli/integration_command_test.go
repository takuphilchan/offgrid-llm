package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

func TestIntegrationUsesInjectedServiceAndInstallBoundary(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin.LoadWithPriority = func(path string) (*config.Config, error) {
		if path != "fixture-config" {
			t.Fatalf("wrong config path %q", path)
		}
		return &config.Config{ServerPort: 12999}, nil
	}
	env.Getenv = func(key string) string {
		if key == "OFFGRID_CONFIG" {
			return "fixture-config"
		}
		return ""
	}
	env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "127.0.0.1:12999" {
			t.Fatalf("unexpected host %s", req.URL.Host)
		}
		var body string
		switch req.URL.Path {
		case "/v1/integrations":
			body = `{"provider":"offgrid","integrations":[{"id":"hermes","name":"Hermes","ready":true,"provider_id":"offgrid","model_id":"fixture","context_window":8192,"minimum_context":4096}]}`
		case "/v1/integrations/hermes/setup":
			if req.URL.Query().Get("model") != "fixture" {
				t.Fatalf("missing model query: %s", req.URL)
			}
			body = `{"integration":{"name":"Hermes"},"setup":{"provider_id":"offgrid","install":"install","config_file":"config","content":"fixture setup"}}`
		default:
			t.Fatalf("unexpected endpoint %s", req.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	installed := 0
	env.Integrations = IntegrationDependencies{
		HomeDir: func() (string, error) { return t.TempDir(), nil },
		Install: func(id, home string, force bool) (integrations.InstallResult, error) {
			installed++
			if id != "hermes" || force {
				t.Fatalf("wrong install request %q force=%v", id, force)
			}
			return integrations.InstallResult{Integration: id, Path: home + "/plugin"}, nil
		},
		LookPath: func(string) (string, error) { return "", nil },
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"integration", "list"}, "Hermes"},
		{[]string{"integration", "setup", "hermes", "--model", "fixture"}, "fixture setup"},
		{[]string{"integration", "install", "hermes"}, "Prepared the native OffGrid provider"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 0 || !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", tc.args, code, out.String(), errOut.String())
		}
	}
	if installed != 1 {
		t.Fatalf("install calls = %d", installed)
	}
	// Validation fails before the installer is asked to touch user files.
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"integration", "install", "hermes", "--unknown"}, env); code != 0 || installed != 1 || !strings.Contains(errOut.String(), "Unknown option") {
		t.Fatalf("invalid install: code=%d installs=%d stderr=%q", code, installed, errOut.String())
	}
}
