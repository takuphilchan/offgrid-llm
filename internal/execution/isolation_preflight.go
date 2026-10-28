package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
)

// LinuxIsolation is a native-supervisor endpoint. No service process receives
// its Docker socket, and container options are not available as agent arguments.
type LinuxIsolation struct {
	policy   IsolationPolicy
	root     string
	write    bool
	daemonID string
	client   *http.Client
}

func NewLinuxIsolation(ctx context.Context, policy IsolationPolicy, root string, write bool) (*LinuxIsolation, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if err := inspectIsolationRoot(root); err != nil {
		return nil, err
	}
	client, err := localIsolationClient(policy.Socket)
	if err != nil {
		return nil, err
	}
	i := &LinuxIsolation{policy: policy, root: root, write: write, client: client}
	if err = i.preflight(ctx); err != nil {
		client.CloseIdleConnections()
		return nil, err
	}
	return i, nil
}

func (i *LinuxIsolation) get(ctx context.Context, path string, target any) error {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker.local/v1.47"+path, nil)
	if err != nil {
		return err
	}
	response, err := i.client.Do(r)
	if err != nil {
		return ErrHostUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("isolated runtime is not locally ready: %w", ErrUnsupported)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxFrameBytes+1))
	// Docker's forward-compatible objects contain fields OffGrid need not
	// interpret. Duplicate/malformed JSON is still rejected by decodeStrict
	// into a generic map before projecting the explicitly needed subset.
	var object map[string]json.RawMessage
	if err != nil || len(data) > MaxFrameBytes || decodeStrict(data, &object) != nil || json.Unmarshal(data, target) != nil {
		return ErrProtocol
	}
	return nil
}

func (i *LinuxIsolation) preflight(ctx context.Context) error {
	var info struct {
		ID, OSType, Architecture            string
		MemoryLimit, CPUCfsQuota, PidsLimit bool
	}
	if err := i.get(ctx, "/info", &info); err != nil {
		return err
	}
	if !validIdentity(info.ID) || info.OSType != "linux" || !info.MemoryLimit || !info.CPUCfsQuota || !info.PidsLimit {
		return ErrUnsupported
	}
	var image struct {
		ID, OS, Architecture string
		Config               struct{ Volumes map[string]json.RawMessage }
	}
	if err := i.get(ctx, "/images/"+i.policy.Image+"/json", &image); err != nil {
		return err
	}
	if image.ID != i.policy.Image || image.OS != "linux" || image.Architecture != runtime.GOARCH || len(image.Config.Volumes) != 0 {
		return ErrUnsupported
	}
	arch := strings.ToLower(info.Architecture)
	if arch == "x86_64" {
		arch = "amd64"
	}
	if arch == "aarch64" {
		arch = "arm64"
	}
	if arch != image.Architecture {
		return ErrUnsupported
	}
	i.daemonID = info.ID
	return nil
}
