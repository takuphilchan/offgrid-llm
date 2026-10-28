// Package execution implements host operations, not another agent loop.
package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

const ProtocolVersion = 1

var (
	ErrAuthority   = errors.New("execution_authority_denied")
	ErrPath        = errors.New("execution_path_outside_workspace")
	ErrRevision    = errors.New("execution_revision_changed")
	ErrLimit       = errors.New("execution_output_limit")
	ErrUnsupported = errors.New("execution_operation_unsupported")
)

// Binding is supplied by the trusted supervisor, never inferred from a prompt.
// Job admission is separate from the session grant: knowing a session ID alone
// does not let a caller dispatch work for a different job.
type Binding struct {
	Actor     string `json:"actor"`
	Workspace string `json:"workspace"`
	Host      string `json:"host"`
	Session   string `json:"session"`
	Grant     string `json:"grant"`
	Revision  int    `json:"revision"`
	Job       string `json:"job"`
}

type Permissions struct {
	Read      bool `json:"read"`
	Write     bool `json:"write"`
	Processes bool `json:"processes"`
}

type Context struct {
	Version     int             `json:"version"`
	Binding     Binding         `json:"binding"`
	Backend     string          `json:"backend"`
	Root        string          `json:"root"`
	Directory   string          `json:"directory"`
	Permissions Permissions     `json:"permissions"`
	Isolation   IsolationPolicy `json:"isolation,omitzero"`
}

type Session struct {
	mu            sync.Mutex
	root          *os.Root
	config        Context
	jobs          map[string]bool
	revoked       atomic.Bool
	revokedSignal chan struct{}
}

func validIdentity(value string) bool {
	return value != "" && len(value) <= 256 && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}

// OpenSession is a host-side authority constructor. Call it only after local
// approval; a service/renderer must not turn arbitrary submitted JSON into it.
func OpenSession(config Context) (*Session, error) {
	b := config.Binding
	for _, value := range []string{b.Actor, b.Workspace, b.Host, b.Session, b.Grant, b.Job} {
		if !validIdentity(value) {
			return nil, ErrAuthority
		}
	}
	if config.Version != ProtocolVersion || b.Revision < 1 || !validBackend(config) || config.Backend == "isolated_linux" && runtime.GOOS != "linux" || !filepath.IsAbs(config.Root) || !utf8.ValidString(config.Root) {
		return nil, ErrAuthority
	}
	root, err := os.OpenRoot(config.Root)
	if err != nil {
		return nil, fmt.Errorf("open approved workspace: %w", err)
	}
	if config.Directory == "" {
		config.Directory = "."
	}
	s := &Session{root: root, config: config, jobs: map[string]bool{b.Job: true}, revokedSignal: make(chan struct{})}
	path, err := s.resolve(config.Directory, ".")
	if err == nil {
		var info os.FileInfo
		info, err = root.Stat(path)
		if err == nil && !info.IsDir() {
			err = ErrPath
		}
	}
	if err != nil {
		root.Close()
		return nil, err
	}
	s.config.Directory = path
	return s, nil
}

func (s *Session) authorize(ctx context.Context, binding Binding) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	expected := s.config.Binding
	expected.Job = binding.Job
	if s.revoked.Load() || binding != expected || !s.jobs[binding.Job] {
		return ErrAuthority
	}
	// Root handles pin an object, while displayed paths name it. Refuse a
	// moved/replaced root rather than reading the old object under a new label.
	pinned, err := s.root.Stat(".")
	if err != nil {
		return err
	}
	current, err := os.Stat(s.config.Root)
	if err != nil || !os.SameFile(pinned, current) {
		return ErrPath
	}
	return nil
}

func validBackend(config Context) bool {
	return config.Backend == "local" && config.Isolation == (IsolationPolicy{}) || config.Backend == "isolated_linux" && config.Isolation.Validate() == nil
}

// AdmitJob is called by the authenticated host supervisor, not a tool operation.
func (s *Session) AdmitJob(binding Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	expected := s.config.Binding
	expected.Job = binding.Job
	if s.revoked.Load() || binding != expected || !validIdentity(binding.Job) {
		return ErrAuthority
	}
	s.jobs[binding.Job] = true
	return nil
}

func (s *Session) Revoke() {
	if s.revoked.CompareAndSwap(false, true) {
		close(s.revokedSignal)
	}
}
func (s *Session) Close() error {
	s.Revoke()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.root.Close()
}

func (s *Session) Snapshot(ctx context.Context, binding Binding) (Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return Context{}, err
	}
	copy := s.config
	copy.Binding = binding
	return copy, nil
}

func (s *Session) resolve(path, directory string) (string, error) {
	if path == "" {
		path = "."
	}
	// Do not reinterpret another platform's paths. Backslashes, drive prefixes
	// and UNC paths are not portable provider-relative names.
	if !utf8.ValidString(path) || strings.ContainsAny(path, "\x00\\:") || filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return "", ErrPath
	}
	path = filepath.Clean(filepath.Join(directory, filepath.FromSlash(path)))
	if !filepath.IsLocal(path) {
		return "", ErrPath
	}
	return path, nil
}

func (s *Session) ChangeDirectory(ctx context.Context, binding Binding, path string) (Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.authorize(ctx, binding); err != nil {
		return Context{}, err
	}
	if !s.config.Permissions.Read {
		return Context{}, ErrAuthority
	}
	name, err := s.resolve(path, s.config.Directory)
	if err != nil {
		return Context{}, err
	}
	info, err := s.root.Stat(name)
	if err != nil {
		return Context{}, err
	}
	if !info.IsDir() {
		return Context{}, ErrPath
	}
	s.config.Directory = name
	copy := s.config
	copy.Binding = binding
	return copy, nil
}
