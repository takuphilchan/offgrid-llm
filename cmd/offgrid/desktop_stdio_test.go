package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/config"
	"github.com/takuphilchan/offgrid-llm/internal/storage"
)

func TestDesktopOwnedEOFWaitsForExistingDrain(t *testing.T) {
	for _, failed := range []bool{false, true} {
		input, parent := io.Pipe()
		cancelled, drained, done := make(chan struct{}), make(chan struct{}), make(chan int, 1)
		go func() {
			done <- serveDesktopOwned(context.Background(), input, func(ctx context.Context) int {
				<-ctx.Done()
				close(cancelled)
				<-drained
				if failed {
					return 1
				}
				return 0
			})
		}()
		parent.Close()
		<-cancelled
		select {
		case <-done:
			t.Fatal("EOF acknowledged before application drain")
		default:
		}
		close(drained)
		if code := <-done; (code != 0) != failed {
			t.Fatal("drain result changed", code, failed)
		}
	}
	input, parent := io.Pipe()
	done := make(chan int, 1)
	go func() {
		done <- serveDesktopOwned(context.Background(), input, func(ctx context.Context) int { <-ctx.Done(); return 0 })
	}()
	parent.Write([]byte("unexpected"))
	parent.Close()
	if code := <-done; code != 2 {
		t.Fatal("unexpected lifecycle control accepted", code)
	}
}

func TestDesktopOwnedServiceProcessHelper(t *testing.T) {
	root := os.Getenv("OFFGRID_OWNED_SERVICE_TEST")
	if root == "" {
		return
	}
	if marker, err := os.ReadFile(filepath.Join(root, "isolated-test.marker")); err != nil || string(marker) != "desktop-owned-service-test" {
		t.Fatal("isolated root required")
	}
	port, err := strconv.Atoi(os.Getenv("OFFGRID_OWNED_SERVICE_PORT"))
	if err != nil || port <= 0 || port > 65535 {
		t.Fatal("isolated port required")
	}
	cfg := config.LoadConfig()
	cfg.ModelsDir, cfg.DataDir = filepath.Join(root, "models"), filepath.Join(root, "data")
	cfg.ServerHost, cfg.ServerPort, cfg.UseMockEngine = "127.0.0.1", port, true
	cfg.EnableP2P, cfg.PrewarmModels = false, false
	loadServeConfig = func(string) (*config.Config, error) { return cfg, nil }
	Version = "desktop-test-v1"
	os.Exit(runRoot(context.Background(), []string{"offgrid", "serve", "--desktop-owned-stdio"}, osCLIEnvironment()))
}

func TestDesktopOwnedServiceActualEOFReleasesWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "isolated-test.marker"), []byte("desktop-owned-service-test"), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDesktopOwnedServiceProcessHelper$")
	command.Env = append(os.Environ(), "OFFGRID_OWNED_SERVICE_TEST="+root, fmt.Sprintf("OFFGRID_OWNED_SERVICE_PORT=%d", port))
	command.Stdout, command.Stderr = io.Discard, io.Discard
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if command.ProcessState == nil {
			command.Process.Kill()
			command.Wait()
		}
	})
	client := &http.Client{Timeout: 250 * time.Millisecond, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(15 * time.Second)
	for {
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("isolated owned service did not become ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if owner, err := storage.AcquireOwnership(filepath.Join(root, "data")); err == nil {
		owner.Close()
		t.Fatal("running service did not own its workspace")
	}
	input.Close()
	if err := command.Wait(); err != nil {
		t.Fatal("private EOF did not acknowledge completed service drain", err)
	}
	owner, err := storage.AcquireOwnership(filepath.Join(root, "data"))
	if err != nil {
		t.Fatal("workspace ownership retained after successful drain", err)
	}
	owner.Close()
}
