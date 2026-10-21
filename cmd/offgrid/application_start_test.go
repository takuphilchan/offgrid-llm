package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/app"
	"github.com/takuphilchan/offgrid-llm/internal/config"
)

func TestDefaultAndExplicitServeUseOneApplicationAdapterWithoutUserState(t *testing.T) {
	previousLoad, previousStart := loadServeConfig, startApplication
	t.Cleanup(func() { loadServeConfig, startApplication = previousLoad, previousStart })
	configPath := filepath.Join(t.TempDir(), "fixture-config.json")
	t.Setenv("OFFGRID_CONFIG", configPath)
	t.Setenv("OFFGRID_VERSION", "fixture-version")
	cfg := &config.Config{ModelsDir: t.TempDir(), DataDir: t.TempDir(), ServerHost: "127.0.0.1"}
	loads, starts := 0, 0
	loadServeConfig = func(path string) (*config.Config, error) {
		if path != configPath {
			t.Fatalf("config path = %q", path)
		}
		loads++
		return cfg, nil
	}
	startApplication = func(_ context.Context, selected *config.Config) error {
		if selected != cfg {
			t.Fatal("different server configuration")
		}
		starts++
		return nil
	}
	for _, args := range [][]string{{"offgrid"}, {"offgrid", "serve"}} {
		if code := runRoot(context.Background(), args, osCLIEnvironment()); code != 0 {
			t.Fatalf("serve args=%#v code=%d", args, code)
		}
	}
	if loads != 2 || starts != 2 {
		t.Fatalf("config loads=%d server starts=%d", loads, starts)
	}
	want := errors.New("fixture startup failure")
	startApplication = func(context.Context, *config.Config) error { return want }
	if code := runRoot(context.Background(), []string{"offgrid", "serve"}, osCLIEnvironment()); code != 1 {
		t.Fatalf("failed startup exit code = %d", code)
	}
}

func TestDefaultServeStartsAndShutsDownWithIsolatedState(t *testing.T) {
	previousLoad, previousStart := loadServeConfig, startApplication
	t.Cleanup(func() { loadServeConfig, startApplication = previousLoad, previousStart })
	root := t.TempDir()
	t.Setenv("OFFGRID_CONFIG", filepath.Join(root, "config.json"))
	t.Setenv("OFFGRID_VERSION", "fixture-version")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	cfg := config.LoadConfig()
	cfg.ModelsDir = filepath.Join(root, "models")
	cfg.DataDir = filepath.Join(root, "data")
	cfg.ServerHost = "127.0.0.1"
	cfg.ServerPort = port
	cfg.UseMockEngine = true
	cfg.EnableP2P = false
	cfg.PrewarmModels = false
	loadServeConfig = func(string) (*config.Config, error) { return cfg, nil }
	started := make(chan *app.Runtime, 1)
	startApplication = func(ctx context.Context, configured *config.Config) error {
		instance, err := app.New(configured)
		if err != nil {
			return err
		}
		started <- instance
		return instance.Run(ctx)
	}
	done := make(chan int, 1)
	go func() { done <- runRoot(context.Background(), []string{"offgrid"}, osCLIEnvironment()) }()
	var instance *app.Runtime
	select {
	case instance = <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("isolated server was not constructed")
	}
	t.Cleanup(func() { _ = instance.Close() })
	client := &http.Client{Timeout: 250 * time.Millisecond, Transport: &http.Transport{Proxy: nil}}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(10 * time.Second)
	for {
		response, requestErr := client.Get(url)
		if requestErr == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case code := <-done:
			t.Fatalf("isolated server exited before health check: code=%d", code)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("isolated server did not become healthy")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := instance.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("isolated server shutdown code=%d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("isolated server did not stop")
	}
}
