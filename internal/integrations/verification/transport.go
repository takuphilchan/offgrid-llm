package verification

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const HermesAdapter = "hermes-0.21.2-files-v1"
const OpenClawAdapter = "openclaw-2026.10.4-files-v1"

// Connection is invocation-private. It must never be persisted or included in
// reports. Hooks receive it through the owned child's environment, not argv.
type Connection struct {
	URL, Token, RunID, Adapter, Workspace string
}

type Session struct {
	mu           sync.Mutex
	boundSession string
	ended        bool
	receipt      *Receipt
	v            *Verifier
	server       *http.Server
	connection   Connection
	stop         func() bool
}

// LoadContract accepts bounded strict JSON, not executable configuration.
func LoadContract(path string) (Contract, error) {
	var c Contract
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return c, errors.New("contract must be a regular file")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return c, errors.New("cannot open contract directory")
	}
	defer root.Close()
	f, err := openRead(root, filepath.Base(path))
	if err != nil {
		return c, errors.New("cannot open verification contract")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return c, errors.New("contract must be a regular file")
	}
	if err = decodeStrict(io.LimitReader(f, 16385), 16384, &c); err != nil {
		return c, errors.New("invalid verification contract JSON")
	}
	if c.Workspace == "" {
		return c, errors.New("contract workspace is required")
	}
	if !filepath.IsAbs(c.Workspace) {
		c.Workspace = filepath.Join(filepath.Dir(path), c.Workspace)
	}
	c.Workspace, err = filepath.Abs(c.Workspace)
	return c, err
}

// No endpoint executes tools, serves file contents or finalizes an invocation.
// The CLI alone calls Finish after the owned external process has stopped.
func Start(ctx context.Context, contract Contract, adapter string) (*Session, error) {
	if adapter != HermesAdapter && adapter != OpenClawAdapter {
		return nil, errors.New("unsupported verification adapter")
	}
	v, err := New(ctx, contract)
	if err != nil {
		return nil, err
	}
	var token [32]byte
	if _, err = rand.Read(token[:]); err != nil {
		v.Close()
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		v.Close()
		return nil, err
	}
	s := &Session{v: v, connection: Connection{URL: "http://" + listener.Addr().String(), Token: hex.EncodeToString(token[:]), RunID: v.RunID(), Adapter: adapter, Workspace: contract.Workspace}}
	s.server = &http.Server{Handler: http.HandlerFunc(s.handle), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 2 * time.Second, MaxHeaderBytes: 4096}
	s.stop = context.AfterFunc(ctx, func() { _ = s.server.Close() })
	go func() { _ = s.server.Serve(listener) }()
	return s, nil
}

func (s *Session) Connection() Connection { return s.connection }
func (s *Session) Close()                 { s.stop(); _ = s.server.Close(); _ = s.v.Close() }
func (s *Session) Finish(ctx context.Context, exit string) Receipt {
	s.stop()
	_ = s.server.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.receipt != nil {
		return *s.receipt
	}
	r := s.v.Finish(ctx, exit)
	if r.State == "passed" && !s.ended {
		r.State, r.Code = "unverified", "hook_lifecycle_incomplete"
	}
	s.receipt = &r
	return r
}

type hookEvent struct {
	Version      int    `json:"version"`
	Adapter      string `json:"adapter"`
	Run          string `json:"run"`
	Session      string `json:"session"`
	Turn         string `json:"turn,omitempty"`
	Call         string `json:"call,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Path         string `json:"path,omitempty"`
	Content      string `json:"content,omitempty"`
	Status       string `json:"status,omitempty"`
	CompleteRead bool   `json:"complete_read,omitempty"`
}

func (s *Session) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost || r.Host != strings.TrimPrefix(s.connection.URL, "http://") || r.Header.Get("Origin") != "" || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/json" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.connection.Token)) != 1 {
		http.Error(w, "verification request rejected", http.StatusForbidden)
		return
	}
	var e hookEvent
	if err := decodeStrict(http.MaxBytesReader(w, r.Body, 8<<20), 8<<20, &e); err != nil || e.Version != Version || e.Adapter != s.connection.Adapter || e.Run != s.connection.RunID {
		http.Error(w, "invalid verification event", http.StatusBadRequest)
		return
	}
	id := Identity{Run: e.Run, Session: e.Session, Turn: e.Turn, Call: e.Call}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended && s.receipt == nil {
		s.v.mu.Lock()
		s.v.fail("event_after_hook_lifecycle_end")
		s.v.mu.Unlock()
	}
	if s.receipt != nil || s.ended {
		http.Error(w, "verification closed", http.StatusConflict)
		return
	}
	d := Decision{Code: "event_invalid"}
	switch r.URL.Path {
	case "/bind":
		if e.Turn != "" || e.Call != "" || e.Kind != "" || e.Path != "" || e.Content != "" || e.Status != "" || e.CompleteRead {
			break
		}
		if s.v.Bind(e.Run, e.Session) == nil {
			s.boundSession = e.Session
			d = Decision{Allowed: true, Code: "hooks_bound"}
		}
	case "/end":
		if e.Session == s.boundSession && e.Session != "" && e.Turn == "" && e.Call == "" && e.Kind == "" && e.Path == "" && e.Content == "" && e.Status == "completed" && !e.CompleteRead {
			s.ended = true
			d = Decision{Allowed: true, Code: "hook_lifecycle_complete"}
		}
	case "/before":
		if e.Status != "" || e.CompleteRead {
			break
		}
		path := e.Path
		if filepath.IsAbs(path) {
			rel, err := filepath.Rel(s.connection.Workspace, path)
			if err != nil {
				path = ""
			} else {
				path = filepath.ToSlash(rel)
			}
		} else {
			// An external tool can have a task cwd different from its process.
			// Never reinterpret relative tool paths as this workspace silently.
			path = ""
		}
		d = s.v.Before(r.Context(), id, Operation{Kind: e.Kind, Path: path, Content: []byte(e.Content)})
	case "/after":
		if e.Kind != "" || e.Path != "" || e.Content != "" {
			break
		}
		d = s.v.After(r.Context(), id, Completion{Status: e.Status, CompleteRead: e.CompleteRead})
	default:
		http.NotFound(w, r)
		return
	}
	if !d.Allowed && d.Message == "" {
		d.Message = rejectionGuidance(d.Code)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}
