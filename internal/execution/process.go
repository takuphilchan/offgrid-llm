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

const DefaultProcessTimeout = 180 * time.Second
const MaxProcessTimeout = time.Hour

type ProcessSpec struct {
	Program        string   `json:"program"`
	Arguments      []string `json:"arguments"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type ProcessResult struct {
	Handle          string              `json:"handle"`
	Directory       string              `json:"directory"`
	State           string              `json:"state"`
	ExitCode        *int                `json:"exit_code,omitempty"`
	Output          []byte              `json:"output"`
	OutputTruncated bool                `json:"output_truncated"`
	Runtime         *ProcessRuntime     `json:"runtime,omitempty"`
	Descendants     *ProcessDescendants `json:"descendants,omitempty"`
}

// Runtime is supervised elapsed time, including launch and termination handling,
// not CPU time. Missing timing from an older host is not a zero measurement.
type ProcessRuntime struct {
	Version   int   `json:"version"`
	ElapsedNS int64 `json:"elapsed_ns"`
	Settled   bool  `json:"settled"`
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
	mu              sync.Mutex
	binding         Binding
	handle          string
	directory       string
	state           string
	exit            *int
	output          processOutput
	cancel          context.CancelFunc
	done            chan struct{}
	started         time.Time
	elapsed         time.Duration
	cleanupVerified bool
	cleanupRelease  func() error
}

type ProcessManager struct {
	mu              sync.Mutex
	session         *Session
	processes       map[string]*ownedProcess
	order           []string
	closed          bool
	cleanupUnproven bool
	isolation       *LinuxIsolation
	journal         *Journal
}

func NewProcessManager(session *Session) *ProcessManager {
	return &ProcessManager{session: session, processes: map[string]*ownedProcess{}}
}

// Start is a provider operation, not a shell risk classifier. The supervisor
// must authorize the exact invocation under trusted-host or isolated policy
// before dispatch. Program is an already selected executable, not model prose.
func (m *ProcessManager) Start(ctx context.Context, binding Binding, spec ProcessSpec) (ProcessResult, error) {
	return m.start(ctx, binding, spec, nil, nil)
}

func (m *ProcessManager) start(ctx context.Context, binding Binding, spec ProcessSpec, intent *Request, journal *Journal) (ProcessResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ProcessResult{}, ErrAuthority
	}
	if m.cleanupUnproven {
		return ProcessResult{}, ErrHostUnavailable
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
			p.mu.Lock()
			if !p.cleanupVerified {
				active++
			}
			p.mu.Unlock()
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
				p := m.processes[h]
				p.mu.Lock()
				verified := p.cleanupVerified
				p.mu.Unlock()
				// Keep retryable native cleanup handles. Otherwise carry the
				// uncertainty beyond bounded public history eviction.
				if !verified && nativeCleanupVerified(nil) {
					continue
				}
				m.cleanupUnproven = m.cleanupUnproven || !verified
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
	processCtx, cancel := context.WithTimeout(ctx, ProcessTimeout(spec))
	if s.config.Backend == "isolated_linux" {
		if m.isolation == nil || m.journal == nil || intent == nil || intent.Binding != binding {
			cancel()
			return ProcessResult{}, ErrAuthority
		}
		p := &ownedProcess{binding: binding, handle: handle, directory: "/workspace", state: "running", cancel: cancel, done: make(chan struct{})}
		if s.config.Directory != "." {
			p.directory += "/" + filepath.ToSlash(s.config.Directory)
		}
		p.started = time.Now()
		if err := m.isolation.start(processCtx, s.revokedSignal, m.journal, *intent, spec, s.config.Directory, p); err != nil {
			var mutation *MutationError
			m.cleanupUnproven = errors.As(err, &mutation) && mutation.Effects == "unknown"
			cancel()
			return ProcessResult{}, err
		}
		m.processes[handle] = p
		m.order = append(m.order, handle)
		return p.snapshot(), nil
	}
	program, err := s.environment.program(spec.Program, directory)
	if err != nil {
		cancel()
		return ProcessResult{}, err
	}
	if intent != nil {
		if journal == nil || intent.Binding != binding {
			cancel()
			return ProcessResult{}, ErrAuthority
		}
		if err := journal.recordProcessManifest(ctx, *intent, directory, program, s.environment.descriptor()); err != nil {
			cancel()
			return ProcessResult{}, err
		}
	}
	command := exec.Command(program, spec.Arguments...)
	command.Dir = directory
	command.Env = s.environment.clone().values
	command.WaitDelay = 2 * time.Second
	p := &ownedProcess{binding: binding, handle: handle, directory: directory, state: "running", cancel: cancel, done: make(chan struct{})}
	command.Stdout = &p.output
	command.Stderr = &p.output
	p.started = time.Now()
	start := m.session.startProcess
	if start == nil {
		start = startOwned
	}
	stop, wait, release, err := start(command)
	if err != nil {
		m.cleanupUnproven = errors.Is(err, errProcessCleanupUncertain)
		cancel()
		return ProcessResult{}, err
	}
	p.cleanupRelease = release
	m.processes[handle] = p
	m.order = append(m.order, handle)
	go func() {
		defer close(p.done)
		defer cancel()
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
		cleanupErr := release()
		p.mu.Lock()
		defer p.mu.Unlock()
		code := command.ProcessState.ExitCode()
		p.exit = &code
		p.state = state
		p.elapsed = time.Since(p.started)
		p.cleanupVerified = nativeCleanupVerified(cleanupErr)
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
	var timing *ProcessRuntime
	descendants := &ProcessDescendants{Version: 1, State: "unknown"}
	if p.state == "running" {
		descendants.State = "pending"
	} else if p.cleanupVerified {
		descendants.State = "settled"
	}
	result := ProcessResult{Handle: p.handle, Directory: p.directory, State: p.state, ExitCode: exit, Output: output, OutputTruncated: truncated, Descendants: descendants}
	if !p.started.IsZero() {
		elapsed := p.elapsed
		if p.state == "running" {
			elapsed = time.Since(p.started)
		}
		timing = &ProcessRuntime{Version: 1, ElapsedNS: max(int64(elapsed), 0), Settled: ProcessDescendantsSettled(result)}
	}
	result.Runtime = timing
	return result
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

// Receipt settlement follows an already owned dispatch even after revocation.
// It cannot launch/adopt a process or expose a public authority bypass.
func (m *ProcessManager) waitOwned(b Binding, handle string) (ProcessResult, error) {
	m.mu.Lock()
	p := m.processes[handle]
	m.mu.Unlock()
	if p == nil || p.binding != b {
		return ProcessResult{}, ErrAuthority
	}
	<-p.done
	return p.snapshot(), nil
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
		if nativeCleanupVerified(nil) && p.cleanupRelease != nil {
			cleanupErr := p.cleanupRelease()
			p.mu.Lock()
			p.cleanupVerified = cleanupErr == nil
			p.mu.Unlock()
			if cleanupErr != nil {
				err = errors.Join(err, cleanupErr)
			}
		}
		if !ProcessDescendantsSettled(p.snapshot()) {
			err = errors.Join(err, ErrHostUnavailable)
		}
	}
	if m.isolation != nil {
		m.isolation.client.CloseIdleConnections()
	}
	m.mu.Lock()
	if m.cleanupUnproven {
		err = errors.Join(err, ErrHostUnavailable)
	}
	m.mu.Unlock()
	return err
}

// Cleanup requires stronger proof than the observed exit of a command leader.
// A private copy remains leased when any native descendant ownership is unknown.
func (m *ProcessManager) cleanupConfirmed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed || m.cleanupUnproven {
		return false
	}
	for _, p := range m.processes {
		p.mu.Lock()
		confirmed := p.cleanupVerified
		p.mu.Unlock()
		if !confirmed {
			return false
		}
	}
	return true
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
			if !ProcessDescendantsSettled(p.snapshot()) {
				return ErrHostUnavailable
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func processEnvironment() []string {
	keys := []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "XDG_CACHE_HOME", "XDG_CONFIG_HOME"}
	env := []string{}
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}
