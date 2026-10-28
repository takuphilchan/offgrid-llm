package execution

import (
	"context"
	"errors"
	"github.com/takuphilchan/offgrid-llm/internal/jsonwire"
	"sort"
	"sync"
)

var ErrHostUnavailable = errors.New("execution_host_unavailable")

// Reference is an address, not a grant. The host must independently hold the
// matching local approval and admit the canonical job before any dispatch.
type Reference struct {
	Host     string `json:"host"`
	Session  string `json:"session"`
	Grant    string `json:"grant"`
	Revision int    `json:"revision"`
}

func (r Reference) Validate() error {
	if !validIdentity(r.Host) || !validIdentity(r.Session) || !validIdentity(r.Grant) || r.Revision < 1 {
		return ErrAuthority
	}
	return nil
}

func reference(b Binding) Reference { return Reference{b.Host, b.Session, b.Grant, b.Revision} }

// HostEndpoint is implemented by an authenticated owned-worker connection.
// Implementations cannot infer local approval from a service request. Admit and
// Cancel are supervisor messages, never model-facing operation names.
type HostEndpoint interface {
	Check(context.Context, Binding) (Context, error)
	Admit(context.Context, Binding) error
	Call(context.Context, Request) (Response, error)
	Cancel(context.Context, Binding) error
	Close() error
}

type Resource struct {
	Reference   Reference   `json:"reference"`
	Backend     string      `json:"backend"`
	Root        string      `json:"root"`
	Directory   string      `json:"directory"`
	Permissions Permissions `json:"permissions"`
}

type hostRegistration struct {
	config   Context
	endpoint HostEndpoint
	jobs     map[string]bool
}

// Hosts only indexes already authenticated connections. RegisterApproved must
// be called by trusted enrollment code AFTER local approval and pack checks,
// never directly with a renderer or model-submitted context. Entries are
// ephemeral: service restart does not restore authority or replay operations.
type Hosts struct {
	mu      sync.RWMutex
	entries map[string]*hostRegistration
}

func NewHosts() *Hosts { return &Hosts{entries: map[string]*hostRegistration{}} }

func (h *Hosts) RegisterApproved(ctx context.Context, config Context, endpoint HostEndpoint) error {
	if endpoint == nil || reference(config.Binding).Validate() != nil || !validIdentity(config.Binding.Actor) || !validIdentity(config.Binding.Workspace) {
		return ErrAuthority
	}
	current, err := endpoint.Check(ctx, config.Binding)
	if err != nil || current != config {
		return ErrAuthority
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) >= 32 || h.entries[config.Binding.Session] != nil {
		return ErrConflict
	}
	h.entries[config.Binding.Session] = &hostRegistration{config: config, endpoint: endpoint, jobs: map[string]bool{}}
	return nil
}

func (h *Hosts) lookup(actor, workspace string, ref Reference) (*hostRegistration, error) {
	if ref.Validate() != nil {
		return nil, ErrAuthority
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	e := h.entries[ref.Session]
	if e == nil || e.config.Binding.Actor != actor || e.config.Binding.Workspace != workspace || reference(e.config.Binding) != ref {
		return nil, ErrHostUnavailable
	}
	return e, nil
}

func (h *Hosts) Resolve(ctx context.Context, actor, workspace string, ref Reference) (Resource, error) {
	e, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return Resource{}, err
	}
	config, err := e.endpoint.Check(ctx, e.config.Binding)
	if err != nil || reference(config.Binding) != ref || config.Binding.Actor != actor || config.Binding.Workspace != workspace || config.Permissions != e.config.Permissions || config.Root != e.config.Root || config.Backend != e.config.Backend || config.Isolation != e.config.Isolation {
		return Resource{}, ErrHostUnavailable
	}
	return Resource{ref, config.Backend, config.Root, config.Directory, config.Permissions}, nil
}

func (h *Hosts) List(ctx context.Context, actor, workspace string) []Resource {
	h.mu.RLock()
	refs := []Reference{}
	for _, e := range h.entries {
		if e.config.Binding.Actor == actor && e.config.Binding.Workspace == workspace {
			refs = append(refs, reference(e.config.Binding))
		}
	}
	h.mu.RUnlock()
	result := []Resource{}
	for _, ref := range refs {
		if r, err := h.Resolve(ctx, actor, workspace, ref); err == nil {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Reference.Session < result[j].Reference.Session })
	return result
}

func (h *Hosts) Admit(ctx context.Context, actor, workspace, job string, ref Reference) error {
	if !validIdentity(job) {
		return ErrAuthority
	}
	e, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return err
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return err
	}
	b := e.config.Binding
	b.Job = job
	if err = e.endpoint.Admit(ctx, b); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.entries[ref.Session] != e {
		return ErrHostUnavailable
	}
	e.jobs[job] = true
	return nil
}

func (h *Hosts) Call(ctx context.Context, actor, workspace, job, call string, ref Reference, operation string, args []byte) (Response, error) {
	return h.CallApproved(ctx, actor, workspace, job, call, ref, operation, args, false)
}

// CallApproved is a trusted service-controller operation. Its approval comes
// from the durable runner's invocation-bound approval, never model arguments.
func (h *Hosts) CallApproved(ctx context.Context, actor, workspace, job, call string, ref Reference, operation string, args []byte, approved bool) (Response, error) {
	if supervisorOperation(operation) {
		return Response{}, ErrUnsupported
	}
	e, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return Response{}, err
	}
	h.mu.RLock()
	admitted := e.jobs[job]
	h.mu.RUnlock()
	if !admitted || !validIdentity(call) {
		return Response{}, ErrAuthority
	}
	if _, err = h.Resolve(ctx, actor, workspace, ref); err != nil {
		return Response{}, err
	}
	b := e.config.Binding
	b.Job = job
	// NUL cannot occur in identities. Operation identity is independent of a
	// transport retry's request ID, and scoped to the canonical job.
	r, err := NewIntent(b, call, operation, args, approved)
	if err != nil {
		return Response{}, err
	}
	response, err := e.endpoint.Call(ctx, r)
	if err != nil {
		return Response{}, err
	}
	if response.Version != ProtocolVersion || response.ID != call || response.Receipt == nil && response.Error == "" {
		return Response{}, ErrProtocol
	}
	if receipt := response.Receipt; receipt != nil {
		if receipt.State == "dispatched" && operation != "process_start" {
			return Response{}, ErrProtocol
		}
		switch receipt.State {
		case "complete", "dispatched", "rejected", "partial", "uncertain":
		default:
			return Response{}, ErrProtocol
		}
		if len(receipt.Result) > MaxFrameBytes || len(receipt.Result) > 0 && jsonwire.Validate(receipt.Result) != nil || len(receipt.Result) == 0 && receipt.State != "uncertain" {
			return Response{}, ErrProtocol
		}
		if response.Error != "" && receipt.State != "uncertain" && receipt.State != "partial" {
			return Response{}, ErrProtocol
		}
	}
	return response, nil
}

func (h *Hosts) Cancel(ctx context.Context, actor, workspace, job string, ref Reference) error {
	e, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return err
	}
	h.mu.RLock()
	admitted := e.jobs[job]
	h.mu.RUnlock()
	if !admitted {
		return ErrAuthority
	}
	b := e.config.Binding
	b.Job = job
	return e.endpoint.Cancel(ctx, b)
}

// CancelIfAdmitted is idempotent supervisory cleanup for a task which may not
// have dispatched any tool. It never creates admission to perform cancellation.
func (h *Hosts) CancelIfAdmitted(ctx context.Context, actor, workspace, job string, ref Reference) error {
	e, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return err
	}
	h.mu.RLock()
	admitted := e.jobs[job]
	h.mu.RUnlock()
	if !admitted {
		return nil
	}
	b := e.config.Binding
	b.Job = job
	return e.endpoint.Cancel(ctx, b)
}

func (h *Hosts) Revoke(actor, workspace string, ref Reference) error {
	e, err := h.lookup(actor, workspace, ref)
	if err != nil {
		return err
	}
	h.mu.Lock()
	if h.entries[ref.Session] == e {
		delete(h.entries, ref.Session)
	}
	h.mu.Unlock()
	return e.endpoint.Close()
}

func (h *Hosts) Close() error {
	h.mu.Lock()
	entries := h.entries
	h.entries = map[string]*hostRegistration{}
	h.mu.Unlock()
	var err error
	for _, e := range entries {
		err = errors.Join(err, e.endpoint.Close())
	}
	return err
}
