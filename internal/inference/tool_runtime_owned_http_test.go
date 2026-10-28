//go:build !llama

package inference

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"testing"
)

func ownedToolRuntimeFixture(t *testing.T, handler http.HandlerFunc) (*LlamaCPPProcess, *ModelCache) {
	t.Helper()
	endpoint := httptest.NewServer(handler)
	t.Cleanup(endpoint.Close)
	_, portString, err := net.SplitHostPort(endpoint.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	// Observe our own test process; never spawn or control a desktop app.
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.Release() })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cache := &ModelCache{instances: map[string]*ModelInstance{"selected": {ModelID: "selected", Generation: 7, ContextSize: 8192, ModelPath: "fixture.gguf", Port: port, Cmd: &exec.Cmd{Path: executable, Process: process, Args: []string{executable, "-m", "fixture.gguf", "--port", portString, "-c", "8192"}, Env: []string{}}}}}
	return &LlamaCPPProcess{cache: cache}, cache
}

func TestOwnedToolRuntimeMeasuresOnlySelectedProcess(t *testing.T) {
	var requests atomic.Int32
	p, _ := ownedToolRuntimeFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/props" {
			t.Errorf("unexpected route %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"build_info":"fixture","chat_template":"exact template","default_generation_settings":{"n_ctx":8192}}`)
	})
	got, err := p.ObserveToolRuntime(context.Background(), "selected", 8192)
	if err != nil || got.ModelID != "selected" || got.Generation != 7 || got.Info.Build != "fixture" || got.Info.Context != 8192 || len(got.Info.TemplateSHA256) != 64 || len(got.ConfigSHA256) != 64 {
		t.Fatal(got, err)
	}
	for _, tc := range []struct {
		model  string
		window int
	}{{"other", 8192}, {"selected", 16384}, {"", 8192}, {"selected", 0}} {
		if _, err := p.ObserveToolRuntime(context.Background(), tc.model, tc.window); err == nil {
			t.Fatal("invalid selection observed", tc)
		}
	}
	if requests.Load() != 1 {
		t.Fatal("invalid selection queried an endpoint", requests.Load())
	}
}

func TestOwnedToolRuntimeDoesNotFollowAnotherEndpoint(t *testing.T) {
	var followed atomic.Bool
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed.Store(true) }))
	defer other.Close()
	p, _ := ownedToolRuntimeFixture(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	})
	if _, err := p.ObserveToolRuntime(context.Background(), "selected", 8192); err == nil || followed.Load() {
		t.Fatal("owned identity followed a foreign runtime")
	}
}

func TestOwnedToolRuntimeRejectsRestartWithoutBlockingCache(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	p, cache := ownedToolRuntimeFixture(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		fmt.Fprint(w, `{"build_info":"fixture","chat_template":"exact template","default_generation_settings":{"n_ctx":8192}}`)
	})
	result := make(chan error, 1)
	go func() { _, err := p.ObserveToolRuntime(context.Background(), "selected", 8192); result <- err }()
	<-entered
	cache.mu.Lock() // HTTP must not retain this mutex and obstruct Stop/restart.
	replacement := *cache.instances["selected"]
	replacement.Generation++
	cache.instances["selected"] = &replacement
	cache.mu.Unlock()
	close(release)
	if err := <-result; err == nil {
		t.Fatal("restarted child retained observation")
	}
}

func TestOwnedToolRuntimeRejectsIncompleteIdentityAndCancellation(t *testing.T) {
	for _, props := range []string{
		`{"default_generation_settings":{"n_ctx":8192}}`,
		`{"build_info":"fixture","chat_template":"template","default_generation_settings":{"n_ctx":4096}}`,
	} {
		p, _ := ownedToolRuntimeFixture(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, props) })
		if _, err := p.ObserveToolRuntime(context.Background(), "selected", 8192); err == nil {
			t.Fatal("incomplete observation accepted")
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := p.ObserveToolRuntime(ctx, "selected", 8192); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation lost", err)
		}
	}
}
