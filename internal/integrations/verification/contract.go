// Package verification checks explicit host-file outcomes. It never executes
// tools, writes files, interprets assistant prose or grants external authority.
// It is not a sandbox; callers must stop the external process before Finish.
package verification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
)

const (
	Version       = 1
	ExactCopy     = "exact_copy"
	NoChanges     = "no_changes"
	maxFileBytes  = 1 << 20
	maxTotalBytes = 8 << 20
	maxEntries    = 256
	maxCalls      = 256
)

// Contract comes from trusted invocation configuration, never model output.
// Paths are slash-separated, relative to an explicitly selected local workspace.
// ExactCopy requires a complete source read, one confirmed write and destination
// read-back. Other workspace entries must retain their bytes and permissions.
type Contract struct {
	Version     int    `json:"version"`
	Kind        string `json:"kind"`
	Workspace   string `json:"workspace"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
}

type Identity struct {
	Run     string
	Session string
	Turn    string
	Call    string
}

// Operation contains only a reviewed file operation. Adapters must not forward
// arbitrary tool arguments or credentials here. Content is not retained.
type Operation struct {
	Kind    string // read or write
	Path    string
	Content []byte
}

type Decision struct {
	Allowed bool   `json:"allowed"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"` // bounded transport guidance, never source contents
}

type Completion struct {
	Status       string // completed, blocked (host rejected before dispatch), failed or unknown
	CompleteRead bool   // false for truncated/partial or unrecognized read results
}

type Receipt struct {
	State          string `json:"state"` // passed, failed, unverified or cancelled
	Code           string `json:"code"`
	Kind           string `json:"kind"`
	CheckedEntries int    `json:"checked_entries"`
	ObservedCalls  int    `json:"observed_calls"`
}

type call struct {
	kind, path        string
	allowed, terminal bool
}

type Verifier struct {
	mu                            sync.Mutex
	contract                      Contract
	workspace                     *workspace
	baseline                      snapshot
	run, session                  string
	calls                         map[Identity]*call
	pending                       *call
	sourceRead, written, readBack bool
	failure                       string
	closed                        bool
	receipt                       *Receipt
}

func New(ctx context.Context, contract Contract) (*Verifier, error) {
	if contract.Version != Version || len(contract.Workspace) > 4096 || !filepath.IsAbs(contract.Workspace) {
		return nil, errors.New("verification requires a supported contract and absolute workspace")
	}
	switch contract.Kind {
	case ExactCopy:
		if !validPath(contract.Source) || !validPath(contract.Destination) || strings.EqualFold(contract.Source, contract.Destination) {
			return nil, errors.New("verification requires distinct local source and destination paths")
		}
	case NoChanges:
		if contract.Source != "" || contract.Destination != "" {
			return nil, errors.New("no_changes does not accept copy paths")
		}
	default:
		return nil, errors.New("unsupported verification outcome")
	}
	w, err := openWorkspace(contract.Workspace)
	if err != nil {
		return nil, err
	}
	baseline, err := w.capture(ctx)
	if err != nil {
		w.close()
		return nil, err
	}
	if contract.Kind == ExactCopy {
		source, ok := baseline[contract.Source]
		destination, exists := baseline[contract.Destination]
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(contract.Destination)))
		parentEntry, parentExists := baseline[parent]
		if !ok || source.directory || (exists && destination.directory) || !parentExists || !parentEntry.directory {
			w.close()
			return nil, errors.New("copy source and destination parent must exist; destination must be a file")
		}
	}
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		w.close()
		return nil, err
	}
	return &Verifier{contract: contract, workspace: w, baseline: baseline, run: hex.EncodeToString(nonce[:]), calls: make(map[Identity]*call)}, nil
}

// RunID correlates events; it is not an authentication credential. Any transport
// exposing this object must separately authenticate the trusted hook process.
func (v *Verifier) RunID() string { return v.run }

func validPath(path string) bool {
	return len(path) <= 1024 && path != "." && fs.ValidPath(path) && !strings.ContainsAny(path, "\\:\x00")
}

func validID(id string) bool {
	return id != "" && len(id) <= 256 && !strings.ContainsAny(id, "\x00\r\n")
}

func (v *Verifier) identity(id Identity) bool {
	return id.Run == v.run && id.Session == v.session && validID(id.Session) && validID(id.Turn) && validID(id.Call)
}

// Bind requires a supported hook adapter to identify the selected root session.
// A second or conflicting binding cannot replace the evidence owner.
func (v *Verifier) Bind(run, session string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed || v.receipt != nil || run != v.run || !validID(session) || v.session != "" {
		v.fail("hook_binding_invalid")
		return errors.New("verification hook binding rejected")
	}
	v.session = session
	return nil
}

func (v *Verifier) fail(code string) {
	if v.failure == "" {
		v.failure = code
	}
}

func (v *Verifier) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return nil
	}
	v.closed = true
	return v.workspace.close()
}
