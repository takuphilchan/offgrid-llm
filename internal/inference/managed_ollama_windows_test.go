package inference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

var managedFixtureBinary struct {
	sync.Once
	data []byte
	err  error
}

func managedOllamaFixture(t *testing.T) HTTPEndpoint {
	t.Helper()
	managedFixtureBinary.Do(func() {
		directory, err := os.MkdirTemp("", "offgrid-ollama-fixture-build-")
		if err != nil {
			managedFixtureBinary.err = err
			return
		}
		defer os.RemoveAll(directory)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		path := filepath.Join(directory, "fixture.exe")
		cmd := exec.CommandContext(ctx, "go", "build", "-o", path, "./testdata/managedollama")
		if output, err := cmd.CombinedOutput(); err != nil {
			managedFixtureBinary.err = fmt.Errorf("build isolated Ollama fixture: %w: %s", err, output)
			return
		}
		managedFixtureBinary.data, managedFixtureBinary.err = os.ReadFile(path)
	})
	if managedFixtureBinary.err != nil {
		t.Fatal(managedFixtureBinary.err)
	}
	root := t.TempDir()
	privateFixtureDirectory(t, root)
	runtimeDir, modelDir, stateDir := filepath.Join(root, "runtime"), filepath.Join(root, "models"), filepath.Join(root, "state")
	for _, path := range []string{runtimeDir, modelDir, stateDir, filepath.Join(modelDir, "blobs"), filepath.Join(modelDir, "manifests", "registry.ollama.ai", "library", "fixture")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(runtimeDir, "fixture.exe")
	if err := os.WriteFile(executable, managedFixtureBinary.data, 0600); err != nil {
		t.Fatal(err)
	}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return "sha256:" + hex.EncodeToString(sum[:]) }
	config, weights := []byte(`{"model_format":"gguf"}`), []byte("isolated fixture, not model weights")
	for _, data := range [][]byte{config, weights} {
		if err := os.WriteFile(filepath.Join(modelDir, "blobs", strings.Replace(digest(data), ":", "-", 1)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest, _ := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": "application/vnd.docker.distribution.manifest.v2+json", "config": managedOllamaLayer{MediaType: "application/vnd.docker.container.image.v1+json", Digest: digest(config), Size: int64(len(config))}, "layers": []managedOllamaLayer{{MediaType: "application/vnd.ollama.image.model", Digest: digest(weights), Size: int64(len(weights))}}})
	if err := os.WriteFile(filepath.Join(modelDir, "manifests", "registry.ollama.ai", "library", "fixture", "tag"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	return HTTPEndpoint{URL: "http://" + address, Model: "fixture:tag", RuntimeVersion: "fixture-1", AllowedIPs: []string{"127.0.0.1"}, Authorize: func(context.Context) (string, error) { return "", nil }, Descriptor: ProviderDescriptor{ProtocolRevision: ProviderProtocolRevision, Provider: ProviderOllama, RunnerRevision: "fixture-1", AdapterRevision: HTTPEndpointAdapterRevision, ModelDigest: digest(manifest), Format: FormatGGUF, Backend: "local-ollama", ContextLength: 8192, Capabilities: []ProviderCapability{CapabilityText, CapabilityStreaming}}, ManagedOllama: &ManagedOllamaConfig{RuntimeDirectory: runtimeDir, Executable: "fixture.exe", RuntimeFiles: map[string]string{"fixture.exe": digest(managedFixtureBinary.data)}, ModelDirectory: modelDir, StateDirectory: stateDir, TrustedSingleUserHost: true, AllowUnauthenticatedLoopback: true}}
}

func privateFixtureDirectory(t *testing.T, path string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

func managedFixtureProcess(t *testing.T, state, name string) windows.Handle {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(state, name+".pid"))
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { windows.TerminateProcess(handle, 1); windows.CloseHandle(handle) })
			return handle
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("isolated managed process did not start")
	return 0
}

func TestManagedOllamaOwnsListenerFilesAndDescendantRelease(t *testing.T) {
	t.Setenv("OFFGRID_OLLAMA_SHOULD_NOT_INHERIT", "private-value")
	endpoint := managedOllamaFixture(t)
	provider, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { provider.Close() })
	root := managedFixtureProcess(t, endpoint.ManagedOllama.StateDirectory, "root")
	var content string
	if err := provider.Generate(context.Background(), httpProviderRequest(t, endpoint, nil), func(event ProviderEvent) error { content += event.Text; return nil }); err != nil {
		t.Fatal(err)
	}
	if content != "owned fixture" {
		t.Fatal(content)
	}
	child := managedFixtureProcess(t, endpoint.ManagedOllama.StateDirectory, "child")
	if err := os.WriteFile(filepath.Join(endpoint.ManagedOllama.RuntimeDirectory, "fixture.exe"), []byte("changed"), 0600); err == nil {
		t.Fatal("active runtime file lease allowed mutation")
	}
	if err := provider.Close(); err != nil {
		t.Fatal(err)
	}
	for _, handle := range []windows.Handle{root, child} {
		if status, err := windows.WaitForSingleObject(handle, 0); err != nil || status != windows.WAIT_OBJECT_0 {
			t.Fatal("managed Ollama released a surviving process", status, err)
		}
	}
	if err := os.WriteFile(filepath.Join(endpoint.ManagedOllama.RuntimeDirectory, "fixture.exe"), []byte("changed"), 0600); err != nil {
		t.Fatal("clean shutdown retained file lease", err)
	}
}

func TestManagedOllamaRejectsOccupiedUnownedListenerBeforeHTTP(t *testing.T) {
	endpoint := managedOllamaFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"version":"fixture-1"}`)
	}))
	defer server.Close()
	endpoint.URL = server.URL
	if provider, err := NewHTTPProvider(context.Background(), endpoint); err == nil || provider != nil {
		t.Fatal("unowned listener admitted")
	}
	if requests.Load() != 0 {
		t.Fatal("HTTP was sent to unowned listener")
	}
}

func TestManagedOllamaRejectsPinsInventoryAndConsentBeforeLaunch(t *testing.T) {
	for _, change := range []string{"pin", "extra", "manifest", "consent", "cloud"} {
		t.Run(change, func(t *testing.T) {
			endpoint := managedOllamaFixture(t)
			switch change {
			case "pin":
				endpoint.ManagedOllama.RuntimeFiles["fixture.exe"] = "sha256:" + strings.Repeat("a", 64)
			case "extra":
				os.WriteFile(filepath.Join(endpoint.ManagedOllama.RuntimeDirectory, "unreviewed.dll"), []byte("unreviewed"), 0600)
			case "manifest":
				endpoint.Descriptor.ModelDigest = "sha256:" + strings.Repeat("b", 64)
			case "consent":
				endpoint.ManagedOllama.AllowUnauthenticatedLoopback = false
			case "cloud":
				endpoint.Model = "fixture:cloud" // no matching offline manifest
			}
			if p, err := NewHTTPProvider(context.Background(), endpoint); err == nil || p != nil {
				t.Fatal("invalid managed runtime launched")
			}
			if _, err := os.Stat(filepath.Join(endpoint.ManagedOllama.StateDirectory, "root.pid")); !os.IsNotExist(err) {
				t.Fatal("rejected runtime executed")
			}
		})
	}
}

func TestManagedOllamaStopAwaitsDescendantExit(t *testing.T) {
	endpoint := managedOllamaFixture(t)
	p, err := NewHTTPProvider(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	request := httpProviderRequest(t, endpoint, nil)
	request.Prompt = []byte(strings.Replace(string(request.Prompt), "hello", "block", 1))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- p.Generate(ctx, request, func(ProviderEvent) error { return nil }) }()
	child := managedFixtureProcess(t, endpoint.ManagedOllama.StateDirectory, "child")
	cancel()
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled generation succeeded")
		}
	case <-time.After(12 * time.Second):
		t.Fatal("managed cancellation did not settle")
	}
	if status, err := windows.WaitForSingleObject(child, 0); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatal("cancelled provider returned before descendant exit", status, err)
	}
}
