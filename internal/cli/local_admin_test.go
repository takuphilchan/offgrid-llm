package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

type localRoundTrip func(*http.Request) (*http.Response, error)

func (fn localRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestMetricsUsesInjectedLocalEndpointAndPreservesShapes(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		json             bool
		fail             bool
	}{
		{name: "text", body: "# HELP counter\ncounter 3\n", want: "Prometheus Metrics\n\n# HELP counter\ncounter 3\n\n"},
		{name: "json", body: "# HELP counter\ncounter 3\n", want: "{\n  \"counter\": \"3\"\n}\n", json: true},
		{name: "offline", want: "Server not running", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			env := testEnvironment(&out, &errOut, &fakeRunner{})
			env.LocalAdmin.LoadConfig = func() *config.Config { return &config.Config{ServerPort: 11737} }
			env.HTTP = &http.Client{Transport: localRoundTrip(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "http://localhost:11737/metrics" {
					t.Fatalf("unexpected metrics target %s", req.URL)
				}
				if tc.fail {
					return nil, errors.New("offline")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{}}, nil
			})}
			args := []string{"metrics"}
			if tc.json {
				args = append(args, "--json")
			}
			if code := Execute(context.Background(), args, env); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, errOut.String())
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output %q missing %q", out.String(), tc.want)
			}
		})
	}
}

func TestConfigUsesDisposableAdaptersAndLegacyExits(t *testing.T) {
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	root := t.TempDir()
	path := filepath.Join(root, "config.yaml")
	env.LocalAdmin = LocalAdminDependencies{
		LoadConfig: func() *config.Config { return &config.Config{ServerPort: 11737} },
		LoadWithPriority: func(got string) (*config.Config, error) {
			if got != path {
				t.Fatalf("wrong priority path %q", got)
			}
			return &config.Config{ServerPort: 11737}, nil
		},
		LoadFromFile: func(got string) (*config.Config, error) {
			if got != path {
				t.Fatalf("wrong validation path %q", got)
			}
			return nil, errors.New("invalid fixture")
		},
		SaveConfig: func(cfg *config.Config, got string) error {
			if cfg.ServerPort != 11737 || got != path {
				t.Fatalf("wrong save request: %q", got)
			}
			return nil
		},
		HomeDir:  func() (string, error) { return root, nil },
		MkdirAll: func(string, uint32) error { return nil },
	}
	env.Getenv = func(key string) string {
		if key == "OFFGRID_CONFIG" {
			return path
		}
		return ""
	}
	for _, tc := range []struct {
		args     []string
		code     int
		out, err string
	}{
		{[]string{"config"}, 1, "Configuration", ""},
		{[]string{"config", "init", path}, 0, "Created config: " + path, ""},
		{[]string{"config", "show"}, 0, "Port:             11737", ""},
		{[]string{"config", "validate", path}, 1, "", "Invalid config: invalid fixture"},
		{[]string{"config", "validate"}, 1, "", "Usage: offgrid config validate <path>"},
		{[]string{"config", "invalid"}, 1, "", "Unknown action: invalid"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != tc.code {
			t.Fatalf("%v code=%d, want %d", tc.args, code, tc.code)
		}
		if !strings.Contains(out.String(), tc.out) || !strings.Contains(errOut.String(), tc.err) {
			t.Fatalf("%v stdout=%q stderr=%q", tc.args, out.String(), errOut.String())
		}
	}
}

func TestUsersAndAuditStayOnDisposableLocalState(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.LocalAdmin = LocalAdminDependencies{
		LoadConfig:      func() *config.Config { return &config.Config{DataDir: root, MultiUserMode: true} },
		NewUserStore:    users.NewUserStore,
		NewQuotaManager: users.NewQuotaManager,
		NewAuditLogger:  audit.NewAuditLogger,
		GlobAuditFiles:  filepath.Glob,
		Entropy:         strings.NewReader(strings.Repeat("z", 18)),
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"users", "list"}, "No users configured"},
		{[]string{"users", "create", "fixture-user", "viewer"}, "Created user: fixture-user"},
		{[]string{"users", "list"}, "fixture-user"},
		{[]string{"audit", "show"}, "Audit Events"},
		{[]string{"audit", "stats"}, "Audit Statistics"},
		{[]string{"audit", "verify"}, "Verifying Audit Chain Integrity"},
	} {
		out.Reset()
		errOut.Reset()
		if code := Execute(context.Background(), tc.args, env); code != 0 {
			t.Fatalf("%v exit=%d stderr=%s", tc.args, code, errOut.String())
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Fatalf("%v stdout=%q missing %q", tc.args, out.String(), tc.want)
		}
	}
	store := users.NewUserStore(root)
	found, ok := store.GetUserByUsername("fixture-user")
	if !ok || found == nil {
		t.Fatal("user mutation was not recorded in disposable store")
	}
	out.Reset()
	if code := Execute(context.Background(), []string{"users", "list", "--json"}, env); code != 0 || !strings.Contains(out.String(), "\"username\": \"fixture-user\"") {
		t.Fatalf("JSON user projection changed: code=%d stdout=%q", code, out.String())
	}
	out.Reset()
	if code := Execute(context.Background(), []string{"users", "quota", found.ID}, env); code != 0 || !strings.Contains(out.String(), "Quota Usage") {
		t.Fatalf("quota output changed: code=%d stdout=%q", code, out.String())
	}
	exportPath := filepath.Join(root, "audit-export.json")
	out.Reset()
	if code := Execute(context.Background(), []string{"audit", "export-json", exportPath}, env); code != 0 {
		t.Fatalf("audit export failed: code=%d stdout=%q", code, out.String())
	}
	if payload, err := os.ReadFile(exportPath); err != nil || len(payload) == 0 {
		t.Fatalf("audit export missing from disposable directory: %v", err)
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), []string{"users", "create", "invalid", "root"}, env); code != 0 || !strings.Contains(errOut.String(), "Invalid role") {
		t.Fatalf("legacy invalid role exit/output changed: code=%d stderr=%q", code, errOut.String())
	}
	out.Reset()
	if code := Execute(context.Background(), []string{"users", "delete", found.ID}, env); code != 0 {
		t.Fatalf("user delete failed: code=%d stdout=%q", code, out.String())
	}
	if _, ok := users.NewUserStore(root).GetUser(found.ID); ok {
		t.Fatal("deleted test user remains in disposable store")
	}
}
