package inference

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Exercise native process/port ownership without downloads or real inference.
func TestMain(m *testing.M) {
	if os.Getenv("OFFGRID_TEST_PORT_RUNTIME") == "1" {
		port := ""
		for i, arg := range os.Args {
			if arg == "--port" && i+1 < len(os.Args) {
				port = os.Args[i+1]
			}
		}
		if port == "" {
			os.Exit(2)
		}
		err := http.ListenAndServe("127.0.0.1:"+port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"status":"ok","data":[]}`)
		}))
		if err != nil {
			os.Exit(3)
		}
		return
	}
	os.Exit(m.Run())
}

func TestModelStartupNeverEvictsAnUnrelatedPortOwner(t *testing.T) {
	// The old fixed port may already host a user's runtime. In either case
	// loading must select a different port without touching the old listener.
	listener, err := net.Listen("tcp4", "127.0.0.1:42382")
	if err == nil {
		defer listener.Close()
	}
	dir := t.TempDir()
	model := filepath.Join(dir, "fixture.gguf")
	if err := os.WriteFile(model, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", binary)
	t.Setenv("OFFGRID_TEST_PORT_RUNTIME", "1")
	for _, capacity := range []int{1, 2} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			cache := NewModelCache(capacity, 0, dir)
			defer cache.StopMonitor()
			defer cache.UnloadAll()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			first, err := cache.GetOrLoadContext(ctx, "fixture", model, "")
			if err != nil {
				t.Fatal(err)
			}
			if first.Port == 42382 {
				t.Fatal("reused occupied port")
			}
			again, err := cache.GetOrLoadContext(ctx, "fixture", model, "")
			if err != nil || again != first {
				t.Fatalf("live runtime not reused: %v", err)
			}
			second, err := cache.GetOrLoadContext(ctx, "other", model, "")
			if err != nil {
				t.Fatal(err)
			}
			if capacity == 2 && second.Port == first.Port {
				t.Fatal("models share a port")
			}
			conn, err := net.DialTimeout("tcp4", "127.0.0.1:42382", time.Second)
			if err != nil {
				t.Fatalf("existing listener disturbed: %v", err)
			}
			conn.Close()
		})
	}
}

func TestFailedRuntimeStartReleasesTracking(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(model, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFFGRID_LLAMA_SERVER_PATH", filepath.Join(dir, "missing"))
	cache := NewModelCache(1, 0, dir)
	defer cache.StopMonitor()
	for range 2 {
		if _, err := cache.GetOrLoadContext(context.Background(), "fixture", model, ""); err == nil {
			t.Fatal("accepted missing runtime")
		}
		if len(cache.instances)+len(cache.usedPorts)+len(cache.portToModel)+len(cache.pendingLoads) != 0 {
			t.Fatal("failed startup leaked tracking")
		}
	}
}
