package inference

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestManagedOllamaSharesSchedulerAndRetiresBeforeSwitch(t *testing.T) {
	endpoint := managedOllamaFixture(t)
	routes, err := NewHTTPEndpointRoutes(map[string]HTTPEndpoint{"owned@r1": endpoint})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewProviderRegistry()
	if err := routes.Register(registry); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { scheduler.Close() })
	descriptor, err := routes.Select(context.Background(), "owned", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Generate(context.Background(), descriptor, httpProviderRequest(t, endpoint, nil), func(ProviderEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	child := managedFixtureProcess(t, endpoint.ManagedOllama.StateDirectory, "child")
	// Admission of the existing llama path must retire this unmeasured local
	// runtime before it can allocate memory; no parallel scheduler is involved.
	release, _, err := scheduler.AcquireGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if status, err := windows.WaitForSingleObject(child, 0); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("legacy admission preceded owned runtime exit", status, err)
	}
}

func TestManagedOllamaRejectsReplacementListenerWithoutSendingHTTP(t *testing.T) {
	endpoint := managedOllamaFixture(t)
	p, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	root := managedFixtureProcess(t, endpoint.ManagedOllama.StateDirectory, "root")
	if err := windows.TerminateProcess(root, 1); err != nil {
		t.Fatal(err)
	}
	if status, err := windows.WaitForSingleObject(root, 3000); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("fixture did not stop", status, err)
	}
	p.client.CloseIdleConnections()
	listener, err := net.Listen("tcp4", endpoint.URL[len("http://"):])
	if err != nil {
		t.Fatal(err)
	}
	var received atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.Write([]byte(`{"version":"fixture-1"}`))
	}), ReadHeaderTimeout: time.Second}
	go server.Serve(listener)
	defer server.Close()
	if err := p.Health(context.Background()); err == nil {
		t.Fatal("replacement listener impersonated owned runtime")
	}
	if received.Load() != 0 {
		t.Fatal("replacement listener received HTTP bytes")
	}
}

func TestManagedOllamaRechecksRevocationAfterHashingAndReleasesFailedStartup(t *testing.T) {
	endpoint := managedOllamaFixture(t)
	calls := 0
	endpoint.Authorize = func(context.Context) (string, error) {
		calls++
		if calls >= 3 {
			return "", errors.New("revoked after file checks")
		}
		return "", nil
	}
	if p, err := NewHTTPProvider(context.Background(), endpoint); err == nil || p != nil {
		t.Fatal("revoked launch succeeded")
	}
	if _, err := os.Stat(filepath.Join(endpoint.ManagedOllama.StateDirectory, "root.pid")); !os.IsNotExist(err) {
		t.Fatal("revoked actor started process")
	}
	if _, err := os.Stat(filepath.Join(endpoint.ManagedOllama.StateDirectory, ".offgrid-managed-ollama.lock")); !os.IsNotExist(err) {
		t.Fatal("failed startup retained state lock", err)
	}
	endpoint.Authorize = func(context.Context) (string, error) { return "", nil }
	endpoint.RuntimeVersion = "not-fixture-version"
	if p, err := NewHTTPProvider(context.Background(), endpoint); err == nil || p != nil {
		t.Fatal("wrong runtime version succeeded")
	}
	if err := os.WriteFile(filepath.Join(endpoint.ManagedOllama.RuntimeDirectory, "fixture.exe"), []byte("changed"), 0600); err != nil {
		t.Fatal("failed readiness retained process/file lease", err)
	}
}

func TestManagedOllamaRefusesConcurrentStateOwnership(t *testing.T) {
	endpoint := managedOllamaFixture(t)
	p, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	if second, err := NewHTTPProvider(context.Background(), endpoint); err == nil || second != nil {
		t.Fatal("second runtime acquired same private state")
	}
	if err := p.Health(context.Background()); err != nil {
		t.Fatal("failed second owner damaged existing runtime", err)
	}
}

func TestManagedOllamaBlockedCallbackStillRetiresOwnedTreeOnCancellation(t *testing.T) {
	testManagedOllamaBlockedCallbackRetirement(t, false)
}

func TestManagedOllamaBlockedCallbackStillRetiresOwnedTreeOnPolicyRevocation(t *testing.T) {
	testManagedOllamaBlockedCallbackRetirement(t, true)
}

func testManagedOllamaBlockedCallbackRetirement(t *testing.T, policyRevoked bool) {
	endpoint := managedOllamaFixture(t)
	policyContext, revoke := context.WithCancel(context.Background())
	defer revoke()
	if policyRevoked {
		endpoint.AcquireRequest = func(ctx context.Context) (context.Context, func(), error) {
			bound, cancel := context.WithCancel(ctx)
			stop := context.AfterFunc(policyContext, cancel)
			return bound, func() { stop(); cancel() }, nil
		}
	}
	p, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if policyRevoked {
		cancel = revoke
	}
	entered, unblock := make(chan struct{}), make(chan struct{})
	var once sync.Once
	cleanup := func() { once.Do(func() { close(unblock) }) }
	t.Cleanup(func() { cleanup(); p.Close() })
	result := make(chan error, 1)
	go func() {
		result <- p.Generate(ctx, httpProviderRequest(t, endpoint, nil), func(event ProviderEvent) error {
			if event.Type == ProviderEventDelta {
				close(entered)
				<-unblock
			}
			return nil
		})
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("fixture callback did not block")
	}
	child := managedFixtureProcess(t, endpoint.ManagedOllama.StateDirectory, "child")
	cancel()
	// Cancellation's ownership watcher must attempt retirement even though the
	// call cannot yet settle. Successful process exit is not a successful call.
	if status, err := windows.WaitForSingleObject(child, 13000); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("blocked consumer kept owned descendant alive", status, err)
	}
	cleanup()
	select {
	case err := <-result:
		if !errors.Is(err, errProviderStopUncertain) {
			t.Fatal("blocked callback retirement uncertainty was erased", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("generation did not settle after consumer released")
	}
	if err := p.Close(); !errors.Is(err, errProviderStopUncertain) {
		t.Fatal("repeat close hid callback uncertainty", err)
	}
}
