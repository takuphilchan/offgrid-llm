package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
)

const HTTPEndpointAdapterRevision = "http-provider-v1"

// HTTPEndpoint is installation-owned configuration. Authorize must recheck
// actor access and the entire current enrollment, including deployment pins.
// Credentials are obtained for each request and are never part of descriptors.
type HTTPEndpoint struct {
	URL, Model, RuntimeVersion string
	AllowedIPs                 []string
	Remote                     bool
	Descriptor                 ProviderDescriptor
	Authorize                  func(context.Context) (string, error)
	// AcquireRequest binds each HTTP request to current service-owned policy.
	// Revocation can cancel admitted transports without claiming remote undo.
	AcquireRequest func(context.Context) (context.Context, func(), error)
	// Local endpoints require exclusive resource ownership. Merely closing HTTP
	// does not prove model memory was released. Remote endpoints never own local
	// model memory. No automatically constructed local lease is provided.
	AcquireLocal  func(context.Context) (func() error, error)
	ManagedOllama *ManagedOllamaConfig
	// VerifyConnection runs before HTTP writes on every newly dialed connection.
	VerifyConnection func(context.Context, net.Conn) error
}

type HTTPProvider struct {
	endpoint  HTTPEndpoint
	client    *http.Client
	mu        sync.Mutex
	active    map[string]context.CancelFunc
	seen      map[string]bool
	closed    bool
	wait      sync.WaitGroup
	release   func() error
	closeOnce sync.Once
	closeErr  error
}

func NewHTTPProvider(ctx context.Context, endpoint HTTPEndpoint) (*HTTPProvider, error) {
	if err := endpoint.Descriptor.Validate(); err != nil {
		return nil, err
	}
	if endpoint.Descriptor.Provider != ProviderOllama && endpoint.Descriptor.Provider != ProviderVLLM ||
		endpoint.Descriptor.AdapterRevision != HTTPEndpointAdapterRevision || endpoint.Model == "" ||
		len(endpoint.Model) > 512 || endpoint.RuntimeVersion == "" || endpoint.Authorize == nil {
		return nil, errors.New("HTTP provider requires an explicitly authorized pinned endpoint")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := endpoint.Authorize(ctx); err != nil {
		return nil, errors.New("HTTP provider authorization is unavailable")
	}
	if endpoint.ManagedOllama != nil {
		var err error
		endpoint, err = prepareManagedOllamaEndpoint(endpoint)
		if err != nil {
			return nil, err
		}
	}
	client, err := endpointHTTPClient(endpoint)
	if err != nil {
		return nil, err
	}
	endpoint.Descriptor.Capabilities = append([]ProviderCapability(nil), endpoint.Descriptor.Capabilities...)
	p := &HTTPProvider{endpoint: endpoint, client: client, active: map[string]context.CancelFunc{}, seen: map[string]bool{}}
	if !endpoint.Remote {
		if endpoint.AcquireLocal == nil {
			return nil, errors.New("local HTTP inference requires an owned resource lease")
		}
		p.release, err = endpoint.AcquireLocal(ctx)
		if err != nil {
			return nil, err
		}
		if p.release == nil {
			return nil, errors.New("local HTTP inference lease has no release proof")
		}
	}
	if err = p.Health(ctx); err != nil {
		if closeErr := p.Close(); closeErr != nil {
			return nil, errors.Join(err, errProviderStopUncertain, closeErr)
		}
		return nil, err
	}
	return p, nil
}

func (p *HTTPProvider) Descriptor(ctx context.Context) (ProviderDescriptor, error) {
	if _, err := p.endpoint.Authorize(ctx); err != nil {
		return ProviderDescriptor{}, errors.New("HTTP provider authorization is unavailable")
	}
	d := p.endpoint.Descriptor
	d.Capabilities = append([]ProviderCapability(nil), d.Capabilities...)
	return d, nil
}

func (p *HTTPProvider) Resources(ctx context.Context) (ProviderResources, error) {
	if _, err := p.Descriptor(ctx); err != nil {
		return ProviderResources{}, err
	}
	return ProviderResources{Backend: p.endpoint.Descriptor.Backend, Device: "enrolled HTTP endpoint", MemoryMeasured: false}, nil
}

func (p *HTTPProvider) request(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	release := func() {}
	if p.endpoint.AcquireRequest != nil {
		var err error
		ctx, release, err = p.endpoint.AcquireRequest(ctx)
		if err != nil {
			return nil, errors.New("HTTP provider request authority is unavailable")
		}
	}
	transferred := false
	defer func() {
		if !transferred {
			release()
		}
	}()
	credential, err := p.endpoint.Authorize(ctx)
	if err != nil {
		return nil, errors.New("HTTP provider authorization is unavailable")
	}
	if len(credential) > 8192 || strings.ContainsAny(credential, "\r\n") {
		return nil, errors.New("HTTP provider credential is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(p.endpoint.URL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("HTTP provider request is invalid")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("HTTP provider transport failed")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP provider returned status %d", resp.StatusCode)
	}
	resp.Body = &endpointResponseBody{ReadCloser: resp.Body, release: release}
	transferred = true
	return resp, nil
}

type endpointResponseBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *endpointResponseBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}

func (p *HTTPProvider) readJSON(ctx context.Context, path string, target any) error {
	resp, err := p.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 || jsonwire.Validate(body) != nil || json.Unmarshal(body, target) != nil {
		return errors.New("HTTP provider metadata is malformed or too large")
	}
	return nil
}

func (p *HTTPProvider) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return errors.New("HTTP provider is closed")
	}
	versionPath, modelsPath := "/version", "/v1/models"
	if p.endpoint.Descriptor.Provider == ProviderOllama {
		versionPath, modelsPath = "/api/version", "/api/tags"
	}
	var version struct {
		Version string `json:"version"`
	}
	if err := p.readJSON(ctx, versionPath, &version); err != nil {
		return err
	}
	if version.Version != p.endpoint.RuntimeVersion {
		return errors.New("HTTP runtime version differs from enrollment")
	}
	var catalog struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name        string `json:"name"`
			Model       string `json:"model"`
			Digest      string `json:"digest"`
			RemoteModel string `json:"remote_model"`
			RemoteHost  string `json:"remote_host"`
		} `json:"models"`
	}
	if err := p.readJSON(ctx, modelsPath, &catalog); err != nil {
		return err
	}
	if p.endpoint.Descriptor.Provider == ProviderOllama {
		for _, m := range catalog.Models {
			if m.Name == p.endpoint.Model || m.Model == p.endpoint.Model {
				if m.RemoteHost != "" || m.RemoteModel != "" {
					return errors.New("Ollama model forwards to an unenrolled upstream")
				}
				if "sha256:"+strings.TrimPrefix(m.Digest, "sha256:") != p.endpoint.Descriptor.ModelDigest {
					return errors.New("Ollama model digest differs from enrollment")
				}
				return nil
			}
		}
	} else {
		for _, m := range catalog.Data {
			if m.ID == p.endpoint.Model {
				return nil
			}
		}
	}
	return errors.New("enrolled model is unavailable; no download was attempted")
}

func (p *HTTPProvider) Generate(ctx context.Context, req ProviderRequest, callback func(ProviderEvent) error) (resultErr error) {
	if err := req.Validate(); err != nil {
		return err
	}
	if callback == nil || req.ModelDigest != p.endpoint.Descriptor.ModelDigest {
		return errors.New("HTTP generation identity is invalid")
	}
	chat, err := httpProviderChat(req, p.endpoint.Descriptor)
	if err != nil {
		return err
	}
	if p.endpoint.AcquireRequest != nil {
		var release func()
		ctx, release, err = p.endpoint.AcquireRequest(ctx)
		if err != nil {
			return errors.New("HTTP generation authority is unavailable")
		}
		// Keep generation cancellation connected while a consumer callback is
		// blocked, not only while the transport is reading response bytes.
		defer release()
	}
	p.mu.Lock()
	if p.closed || p.active[req.CancellationID] != nil || p.seen[req.RequestID] || len(p.seen) >= 10000 {
		p.mu.Unlock()
		return errors.New("HTTP generation identity was used or runtime is unavailable")
	}
	runCtx, cancel := context.WithCancel(ctx)
	p.active[req.CancellationID] = cancel
	p.seen[req.RequestID] = true
	p.wait.Add(1)
	p.mu.Unlock()
	stopCancellation := func() bool { return true }
	if p.endpoint.ManagedOllama != nil {
		// A blocked consumer callback can prevent the stream loop from seeing
		// context cancellation. The owned process must still be retired.
		stopCancellation = context.AfterFunc(runCtx, func() { _ = p.Close() })
	}
	defer func() {
		stopCancellation()
		cancel()
		p.mu.Lock()
		delete(p.active, req.CancellationID)
		p.mu.Unlock()
		p.wait.Done()
		if resultErr != nil && p.endpoint.ManagedOllama != nil {
			resultErr = errors.Join(resultErr, p.Close())
		}
	}()
	if err := p.Health(runCtx); err != nil {
		if runCtx.Err() != nil {
			return runCtx.Err()
		}
		return err
	}
	chat.Model = p.endpoint.Model
	gate := providerStreamGate{descriptor: p.endpoint.Descriptor, request: req, callback: callback}
	var sequence uint64
	emit := func(event ProviderEvent) error {
		sequence++
		event.Sequence = sequence
		event.RequestID = req.RequestID
		event.ProtocolRevision = ProviderProtocolRevision
		return gate.accept(event)
	}
	if p.endpoint.Descriptor.Provider == ProviderOllama {
		err = p.generateOllama(runCtx, req.RequestID, req.ContextLength, chat, emit)
	} else {
		err = p.generateVLLM(runCtx, chat, emit)
	}
	if err != nil {
		if runCtx.Err() != nil {
			return runCtx.Err()
		}
		return err
	}
	if err = runCtx.Err(); err != nil {
		return err
	}
	return gate.finish()
}

func (p *HTTPProvider) Cancel(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	cancel := p.active[id]
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

func (p *HTTPProvider) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		for _, cancel := range p.active {
			cancel()
		}
		p.mu.Unlock()
		done := make(chan struct{})
		go func() { p.wait.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			p.closeErr = errProviderStopUncertain
			p.client.CloseIdleConnections()
			if p.endpoint.ManagedOllama != nil && p.release != nil {
				// The callback remains uncertain, but that must not leave the
				// owned inference tree running indefinitely. Preserve the error
				// even if process termination is now independently proven.
				p.closeErr = errors.Join(p.closeErr, p.release())
			}
			return
		}
		p.client.CloseIdleConnections()
		if p.release != nil {
			p.closeErr = p.release()
		}
	})
	return p.closeErr
}
