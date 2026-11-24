package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/internal/users"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// The transport is an isolated loopback fixture with an explicit no-op local
// lease, never a production enrollment exemption. Selection and revocation use
// the real trusted-file route, durable policy, scheduler and HTTP adapter.
func TestEndpointManagementDisableStopsActualHTTPAndNoLaterDispatch(t *testing.T) {
	s, entry := endpointPolicyFixture(t)
	reviewEndpointFixture(t, s, entry)
	var calls atomic.Int32
	started := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/version":
			fmt.Fprintf(w, `{"version":%q}`, entry.RuntimeVersion)
		case "/api/tags":
			fmt.Fprintf(w, `{"models":[{"name":%q,"digest":%q}]}`, entry.Model, strings.TrimPrefix(entry.ModelDigest, "sha256:"))
		default:
			if calls.Add(1) == 1 {
				close(started)
			}
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	defer upstream.Close()
	descriptor := endpointDescriptor(entry)
	endpoint := inference.HTTPEndpoint{URL: upstream.URL, Model: entry.Model, RuntimeVersion: entry.RuntimeVersion, AllowedIPs: []string{"127.0.0.1"}, Descriptor: descriptor,
		Authorize: func(ctx context.Context) (string, error) {
			_, _, err := s.selectEndpointChat(ctx, endpointKey(entry))
			return "", err
		},
		AcquireRequest: func(ctx context.Context) (context.Context, func(), error) {
			return s.endpointPolicy.acquire(ctx, entry)
		},
		AcquireLocal: func(context.Context) (func() error, error) { return func() error { return nil }, nil },
	}
	routes, err := inference.NewHTTPEndpointRoutes(map[string]inference.HTTPEndpoint{endpointKey(entry): endpoint})
	if err != nil {
		t.Fatal(err)
	}
	registry := inference.NewProviderRegistry()
	if err = routes.Register(registry); err != nil {
		t.Fatal(err)
	}
	s.providerScheduler, err = inference.NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer s.providerScheduler.Close()
	state := endpointSnapshotFixture(t, s, "")
	probe := endpointManagementRequest(s, "", "/probe", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision})
	if probe.Code != 200 || !strings.Contains(probe.Body.String(), `"connection_verified":true`) || !strings.Contains(probe.Body.String(), `"qualified":false`) || calls.Load() != 0 {
		t.Fatal("metadata probe generated or claimed qualification", probe.Code, probe.Body.String(), calls.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = inference.WithSchedulingScope(ctx, inference.SchedulingScope{Actor: "local", Work: "endpoint-policy-fixture"})
	done := make(chan error, 1)
	go func() {
		_, err := s.completeChat(ctx, &api.ChatCompletionRequest{Model: "endpoint:" + endpointKey(entry), Messages: []api.ChatMessage{{Role: "user", Content: "fixture"}}})
		done <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("provider did not receive initial request")
	}
	queued := make(chan error, 1)
	go func() {
		_, err := s.completeChat(ctx, &api.ChatCompletionRequest{Model: "endpoint:" + endpointKey(entry), Messages: []api.ChatMessage{{Role: "user", Content: "queued before revocation"}}})
		queued <- err
	}()
	for s.providerScheduler.AdmissionStatus().ForegroundQueued != 1 {
		if ctx.Err() != nil {
			t.Fatal("second request did not enter shared scheduler queue")
		}
		time.Sleep(time.Millisecond)
	}
	state = endpointSnapshotFixture(t, s, "")
	disabled := false
	w := endpointManagementRequest(s, "", "/review", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision, Enabled: &disabled})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("revoked generation reported success")
		}
	case <-ctx.Done():
		t.Fatal("revoked HTTP request did not finish")
	}
	select {
	case err := <-queued:
		if err == nil {
			t.Fatal("previously queued generation reported success")
		}
	case <-ctx.Done():
		t.Fatal("queued revoked generation did not settle")
	}
	for i := 0; i < 3; i++ {
		if _, err := s.completeChat(ctx, &api.ChatCompletionRequest{Model: "endpoint:" + endpointKey(entry), Messages: []api.ChatMessage{{Role: "user", Content: "must not dispatch"}}}); err == nil {
			t.Fatal("revoked request accepted")
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("revocation dispatched or replayed request: %d", calls.Load())
	}
}

func TestEndpointManagementCurrentAdminAndEnrollmentScopeRemainCeiling(t *testing.T) {
	s, entry := endpointPolicyFixture(t)
	s.config.RequireAuth = true
	s.userStore = users.NewUserStore("")
	admin, key, err := s.userStore.CreateUser("manager", "isolated-manager-password", users.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	_, userKey, err := s.userStore.CreateUser("reader", "isolated-reader-password", users.RoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if w := endpointManagementRequest(s, userKey, "", nil); w.Code != 403 {
		t.Fatal("non-admin read endpoint configuration", w.Code)
	}
	state := endpointSnapshotFixture(t, s, key)
	enabled := true
	body := endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision, Enabled: &enabled}
	if w := endpointManagementRequest(s, key, "/review", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	state = endpointSnapshotFixture(t, s, key)
	if state.Items[0].ProbeAllowed {
		t.Fatal("administrator broadened trusted actor scope")
	}
	if w := endpointManagementRequest(s, key, "/probe", endpointReview{ID: entry.ID, Revision: entry.Revision, ConfigPin: endpointPin(entry), PolicyRevision: state.PolicyRevision}); w.Code != 403 {
		t.Fatal("probe bypassed enrolled actor authority", w.Code)
	}
	var retainedContext context.Context
	endpointAuthenticatedRequest(t, s, key, "/fixture", "", func(_ http.ResponseWriter, r *http.Request) { retainedContext = r.Context() })
	if err = s.userStore.UpdateUser(admin.ID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, inferenceEndpointsPath, nil).WithContext(retainedContext)
	w := httptest.NewRecorder()
	s.handleInferenceEndpoints(w, r)
	if w.Code != 403 {
		t.Fatal("stale authenticated identity retained admin access", w.Code)
	}
}
