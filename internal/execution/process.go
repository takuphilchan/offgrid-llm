package execution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const MaxProcessOutput = 256 * 1024

type ProcessSpec struct {
	Program        string   `json:"program"`
	Arguments      []string `json:"arguments"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type ProcessResult struct {
	Handle          string `json:"handle"`
	Directory       string `json:"directory"`
	State           string `json:"state"`
	ExitCode        *int   `json:"exit_code,omitempty"`
	Output          []byte `json:"output"`
	OutputTruncated bool   `json:"output_truncated"`
}

type processOutput struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (o *processOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := len(p)
	room := MaxProcessOutput - len(o.data)
	if room < len(p) {
		o.truncated = true
		p = p[:room]
	}
	o.data = append(o.data, p...)
	return n, nil
}
func (o *processOutput) snapshot() ([]byte, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]byte{}, o.data...), o.truncated
}

type ownedProcess struct {
	mu        sync.Mutex
	binding   Binding
	handle    string
	directory string
	state     string
	exit      *int
	output    processOutput
	cancel    context.CancelFunc
	done      chan struct{}
}

type ProcessManager struct {
	mu        sync.Mutex
	session   *Session
	processes map[string]*ownedProcess
	order     []string
	closed    bool
	isolation *LinuxIsolation
	journal   *Journal
}

func NewProcessManager(session *Session) *ProcessManager {
	return &ProcessManager{session: session, processes: map[string]*ownedProcess{}}
}

// Start is a provider operation, not a shell risk classifier. The supervisor
// must authorize the exact invocation under trusted-host or isolated policy
// before dispatch. Program is an already selected executable, not model prose.
func (m *ProcessManager) Start(ctx context.Context, binding Binding, spec ProcessSpec) (ProcessResult, error) {
	return m.start(ctx, binding, spec, nil)
}

func (m *ProcessManager) start(ctx context.Context, binding Binding, spec ProcessSpec, intent *Request) (ProcessResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ProcessResult{}, ErrAuthority
	}
	if err := ValidateProcessSpec(spec); err != nil {
		return ProcessResult{}, err
	}
	s := m.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return ProcessResult{}, err
	}
	if !s.config.Permissions.Processes {
		return ProcessResult{}, ErrAuthority
	}
	active := 0
	for _, p := range m.processes {
		select {
		case <-p.done:
		default:
			active++
		}
	}
	if active >= 32 {
		return ProcessResult{}, ErrLimit
	}
	if len(m.processes) >= 128 {
		for i, h := range m.order {
			select {
			case <-m.processes[h].done:
				delete(m.processes, h)
				m.order = append(m.order[:i], m.order[i+1:]...)
				goto room
			default:
			}
		}
		return ProcessResult{}, ErrLimit
	}
room:
	directory := filepath.Join(s.config.Root, s.config.Directory)
	// os.Root pins file operations, but a process cwd is opened by pathname.
	// Fail if the selected root has been renamed/replaced instead of switching
	// the process into an unrelated directory at its former path.
	rootInfo, err := s.root.Stat(".")
	if err != nil {
		return ProcessResult{}, err
	}
	currentInfo, err := os.Stat(s.config.Root)
	if err != nil || !os.SameFile(rootInfo, currentInfo) {
		return ProcessResult{}, ErrPath
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return ProcessResult{}, err
	}
	handle := hex.EncodeToString(nonce[:])
	seconds := spec.TimeoutSeconds
	if seconds == 0 {
		seconds = 180
	}
	processCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	if s.config.Backend == "isolated_linux" {
		if m.isolation == nil || m.journal == nil || intent == nil || intent.Binding != binding {
			cancel()
			return ProcessResult{}, ErrAuthority
		}
		p := &ownedProcess{binding: binding, handle: handle, directory: "/workspace", state: "running", cancel: cancel, done: make(chan struct{})}
		if s.config.Directory != "." {
			p.directory += "/" + filepath.ToSlash(s.config.Directory)
		}
		if err := m.isolation.start(processCtx, s.revokedSignal, m.journal, *intent, spec, s.config.Directory, p); err != nil {
			cancel()
			return ProcessResult{}, err
		}
		m.processes[handle] = p
		m.order = append(m.order, handle)
		return p.snapshot(), nil
	}
	command := exec.Command(spec.Program, spec.Arguments...)
	command.Dir = directory
	command.Env = processEnvironment()
	command.WaitDelay = 2 * time.Second
	p := &ownedProcess{binding: binding, handle: handle, directory: directory, state: "running", cancel: cancel, done: make(chan struct{})}
	command.Stdout = &p.output
	command.Stderr = &p.output
	stop, wait, release, err := startOwned(command)
	if err != nil {
		cancel()
		return ProcessResult{}, err
	}
	m.processes[handle] = p
	m.order = append(m.order, handle)
	go func() {
		defer close(p.done)
		defer cancel()
		defer release()
		finished := make(chan error, 1)
		go func() { finished <- wait() }()
		var waitErr error
		state := "exited"
		select {
		case waitErr = <-finished:
		case <-processCtx.Done():
			state = "cancelled"
			if errors.Is(processCtx.Err(), context.DeadlineExceeded) {
				state = "timed_out"
			}
			_ = stop()
			waitErr = <-finished
		case <-s.revokedSignal:
			state = "cancelled"
			_ = stop()
			waitErr = <-finished
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		code := command.ProcessState.ExitCode()
		p.exit = &code
		p.state = state
		if waitErr != nil && state == "exited" {
			p.state = "failed"
		}
	}()
	return p.snapshot(), nil
}

func (p *ownedProcess) snapshot() ProcessResult {
	p.mu.Lock()
	defer p.mu.Unlock()
	output, truncated := p.output.snapshot()
	var exit *int
	if p.exit != nil {
		copy := *p.exit
		exit = &copy
	}
	return ProcessResult{Handle: p.handle, Directory: p.directory, State: p.state, ExitCode: exit, Output: output, OutputTruncated: truncated}
}

func (m *ProcessManager) get(ctx context.Context, b Binding, handle string) (*ownedProcess, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrAuthority
	}
	m.session.mu.Lock()
	err := m.session.authorize(ctx, b)
	m.session.mu.Unlock()
	if err != nil {
		return nil, err
	}
	p := m.processes[handle]
	if p == nil || p.binding != b {
		return nil, ErrAuthority
	}
	return p, nil
}

func (m *ProcessManager) Inspect(ctx context.Context, b Binding, handle string) (ProcessResult, error) {
	p, err := m.get(ctx, b, handle)
	if err != nil {
		return ProcessResult{}, err
	}
	return p.snapshot(), nil
}
func (m *ProcessManager) Wait(ctx context.Context, b Binding, handle string) (ProcessResult, error) {
	p, err := m.get(ctx, b, handle)
	if err != nil {
		return ProcessResult{}, err
	}
	select {
	case <-p.done:
		return p.snapshot(), nil
	case <-ctx.Done():
		return p.snapshot(), ctx.Err()
	}
}
func (m *ProcessManager) Stop(ctx context.Context, b Binding, handle string) (ProcessResult, error) {
	p, err := m.get(ctx, b, handle)
	if err != nil {
		return ProcessResult{}, err
	}
	p.cancel()
	return m.Wait(ctx, b, handle)
}
func (m *ProcessManager) Close() error {
	m.mu.Lock()
	m.closed = true
	all := make([]*ownedProcess, 0, len(m.processes))
	for _, p := range m.processes {
		p.cancel()
		all = append(all, p)
	}
	m.mu.Unlock()
	var err error
	for _, p := range all {
		<-p.done
		if p.snapshot().State == "uncertain" {
			err = ErrHostUnavailable
		}
	}
	if m.isolation != nil {
		m.isolation.client.CloseIdleConnections()
	}
	return err
}

// CancelJob closes only this admitted job's owned processes. Returning success
// means their supervisors have observed exit, not merely sent a stop request.
func (m *ProcessManager) CancelJob(ctx context.Context, b Binding) error {
	if _, err := m.session.Snapshot(ctx, b); err != nil {
		return err
	}
	m.mu.Lock()
	all := []*ownedProcess{}
	for _, p := range m.processes {
		if p.binding == b {
			p.cancel()
			all = append(all, p)
		}
	}
	m.mu.Unlock()
	for _, p := range all {
		select {
		case <-p.done:
			if p.snapshot().State == "uncertain" {
				return ErrHostUnavailable
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func processEnvironment() []string {
	keys := []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "XDG_CACHE_HOME"}
	env := []string{}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}
