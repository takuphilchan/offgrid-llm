package inference

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestModelStartupNeverEvictsAnUnrelatedPortOwner(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	dir := t.TempDir()
	model := filepath.Join(dir, "fixture.gguf")
	if err := os.WriteFile(model, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	cache := NewModelCache(1, 0, dir)
	cache.basePort = listener.Addr().(*net.TCPAddr).Port
	_, err = cache.GetOrLoadContext(context.Background(), "fixture", model, "")
	if err == nil || !strings.Contains(err.Error(), "already occupied") {
		t.Fatalf("wrong failure: %v", err)
	}
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("port owner was disturbed: %v", err)
	}
	conn.Close()
	if len(cache.instances) != 0 || len(cache.pendingLoads) != 0 {
		t.Fatal("failed startup retained runtime state")
	}
}
