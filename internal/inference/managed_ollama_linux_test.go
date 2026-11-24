package inference

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// This success-path qualification requires explicit external administrator
// provisioning. Ordinary tests never elevate, chown or change installed state.
func TestManagedOllamaLinuxProvisionedFactorySchedulerAndCancellation(t *testing.T) {
	path := os.Getenv("OFFGRID_MANAGED_LINUX_FIXTURE")
	if path == "" {
		t.Skip("requires explicitly administrator-provisioned isolated fixture; see provider-linux-namespace evidence")
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 65536 {
		t.Fatal("invalid fixture record", err)
	}
	var record struct {
		Config *ManagedOllamaConfig `json:"managed_ollama"`
		Digest string               `json:"model_digest"`
	}
	if err := json.Unmarshal(raw, &record); err != nil || record.Config == nil {
		t.Fatal("invalid fixture record", err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	endpoint := HTTPEndpoint{URL: "http://" + address, Model: "fixture:tag", RuntimeVersion: "fixture-1", AllowedIPs: []string{"127.0.0.1"}, ManagedOllama: record.Config, Authorize: func(context.Context) (string, error) { return "", nil }, Descriptor: ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderOllama, RunnerRevision: "fixture-1", AdapterRevision: HTTPEndpointAdapterRevision, ModelDigest: record.Digest, Format: FormatGGUF, Backend: "local-ollama", ContextLength: 8192, Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}}
	socket, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: filepath.Join(record.Config.StateDirectory, "observer.sock"), Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	defer os.Remove(socket.LocalAddr().String())
	control, err := socket.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var optionErr error
	if err := control.Control(func(fd uintptr) { optionErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_PASSCRED, 1) }); err != nil || optionErr != nil {
		t.Fatal(err, optionErr)
	}
	routes, err := NewHTTPEndpointRoutes(map[string]HTTPEndpoint{"owned@r1": endpoint})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewProviderRegistry()
	if err := routes.Register(registry); err != nil {
		t.Fatal(err)
	}
	descriptor, err := routes.Select(context.Background(), "owned", "r1")
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewProviderScheduler(registry, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer scheduler.Close()
	var content string
	if err := scheduler.Generate(context.Background(), descriptor, httpProviderRequest(t, endpoint, nil), func(event ProviderEvent) error { content += event.Text; return nil }); err != nil {
		t.Fatal(err)
	}
	if content != "owned fixture" {
		t.Fatal(content)
	}
	children := namespaceFixtureChildren(t, socket)
	release, _, err := scheduler.AcquireLegacyMaintenance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	assertNamespaceChildrenExited(t, children)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httpProviderRequest(t, endpoint, nil)
	request.Prompt = json.RawMessage(strings.Replace(string(request.Prompt), "hello", "block", 1))
	done := make(chan error, 1)
	go func() { done <- scheduler.Generate(ctx, descriptor, request, func(ProviderEvent) error { return nil }) }()
	children = namespaceFixtureChildren(t, socket)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled generation succeeded")
		}
	case <-time.After(12 * time.Second):
		t.Fatal("cancel did not settle")
	}
	assertNamespaceChildrenExited(t, children)
}
