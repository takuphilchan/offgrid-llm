package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func executionHTTPFixture(t *testing.T) (*Server, *httptest.Server, *serviceclient.Client, *atomic.Int32) {
	return executionHTTPFixtureEnabled(t, true)
}

func executionHTTPFixtureEnabled(t *testing.T, enabled bool) (*Server, *httptest.Server, *serviceclient.Client, *atomic.Int32) {
	t.Helper()
	s := profileFixture(t)
	s.config.EnableExecutionPreview = enabled
	s.version = "development"
	s.executionHosts = execution.NewHosts()
	s.executionEnrollment = execution.NewEnrollment(s.executionHosts, s.version)
	var leaked atomic.Int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer user-secret" {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/api/v2/system":
			s.handleSystemIdentity(w, r)
		case "/api/v2/execution/pairing":
			s.handleExecutionPairing(w, r)
		case "/api/v2/execution/workspaces":
			s.handleExecutionWorkspaces(w, r)
		case "/api/v2/execution/revoke":
			s.handleExecutionRevoke(w, r)
		default:
			w.WriteHeader(404)
		}
	})
	transport := s.executionTransport(next)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/execution/host/") && r.Header.Get("Authorization") != "" {
			leaked.Add(1)
		}
		transport.ServeHTTP(w, r)
	}))
	client, err := serviceclient.New(server.URL, "user-secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.executionEnrollment.Close(); server.Close() })
	return s, server, client, &leaked
}

func connectHTTPExecutionFixture(t *testing.T, s *Server, client *serviceclient.Client) (*execution.HostConnection, execution.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var pair execution.Pairing
	if err := client.StrictJSON(ctx, "POST", "/api/v2/execution/pairing", struct{}{}, &pair, 4096); err != nil {
		t.Fatal(err)
	}
	config := execution.Context{Version: 1, Binding: execution.Binding{Actor: pair.Actor, Workspace: pair.Workspace, Host: "fixture-host", Session: "fixture-session", Grant: "fixture-grant", Revision: 1, Job: execution.BootstrapJob}, Backend: "local", Root: t.TempDir(), Directory: ".", Permissions: execution.Permissions{Read: true, Write: true}}
	session, err := execution.OpenSession(config)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := execution.OpenJournal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := &execution.Worker{Session: session, Journal: journal, Processes: execution.NewProcessManager(session), Token: strings.Repeat("a", 64)}
	endpoint := execution.NewLocalEndpoint(worker)
	t.Cleanup(func() { endpoint.Close(); session.Close(); journal.Close() })
	file := "offgrid-executor"
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	pack := execution.Pack{Schema: 1, Protocol: 1, Revision: execution.PackRevision, Version: "development", Platform: runtime.GOOS, Architecture: runtime.GOARCH, File: file, SHA256: strings.Repeat("d", 64)}
	connection, err := execution.ConnectApproved(ctx, client, endpoint, config, pack, pair.Code, strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	return connection, config
}

func TestAuthenticatedEnrollmentAndDelegatedHTTPFileFlow(t *testing.T) {
	s, _, client, leaked := executionHTTPFixture(t)
	connection, config := connectHTTPExecutionFixture(t, s, client)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.executionHosts.Admit(ctx, config.Binding.Actor, config.Binding.Workspace, "canonical-job", connection.Reference); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"path": "verified.txt", "data": []byte("café 日本語\n1| real line")})
	result, err := s.executionHosts.Call(ctx, config.Binding.Actor, config.Binding.Workspace, "canonical-job", "create-once", connection.Reference, "file_write", args)
	if err != nil || result.Receipt == nil || result.Receipt.State != "complete" {
		t.Fatal(result, err)
	}
	actual, err := os.ReadFile(filepath.Join(config.Root, "verified.txt"))
	if err != nil || string(actual) != "café 日本語\n1| real line" {
		t.Fatal(string(actual), err)
	}
	var catalog struct {
		Workspaces []execution.Resource `json:"workspaces"`
		Protocol   int                  `json:"protocol"`
	}
	if err = client.JSON(ctx, "GET", "/api/v2/execution/workspaces", nil, &catalog); err != nil || len(catalog.Workspaces) != 1 {
		t.Fatal(catalog, err)
	}
	if leaked.Load() != 0 {
		t.Fatal("user API credential reached host-only channel")
	}
	if err = client.JSON(ctx, "POST", "/api/v2/execution/revoke", map[string]any{"reference": connection.Reference}, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-connection.Done():
	case <-ctx.Done():
		t.Fatal("revoked host kept running")
	}
	if len(s.executionHosts.List(ctx, config.Binding.Actor, config.Binding.Workspace)) != 0 {
		t.Fatal("revoked host remained ready")
	}
}

func TestHostTransportIsNarrowAndPreviewDoesNotMintAuthority(t *testing.T) {
	s, server, _, _ := executionHTTPFixture(t)
	for _, path := range []string{"/api/v2/execution/pairing", "/api/v2/execution/revoke", "/api/v2/jobs", "/api/v2/execution/host/poll/", "/api/v2/execution/host/anything"} {
		response, err := server.Client().Post(server.URL+path, "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatal("transport bypass expanded", path, response.StatusCode)
		}
	}
	response, err := server.Client().Post(server.URL+"/api/v2/execution/host/poll", "application/json", strings.NewReader(`{"session":"foreign","token":"`+strings.Repeat("f", 64)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 410 {
		t.Fatal("unpaired host accepted", response.StatusCode)
	}
	_, disabledServer, _, _ := executionHTTPFixtureEnabled(t, false)
	response, err = disabledServer.Client().Post(disabledServer.URL+"/api/v2/execution/host/enroll", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 503 {
		t.Fatal("disabled preview accepted enrollment", response.StatusCode)
	}
	if len(s.executionHosts.List(context.Background(), "local-admin", s.workspaceID)) != 0 {
		t.Fatal("preview minted a grant")
	}
}

func TestExecutionEnvelopeRejectsUnknownDuplicateAndCrossSiteInputs(t *testing.T) {
	_, server, _, _ := executionHTTPFixture(t)
	for _, body := range []string{`null`, `{"session":"a","session":"b","token":"x"}`, `{"session":"a","token":"x","command":"whoami"}`} {
		response, err := server.Client().Post(server.URL+"/api/v2/execution/host/poll", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatal(body, response.StatusCode)
		}
	}
	request, _ := http.NewRequest("POST", server.URL+"/api/v2/execution/host/poll", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal(response.StatusCode)
	}
}
