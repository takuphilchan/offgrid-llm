package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestServiceCommandsUseAuthenticatedClientAndStableJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer isolated-key" {
			t.Errorf("missing actor credential on %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/v1/sessions":
			_, _ = w.Write([]byte(`{"sessions":[]}`))
		case "/v1/rag/status":
			_, _ = w.Write([]byte(`{"enabled":false}`))
		case "/api/v2/system":
			_, _ = w.Write([]byte(`{"product":"offgrid","capabilities":["agent-profiles-v1"]}`))
		case "/api/v2/agent-profiles":
			_, _ = w.Write([]byte(`{"profiles":[]}`))
		case "/api/v2/jobs/job-1":
			_, _ = w.Write([]byte(`{"run_id":"job-1","status":"completed"}`))
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "isolated-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args  []string
		field string
	}{
		{[]string{"session", "list"}, "sessions"},
		{[]string{"knowledge", "status"}, "enabled"},
		{[]string{"agent", "profiles", "list"}, "profiles"},
		{[]string{"agent", "status", "job-1"}, "run_id"},
	} {
		var out, errOut bytes.Buffer
		env := testEnvironment(&out, &errOut, &fakeRunner{})
		env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
		if code := Execute(context.Background(), append([]string{"--json"}, tc.args...), env); code != 0 {
			t.Fatalf("%v: code=%d stderr=%q stdout=%q", tc.args, code, errOut.String(), out.String())
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || result[tc.field] == nil {
			t.Fatalf("%v: result=%q error=%v", tc.args, out.String(), err)
		}
	}
}

func TestServiceCommandExitAndNoLocalFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/v1/sessions", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":"unavailable","message":"fixture unavailable"}}`))
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	makeEnv := func() Environment {
		out.Reset()
		errOut.Reset()
		env := testEnvironment(&out, &errOut, &fakeRunner{})
		env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
		return env
	}
	if code := Execute(context.Background(), []string{"--json", "session", "list"}, makeEnv()); code != 1 || !strings.Contains(out.String(), `"error"`) {
		t.Fatalf("operational code=%d result=%q", code, out.String())
	}
	if code := Execute(context.Background(), []string{"--json", "session", "delete"}, makeEnv()); code != 2 || !strings.Contains(out.String(), `"invalid_usage"`) {
		t.Fatalf("usage code=%d result=%q", code, out.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := Execute(ctx, []string{"--json", "session", "list"}, makeEnv()); code != 130 || !strings.Contains(out.String(), `"cancelled"`) {
		t.Fatalf("cancellation code=%d result=%q", code, out.String())
	}
	env := makeEnv()
	env.Service.Client = nil
	if code := Execute(context.Background(), []string{"session", "list"}, env); code != 1 || !strings.Contains(errOut.String(), "authenticated service client unavailable") {
		t.Fatalf("fallback code=%d error=%q", code, errOut.String())
	}
}

func TestArtifactDownloadRequiresDigestAndExclusiveOutput(t *testing.T) {
	content := []byte("isolated artifact")
	digest := sha256.Sum256(content)
	wantDigest := hex.EncodeToString(digest[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer actor" || r.URL.Path != "/api/v2/jobs/job-1/artifact" || r.URL.Query().Get("digest") == "" {
			t.Errorf("unscoped request: %s", r.URL)
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "actor", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "artifact.txt")
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
	env.Service.CreateArtifactFile = func(name string) (ArtifactFile, error) {
		return os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	}
	bad := strings.Repeat("0", 64)
	if code := Execute(context.Background(), []string{"agent", "artifact", "job-1", bad, "--output", path}, env); code != 1 {
		t.Fatalf("bad digest code=%d", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unverified artifact created: %v", err)
	}
	out.Reset()
	errOut.Reset()
	args := []string{"--json", "agent", "artifact", "job-1", wantDigest, "--output", path}
	if code := Execute(context.Background(), args, env); code != 0 || !strings.Contains(out.String(), `"verified":true`) {
		t.Fatalf("verified code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, content) {
		t.Fatalf("file=%q error=%v", data, err)
	}
	out.Reset()
	errOut.Reset()
	if code := Execute(context.Background(), args, env); code != 1 || !strings.Contains(out.String(), `"error"`) {
		t.Fatalf("overwrite code=%d out=%q", code, out.String())
	}
}

func TestSessionServiceRejectsCredentialedRedirect(t *testing.T) {
	visited := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visited = true
		if r.Header.Get("Authorization") != "" {
			t.Error("credential leaked to redirect target")
		}
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/capture", http.StatusFound)
	}))
	defer server.Close()
	client, err := serviceclient.New(server.URL, "actor-secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	env := testEnvironment(&out, &errOut, &fakeRunner{})
	env.Service.Client = func() (*serviceclient.Client, error) { return client, nil }
	if code := Execute(context.Background(), []string{"--json", "session", "list"}, env); code != 1 || visited || !strings.Contains(out.String(), `"error"`) {
		t.Fatalf("redirect code=%d visited=%v out=%q", code, visited, out.String())
	}
}
