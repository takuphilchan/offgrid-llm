package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
)

func isolatedPolicyFixture() IsolationPolicy {
	return IsolationPolicy{Socket: "/var/run/docker.sock", Image: "sha256:" + strings.Repeat("a", 64), Network: "none", MemoryBytes: 512 << 20, CPUMillis: 1000, Pids: 64, UID: 1000, GID: 1000}
}

func TestIsolationConfigurationHasOnlyExplicitMountAndNetwork(t *testing.T) {
	p := isolatedPolicyFixture()
	p.Socket = "/run/fixture/docker.sock"
	root := t.TempDir()
	config, err := p.containerConfig(root, "subdir", strings.Repeat("b", 64), false, ProcessSpec{Program: "sh", Arguments: []string{"-c", "printf 'exact'"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(config.HostConfig.Mounts) != 1 || config.HostConfig.Mounts[0].Source != root || config.HostConfig.Mounts[0].Target != "/workspace" || !config.HostConfig.Mounts[0].ReadOnly || !config.HostConfig.Mounts[0].BindOptions.NonRecursive || config.WorkingDir != "/workspace/subdir" || !config.NetworkDisabled || config.HostConfig.NetworkMode != "none" || !config.HostConfig.ReadonlyRootfs || config.User != "1000:1000" || config.HostConfig.Memory != p.MemoryBytes || config.HostConfig.LogConfig.Type != "none" {
		t.Fatal(config)
	}
	encoded, _ := json.Marshal(config)
	for _, secret := range []string{p.Socket, "OFFGRID_EXECUTOR_TOKEN", "OFFGRID_API_KEY", "grant.json", "execution-dispatch.sqlite"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("private host surface forwarded", secret)
		}
	}
	for _, network := range []string{"host", "container:other", "service_default", ""} {
		bad := p
		bad.Network = network
		if _, err := bad.containerConfig(root, ".", strings.Repeat("b", 64), true, ProcessSpec{Program: "go"}); err == nil {
			t.Fatal("unapproved network", network)
		}
	}
	for _, image := range []string{"alpine:latest", "docker.io/alpine", "https://remote/model", "sha256:short"} {
		bad := p
		bad.Image = image
		if bad.Validate() == nil {
			t.Fatal("unpinned image", image)
		}
	}
	for _, directory := range []string{"..", "../escape", root, "C:/foreign", `\escape`, "bad\x00directory"} {
		if _, err := p.containerConfig(root, directory, strings.Repeat("b", 64), true, ProcessSpec{Program: "go"}); err == nil {
			t.Fatal("foreign cwd", directory)
		}
	}
}

func TestIsolationPreflightRejectsUnavailableOrExtraAuthority(t *testing.T) {
	for _, kind := range []string{"ready", "missing-image", "wrong-image", "implicit-volume", "wrong-platform", "no-limits", "redirect"} {
		t.Run(kind, func(t *testing.T) {
			p := isolatedPolicyFixture()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.Header.Get("Authorization") != "" {
					t.Error("non-observational/preauthenticated request")
				}
				switch r.URL.Path {
				case "/v1.47/info":
					if kind == "redirect" {
						w.Header().Set("Location", "http://outside.invalid")
						w.WriteHeader(302)
						return
					}
					fmt.Fprintf(w, `{"ID":"fixture-daemon","OSType":"linux","Architecture":%q,"MemoryLimit":%t,"CPUCfsQuota":true,"PidsLimit":true}`, runtime.GOARCH, kind != "no-limits")
				case "/v1.47/images/" + p.Image + "/json":
					if kind == "missing-image" {
						w.WriteHeader(404)
						return
					}
					id, platform, volumes := p.Image, "linux", "null"
					if kind == "wrong-image" {
						id = "sha256:" + strings.Repeat("c", 64)
					}
					if kind == "wrong-platform" {
						platform = "windows"
					}
					if kind == "implicit-volume" {
						volumes = `{"/private":{}}`
					}
					fmt.Fprintf(w, `{"Id":%q,"Os":%q,"Architecture":%q,"Config":{"Volumes":%s}}`, id, platform, runtime.GOARCH, volumes)
				default:
					t.Error("unreviewed API", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			client.Transport = isolationFixtureTransport{next: client.Transport, origin: server.URL}
			i := &LinuxIsolation{policy: p, root: t.TempDir(), client: client}
			err := i.preflight(context.Background())
			if (err == nil) != (kind == "ready") {
				t.Fatal(kind, err)
			}
		})
	}
}

type isolationFixtureTransport struct {
	next   http.RoundTripper
	origin string
}

func (t isolationFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	newRequest, _ := http.NewRequestWithContext(r.Context(), r.Method, t.origin+r.URL.RequestURI(), nil)
	copy.URL = newRequest.URL
	return t.next.RoundTrip(copy)
}

func TestRealLocalIsolationPreflightWithoutRunningContainer(t *testing.T) {
	image := os.Getenv("OFFGRID_TEST_ISOLATION_IMAGE")
	if image == "" {
		t.Skip("explicit installed image required; fixture checks are not real Docker qualification")
	}
	if runtime.GOOS != "linux" {
		t.Skip("native isolated Linux host required")
	}
	p := isolatedPolicyFixture()
	p.Image = image
	if socket := os.Getenv("OFFGRID_TEST_ISOLATION_SOCKET"); socket != "" {
		p.Socket = socket
	}
	i, err := NewLinuxIsolation(context.Background(), p, t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer i.client.CloseIdleConnections()
	if i.daemonID == "" {
		t.Fatal("missing daemon identity")
	}
}
