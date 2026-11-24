package inference

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type managedOllamaOwner struct {
	mu  sync.RWMutex
	job *providerProcessJob
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
			return errors.New("managed Ollama has no active process lease")
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
	// State is supplied by the operator and only its existing private directory
	// is opened. No user installation, shared model cache or ACL is modified.
	if _, err := lease.open(c.StateDirectory, true); err != nil {
		return nil, err
	}
	if err := lease.acquireStateLock(c.StateDirectory); err != nil {
		return nil, err
	}
	if err := lease.tree(ctx, c.RuntimeDirectory, c.RuntimeFiles, 4<<30); err != nil {
		return nil, err
	}
	if err := lease.model(ctx, endpoint); err != nil {
		return nil, err
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(endpoint.URL)
	cmd := exec.Command(filepath.Join(c.RuntimeDirectory, filepath.FromSlash(c.Executable)), "serve")
	cmd.Dir = c.RuntimeDirectory
	cmd.Env = []string{
		"SystemRoot=" + filepath.Dir(system), "PATH=" + c.RuntimeDirectory + ";" + system,
		"HOME=" + c.StateDirectory, "USERPROFILE=" + c.StateDirectory, "LOCALAPPDATA=" + c.StateDirectory, "APPDATA=" + c.StateDirectory,
		"TEMP=" + c.StateDirectory, "TMP=" + c.StateDirectory,
		"OLLAMA_HOST=" + u.Host, "OLLAMA_MODELS=" + c.ModelDirectory,
		"OLLAMA_NO_CLOUD=1", "OLLAMA_NOPRUNE=1", "OLLAMA_NUM_PARALLEL=1", "OLLAMA_MAX_LOADED_MODELS=1",
		"OLLAMA_CONTEXT_LENGTH=" + strconv.Itoa(endpoint.Descriptor.ContextLength), "OLLAMA_LOG_REQUESTS=false", "OLLAMA_DEBUG=false",
	}
	// Hashing large pre-existing weights is cancellable. Recheck enrollment and
	// actor authority after verification, immediately before creating a process.
	if token, err := endpoint.Authorize(ctx); err != nil || token != "" {
		return nil, errors.New("managed Ollama launch authority changed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	job, err := startProviderJob(cmd)
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
	if err = awaitManagedOllamaListener(ctx, job, u.Host); err != nil {
		return nil, errors.Join(err, release())
	}
	return release, nil
}

func (lease *managedOllamaFiles) acquireStateLock(directory string) error {
	path := filepath.Join(directory, ".offgrid-managed-ollama.lock")
	encoded, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// CREATE_NEW preserves an unrelated existing object. DELETE_ON_CLOSE makes
	// clean retirement and service crashes release this private admission lock.
	handle, err := windows.CreateFile(encoded, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE, 0, nil, windows.CREATE_NEW, windows.FILE_FLAG_DELETE_ON_CLOSE|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return errors.New("managed Ollama state is already leased or unavailable")
	}
	lease.files = append(lease.files, os.NewFile(uintptr(handle), path))
	return nil
}

func awaitManagedOllamaListener(ctx context.Context, job *providerProcessJob, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		conn, err := (&net.Dialer{Timeout: 250 * time.Millisecond}).DialContext(ctx, "tcp4", address)
		if err == nil {
			err = job.verifyConnection(ctx, conn)
			conn.Close()
			return err
		}
		counts, countErr := providerJobCounts(job.handle)
		if countErr != nil || counts.ActiveProcesses == 0 {
			return errors.New("managed Ollama exited before owning its listener")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
