package execution

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// NewIsolatedProcessManager is native-supervisor setup, never a tool operation.
// A restarted worker must settle owned-container cleanup before it is ready.
func NewIsolatedProcessManager(ctx context.Context, session *Session, journal *Journal) (*ProcessManager, error) {
	c := session.config
	if c.Backend != "isolated_linux" || journal == nil {
		return nil, ErrAuthority
	}
	i, err := NewLinuxIsolation(ctx, c.Isolation, c.Root, c.Permissions.Write)
	if err != nil {
		return nil, err
	}
	if err = i.recover(ctx, journal); err != nil {
		i.client.CloseIdleConnections()
		return nil, err
	}
	m := NewProcessManager(session)
	m.isolation, m.journal = i, journal
	return m, nil
}

func containerID(id string) bool {
	data, err := hex.DecodeString(id)
	return err == nil && len(data) == 32 && len(id) == 64
}

// No daemon error body is returned to the model: it can contain private host
// paths. Every regular response has a hard bound and every call has a deadline.
func (i *LinuxIsolation) request(ctx context.Context, method, path string, body any) (int, []byte, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
	}
	r, err := http.NewRequestWithContext(ctx, method, "http://docker.local/v1.47"+path, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	response, err := i.client.Do(r)
	if err != nil {
		return 0, nil, ErrHostUnavailable
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxFrameBytes+1))
	if err != nil || len(data) > MaxFrameBytes {
		return response.StatusCode, nil, ErrProtocol
	}
	return response.StatusCode, data, nil
}

type containerInspection struct {
	ID     string `json:"Id"`
	Name   string
	Config struct {
		Image  string
		Labels map[string]string
	}
	State struct {
		Running, Restarting, Dead bool
		ExitCode                  int
		StartedAt                 string
	}
}

func (i *LinuxIsolation) inspect(ctx context.Context, c containerOwnership) (containerInspection, bool, error) {
	var result containerInspection
	if c.Daemon != i.daemonID || !validCredential(c.Owner) || c.Name != "offgrid-"+c.Owner || c.ID != "" && !containerID(c.ID) {
		return result, false, ErrAuthority
	}
	key := c.ID
	if key == "" {
		key = c.Name
	}
	status, data, err := i.request(ctx, "GET", "/containers/"+key+"/json", nil)
	if err != nil {
		return result, false, err
	}
	if status == 404 {
		return result, false, nil
	}
	var raw map[string]json.RawMessage
	if status != 200 || decodeStrict(data, &raw) != nil || json.Unmarshal(data, &result) != nil || !containerID(result.ID) || c.ID != "" && c.ID != result.ID || result.Name != "/"+c.Name || result.Config.Image != i.policy.Image || result.Config.Labels["io.offgrid.executor.ownership"] != c.Owner {
		return result, false, ErrAuthority
	}
	return result, true, nil
}

func (i *LinuxIsolation) sameDaemon(ctx context.Context) error {
	var info struct{ ID string }
	if err := i.get(ctx, "/info", &info); err != nil {
		return err
	}
	if info.ID != i.daemonID {
		return ErrAuthority
	}
	return nil
}

func (i *LinuxIsolation) cleanup(ctx context.Context, c containerOwnership) error {
	if err := i.sameDaemon(ctx); err != nil {
		return err
	}
	state, exists, err := i.inspect(ctx, c)
	if err != nil || !exists {
		return err
	}
	c.ID = state.ID
	if state.State.Running || state.State.Restarting {
		status, _, err := i.request(ctx, "POST", "/containers/"+c.ID+"/kill?signal=SIGKILL", nil)
		if err != nil || status != 204 && status != 409 {
			return ErrHostUnavailable
		}
		for {
			state, exists, err = i.inspect(ctx, c)
			if err != nil {
				return err
			}
			if !exists {
				return nil
			}
			if !state.State.Running && !state.State.Restarting {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	status, _, err := i.request(ctx, "DELETE", "/containers/"+c.ID+"?force=false&v=false", nil)
	if err != nil || status != 204 && status != 404 {
		return ErrHostUnavailable
	}
	return nil
}

func (i *LinuxIsolation) attach(ctx context.Context, id string) (io.ReadCloser, error) {
	r, err := http.NewRequestWithContext(ctx, "POST", "http://docker.local/v1.47/containers/"+id+"/attach?stream=true&stdout=true&stderr=true&logs=false", nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "tcp")
	// The attached stream lasts for the bounded command, not the five-second
	// control request timeout. Cancellation closes it independently.
	client := *i.client
	client.Timeout = 0
	handshake, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(5*time.Second, cancel)
	r = r.WithContext(handshake)
	response, err := client.Do(r)
	if !timer.Stop() {
		cancel()
	}
	if err != nil {
		cancel()
		return nil, ErrHostUnavailable
	}
	if response.StatusCode != 101 && response.StatusCode != 200 {
		response.Body.Close()
		cancel()
		return nil, ErrHostUnavailable
	}
	return &isolatedStream{ReadCloser: response.Body, cancel: cancel}, nil
}

type isolatedStream struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (s *isolatedStream) Close() error { s.cancel(); return s.ReadCloser.Close() }

func copyContainerOutput(input io.Reader, output io.Writer) error {
	for {
		var header [8]byte
		_, err := io.ReadFull(input, header[:])
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if header[0] != 1 && header[0] != 2 || header[1] != 0 || header[2] != 0 || header[3] != 0 {
			return ErrProtocol
		}
		// CopyN streams without allocating a frame-sized buffer. The output
		// sink retains only MaxProcessOutput while continuing to drain.
		if _, err = io.CopyN(output, input, int64(binary.BigEndian.Uint32(header[4:]))); err != nil {
			return err
		}
	}
}

func (i *LinuxIsolation) start(ctx context.Context, revoked <-chan struct{}, journal *Journal, r Request, spec ProcessSpec, directory string, p *ownedProcess) error {
	if err := inspectIsolationRoot(i.root); err != nil {
		return err
	}
	if err := i.sameDaemon(ctx); err != nil {
		return err
	}
	owner, err := opaqueToken()
	if err != nil {
		return err
	}
	c := containerOwnership{Operation: r.OperationID, Daemon: i.daemonID, Name: "offgrid-" + owner, Owner: owner}
	config, err := i.policy.containerConfig(i.root, directory, owner, i.write, spec)
	if err != nil {
		return err
	}
	if err = journal.claimContainer(ctx, r, c); err != nil {
		return err
	}
	uncertain := func(err error) error { return &MutationError{Cause: err, Effects: "unknown"} }
	status, data, err := i.request(ctx, "POST", "/containers/create?name="+c.Name, config)
	if err != nil {
		return uncertain(err)
	}
	var created struct {
		ID string `json:"Id"`
	}
	var raw map[string]json.RawMessage
	if status != 201 || decodeStrict(data, &raw) != nil || json.Unmarshal(data, &created) != nil || !containerID(created.ID) {
		return uncertain(ErrProtocol)
	}
	c.ID = created.ID
	if err = journal.identifyContainer(ctx, c); err != nil {
		return uncertain(err)
	}
	if _, exists, err := i.inspect(ctx, c); err != nil || !exists {
		return uncertain(ErrAuthority)
	}
	streamctx, stopStream := context.WithCancel(context.Background())
	stream, err := i.attach(streamctx, c.ID)
	if err != nil {
		stopStream()
		return uncertain(err)
	}
	outputDone := make(chan error, 1)
	go func() { outputDone <- copyContainerOutput(stream, &p.output) }()
	status, _, err = i.request(ctx, "POST", "/containers/"+c.ID+"/start", nil)
	if err != nil || status != 204 {
		stream.Close()
		stopStream()
		cleanupctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if i.cleanup(cleanupctx, c) == nil {
			_ = journal.forgetContainer(cleanupctx, c)
		}
		return uncertain(ErrHostUnavailable)
	}
	go i.supervise(ctx, revoked, journal, c, p, stream, stopStream, outputDone)
	return nil
}

func (i *LinuxIsolation) supervise(ctx context.Context, revoked <-chan struct{}, journal *Journal, c containerOwnership, p *ownedProcess, stream io.ReadCloser, stopStream context.CancelFunc, outputDone <-chan error) {
	defer close(p.done)
	defer p.cancel()
	defer stopStream()
	defer stream.Close()
	state := "uncertain"
	var exit *int
	for {
		control, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		observed, exists, err := i.inspect(control, c)
		cancel()
		if err != nil || !exists {
			break
		}
		if !observed.State.Running && !observed.State.Restarting {
			if observed.State.Dead || strings.HasPrefix(observed.State.StartedAt, "0001-") || observed.State.StartedAt == "" {
				break
			}
			code := observed.State.ExitCode
			exit = &code
			state = "exited"
			if code != 0 {
				state = "failed"
			}
			break
		}
		select {
		case <-ctx.Done():
			state = "cancelled"
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				state = "timed_out"
			}
		case <-revoked:
			state = "cancelled"
		case <-time.After(100 * time.Millisecond):
			continue
		}
		control, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		// Kill, then inspect exit before cleanup deletes the evidence.
		status, _, err := i.request(control, "POST", "/containers/"+c.ID+"/kill?signal=SIGKILL", nil)
		if err == nil && (status == 204 || status == 409) {
			for {
				observed, exists, err = i.inspect(control, c)
				if err != nil || !exists {
					break
				}
				if !observed.State.Running && !observed.State.Restarting {
					code := observed.State.ExitCode
					exit = &code
					break
				}
				select {
				case <-control.Done():
					err = control.Err()
				case <-time.After(50 * time.Millisecond):
				}
				if err != nil {
					break
				}
			}
		}
		cancel()
		if err != nil || exit == nil {
			state = "uncertain"
			exit = nil
		}
		break
	}
	// Allow already-buffered final output to drain, never wait indefinitely.
	select {
	case err := <-outputDone:
		if err != nil {
			p.output.mu.Lock()
			p.output.truncated = true
			p.output.mu.Unlock()
		}
	case <-time.After(time.Second):
		stream.Close()
		p.output.mu.Lock()
		p.output.truncated = true
		p.output.mu.Unlock()
	}
	control, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := i.cleanup(control, c); err != nil {
		state = "uncertain"
		exit = nil
	} else if err = journal.forgetContainer(control, c); err != nil {
		state = "uncertain"
		exit = nil
	}
	p.mu.Lock()
	p.state, p.exit = state, exit
	p.mu.Unlock()
}
