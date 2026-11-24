package inference

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type managedOllamaOwner struct {
	mu  sync.RWMutex
	job *providerNamespace
}

func prepareManagedOllamaEndpoint(endpoint HTTPEndpoint) (HTTPEndpoint, error) {
	if err := validateManagedOllama(endpoint); err != nil {
		return HTTPEndpoint{}, err
	}
	endpoint.ManagedOllama = cloneManagedOllama(endpoint.ManagedOllama)
	owner := &managedOllamaOwner{}
	endpoint.VerifyConnection = func(ctx context.Context, conn net.Conn) error {
		owner.mu.RLock()
		job := owner.job
		owner.mu.RUnlock()
		if job == nil {
			return errors.New("managed Ollama has no active namespace lease")
		}
		return job.verifyConnection(ctx, conn)
	}
	endpoint.AcquireLocal = func(ctx context.Context) (func() error, error) { return owner.start(ctx, endpoint) }
	return endpoint, nil
}

func (owner *managedOllamaOwner) start(ctx context.Context, endpoint HTTPEndpoint) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := managedLinuxUnprivileged(); err != nil {
		return nil, err
	}
	if token, err := endpoint.Authorize(ctx); err != nil || token != "" {
		return nil, errors.New("managed Ollama requires current local authority without upstream credentials")
	}
	c := endpoint.ManagedOllama
	lease := &managedOllamaFiles{}
	retained := false
	defer func() {
		if !retained {
			lease.close()
		}
	}()
	if err := lease.openState(c.StateDirectory); err != nil {
		return nil, err
	}
	if _, err := lease.open(c.NamespaceLauncher, false); err != nil {
		return nil, err
	}
	if err := lease.tree(ctx, c.RuntimeDirectory, c.RuntimeFiles, 4<<30); err != nil {
		return nil, err
	}
	if err := lease.model(ctx, endpoint); err != nil {
		return nil, err
	}
	u, _ := url.Parse(endpoint.URL)
	cmd := exec.Command(filepath.Join(c.RuntimeDirectory, filepath.FromSlash(c.Executable)), "serve")
	cmd.Dir = c.RuntimeDirectory
	cmd.Env = []string{
		"PATH=" + c.RuntimeDirectory + ":/usr/bin:/bin", "HOME=" + c.StateDirectory,
		"XDG_CONFIG_HOME=" + c.StateDirectory, "XDG_CACHE_HOME=" + c.StateDirectory, "XDG_DATA_HOME=" + c.StateDirectory,
		"TMPDIR=" + c.StateDirectory, "TEMP=" + c.StateDirectory, "TMP=" + c.StateDirectory,
		"OLLAMA_HOST=" + u.Host, "OLLAMA_MODELS=" + c.ModelDirectory,
		"OLLAMA_NO_CLOUD=1", "OLLAMA_NOPRUNE=1", "OLLAMA_NUM_PARALLEL=1", "OLLAMA_MAX_LOADED_MODELS=1",
		"OLLAMA_CONTEXT_LENGTH=" + strconv.Itoa(endpoint.Descriptor.ContextLength), "OLLAMA_LOG_REQUESTS=false", "OLLAMA_DEBUG=false",
	}
	if token, err := endpoint.Authorize(ctx); err != nil || token != "" {
		return nil, errors.New("managed Ollama launch authority changed")
	}
	job, err := startProviderNamespace(ctx, cmd, providerNamespaceLauncher{Path: c.NamespaceLauncher, SHA256: c.NamespaceLauncherSHA256})
	if err != nil {
		retained = errors.Is(err, errProviderStopUncertain)
		if retained {
			lease.retainUncertain()
		}
		return nil, err
	}
	retained = true
	owner.mu.Lock()
	owner.job = job
	owner.mu.Unlock()
	var once sync.Once
	var closeErr error
	release := func() error {
		once.Do(func() {
			closeErr = job.close()
			if closeErr == nil {
				lease.close()
			} else {
				lease.retainUncertain()
			}
		})
		return closeErr
	}
	if err := awaitManagedOllamaListener(ctx, job, u.Host); err != nil {
		return nil, errors.Join(err, release())
	}
	return release, nil
}

func awaitManagedOllamaListener(ctx context.Context, job *providerNamespace, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		conn, err := (&net.Dialer{Timeout: 250 * time.Millisecond}).DialContext(ctx, "tcp4", address)
		if err == nil {
			err = job.verifyConnection(ctx, conn)
			conn.Close()
			return err
		}
		select {
		case <-job.done:
			return errors.New("managed Ollama exited before owning its listener")
		default:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
