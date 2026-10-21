package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewComposesTransportAndReleasesWorkspace(t *testing.T) {
	cfg := disposableCoreConfig(t)
	first, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owners := make([]string, 0, len(first.owned.steps))
	for _, step := range first.owned.steps {
		owners = append(owners, step.name)
	}
	if len(owners) < 3 || owners[len(owners)-1] != "runtime context" || owners[len(owners)-2] != "provider scheduler" {
		t.Fatalf("application did not stop task context and provider admission first: %v", owners)
	}
	cacheHasIndependentOwner := false
	for _, owner := range owners[:len(owners)-2] {
		if owner == "model cache" {
			cacheHasIndependentOwner = true
		}
	}
	if cacheHasIndependentOwner {
		t.Fatalf("model cache shutdown bypasses provider ownership: %v", owners)
	}
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	first.transport.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("composed health status = %d: %s", response.Code, response.Body.String())
	}
	if _, err := New(cfg); err == nil {
		t.Fatal("another application acquired the same workspace")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := New(cfg)
	if err != nil {
		t.Fatalf("application workspace was not released: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewComposesRealLlamaHTTPWithoutModel(t *testing.T) {
	cfg := disposableCoreConfig(t)
	cfg.UseMockEngine = false
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("real llama HTTP mode must start before a model is selected: %v", err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRunCancelDrainsAndReleasesWorkspace(t *testing.T) {
	cfg := disposableCoreConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.ServerHost = "127.0.0.1"
	cfg.ServerPort = listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	runtime, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()
	client := &http.Client{Timeout: 200 * time.Millisecond, Transport: &http.Transport{Proxy: nil}}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", cfg.ServerPort)
	deadline := time.Now().Add(8 * time.Second)
	for {
		response, requestErr := client.Get(url)
		if requestErr == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case runErr := <-done:
			t.Fatalf("application exited before readiness: %v", runErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("application did not become ready")
		}
		time.Sleep(25 * time.Millisecond)
	}
	cancel()
	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("cancelled run: %v", runErr)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("application did not stop")
	}
	again, err := New(cfg)
	if err != nil {
		t.Fatalf("cancel did not release workspace: %v", err)
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRunBindFailureReleasesWorkspace(t *testing.T) {
	cfg := disposableCoreConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	cfg.ServerHost = "127.0.0.1"
	cfg.ServerPort = listener.Addr().(*net.TCPAddr).Port
	runtime, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Run(context.Background()); err == nil {
		t.Fatal("occupied port did not fail")
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := New(cfg)
	if err != nil {
		t.Fatalf("failed bind retained workspace ownership: %v", err)
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNewRollbackAfterTransportRecoveryFailure(t *testing.T) {
	cfg := disposableCoreConfig(t)
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfg.DataDir, "downloads.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(cfg); err == nil {
		t.Fatal("corrupt recovery state accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	runtime, err := New(cfg)
	if err != nil {
		t.Fatalf("failed startup retained workspace ownership: %v", err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
}
