package execution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

const HostHeartbeatTimeout = 30 * time.Second

type BridgeMessage struct {
	Version int      `json:"version"`
	ID      string   `json:"id,omitempty"`
	Kind    string   `json:"kind"`
	Binding Binding  `json:"binding"`
	Request *Request `json:"request,omitempty"`
}

type BridgeReply struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Snapshot *Context  `json:"snapshot,omitempty"`
	Response *Response `json:"response,omitempty"`
	Error    string    `json:"error,omitempty"`
}

type pendingBridge struct {
	message   BridgeMessage
	delivered bool
	result    chan BridgeReply
}

// Peer carries operations to an independently approved local supervisor. A
// delivered message is never redelivered, even after a lost HTTP response.
// Host receipts, not a timeout or reconnect, decide the outcome of a mutation.
type Peer struct {
	RecordReceipt func(context.Context, Request, Response) error
	mu            sync.Mutex
	config        Context
	credential    [32]byte
	seen          time.Time
	ready         bool
	closed        bool
	pending       *pendingBridge
	lastID        string
	lastDigest    [32]byte
	changed       chan struct{}
	done          chan struct{}
	gate          chan struct{}
	now           func() time.Time
}

func opaqueToken() (string, error) {
	var value [32]byte
	_, err := rand.Read(value[:])
	return hex.EncodeToString(value[:]), err
}

func newPeer(config Context, token string, now func() time.Time) (*Peer, error) {
	if !validCredential(token) {
		return nil, ErrAuthority
	}
	id, err := opaqueToken()
	if err != nil {
		return nil, err
	}
	message := BridgeMessage{Version: ProtocolVersion, ID: id, Kind: "check", Binding: config.Binding}
	return &Peer{config: config, credential: sha256.Sum256([]byte(token)), seen: now(), changed: make(chan struct{}), done: make(chan struct{}), gate: make(chan struct{}, 1), now: now, pending: &pendingBridge{message: message, result: make(chan BridgeReply, 1)}}, nil
}

func validCredential(token string) bool {
	data, err := hex.DecodeString(token)
	return err == nil && len(data) == 32 && len(token) == 64
}

func (p *Peer) authenticate(token string) error {
	sum := sha256.Sum256([]byte(token))
	if !validCredential(token) || subtle.ConstantTimeCompare(sum[:], p.credential[:]) != 1 {
		return ErrAuthority
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.now().Sub(p.seen) > HostHeartbeatTimeout {
		return ErrHostUnavailable
	}
	p.seen = p.now()
	return nil
}

func (p *Peer) notifyLocked() { close(p.changed); p.changed = make(chan struct{}) }

func (p *Peer) Check(ctx context.Context, b Binding) (Context, error) {
	if err := ctx.Err(); err != nil {
		return Context{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || !p.ready || p.now().Sub(p.seen) > HostHeartbeatTimeout || b != p.config.Binding {
		return Context{}, ErrHostUnavailable
	}
	return p.config, nil
}

func (p *Peer) Poll(ctx context.Context, token string) (BridgeMessage, error) {
	if err := p.authenticate(token); err != nil {
		return BridgeMessage{}, err
	}
	for {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			return BridgeMessage{}, ErrHostUnavailable
		}
		if p.pending != nil && !p.pending.delivered {
			p.pending.delivered = true
			message := p.pending.message
			p.mu.Unlock()
			return message, nil
		}
		changed := p.changed
		p.mu.Unlock()
		select {
		case <-changed:
		case <-p.done:
			return BridgeMessage{}, ErrHostUnavailable
		case <-ctx.Done():
			return BridgeMessage{Version: ProtocolVersion, Kind: "heartbeat"}, ctx.Err()
		}
	}
}

func (p *Peer) Reply(token string, reply BridgeReply) (bool, error) {
	if err := p.authenticate(token); err != nil {
		return false, err
	}
	encoded, err := json.Marshal(reply)
	if err != nil || len(encoded) > MaxFrameBytes || reply.Version != ProtocolVersion || !validIdentity(reply.ID) {
		return false, ErrProtocol
	}
	digest := sha256.Sum256(encoded)
	p.mu.Lock()
	defer p.mu.Unlock()
	if reply.ID == p.lastID {
		if digest != p.lastDigest {
			return false, ErrConflict
		}
		return p.ready, nil
	}
	pending := p.pending
	if pending == nil || !pending.delivered || pending.message.ID != reply.ID {
		return false, ErrConflict
	}
	if pending.message.Kind == "check" {
		if reply.Error != "" || reply.Response != nil || reply.Snapshot == nil || *reply.Snapshot != p.config {
			return false, ErrAuthority
		}
		p.ready = true
	} else {
		if reply.Snapshot != nil || reply.Error != "" && reply.Response != nil {
			return false, ErrProtocol
		}
		isResult := pending.message.Kind == "call" || pending.message.Kind == "receipt"
		if isResult && reply.Error == "" && (reply.Response == nil || pending.message.Request == nil || reply.Response.ID != pending.message.Request.ID || reply.Response.Version != ProtocolVersion) {
			return false, ErrProtocol
		}
		if !isResult && reply.Response != nil {
			return false, ErrProtocol
		}
		if isResult && reply.Error == "" && p.RecordReceipt != nil && pending.message.Request != nil && MutatingOperation(pending.message.Request.Operation) && reply.Response != nil && reply.Response.Error == "" && reply.Response.Receipt != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := p.RecordReceipt(ctx, *pending.message.Request, *reply.Response)
			cancel()
			if err != nil {
				return false, err
			}
		}
		if reply.Response != nil && reply.Response.Receipt != nil && reply.Response.Receipt.State == "complete" && (pending.message.Request.Operation == "directory_change" || pending.message.Request.Operation == "workspace_info") {
			var current Context
			if decodeStrict(reply.Response.Receipt.Result, &current) != nil || current.Binding != pending.message.Binding || current.Root != p.config.Root || current.Backend != p.config.Backend || current.Permissions != p.config.Permissions || current.Isolation != p.config.Isolation {
				return false, ErrProtocol
			}
			p.config.Directory = current.Directory
		}
		pending.result <- reply
	}
	p.lastID, p.lastDigest = reply.ID, digest
	p.pending = nil
	p.notifyLocked()
	return p.ready, nil
}

func (p *Peer) exchange(ctx context.Context, kind string, b Binding, request *Request) (BridgeReply, error) {
	select {
	case p.gate <- struct{}{}:
		defer func() { <-p.gate }()
	case <-ctx.Done():
		return BridgeReply{}, ctx.Err()
	case <-p.done:
		return BridgeReply{}, ErrHostUnavailable
	}
	if err := ctx.Err(); err != nil {
		return BridgeReply{}, err
	}
	id, err := opaqueToken()
	if err != nil {
		return BridgeReply{}, err
	}
	p.mu.Lock()
	expected := p.config.Binding
	expected.Job = b.Job
	if p.closed || !p.ready || p.pending != nil || p.now().Sub(p.seen) > HostHeartbeatTimeout || b != expected || !validIdentity(b.Job) {
		p.mu.Unlock()
		return BridgeReply{}, ErrHostUnavailable
	}
	pending := &pendingBridge{message: BridgeMessage{Version: ProtocolVersion, ID: id, Kind: kind, Binding: b, Request: request}, result: make(chan BridgeReply, 1)}
	p.pending = pending
	p.notifyLocked()
	p.mu.Unlock()
	select {
	case reply := <-pending.result:
		if reply.Error != "" {
			return BridgeReply{}, ErrAuthority
		}
		return reply, nil
	case <-ctx.Done():
		p.Close()
		return BridgeReply{}, ctx.Err()
	case <-p.done:
		return BridgeReply{}, ErrHostUnavailable
	}
}

func (p *Peer) Admit(ctx context.Context, b Binding) error {
	_, err := p.exchange(ctx, "admit", b, nil)
	return err
}
func (p *Peer) Cancel(ctx context.Context, b Binding) error {
	_, err := p.exchange(ctx, "cancel", b, nil)
	return err
}
func (p *Peer) Call(ctx context.Context, r Request) (Response, error) {
	r.Token = ""
	reply, err := p.exchange(ctx, "call", r.Binding, &r)
	if err != nil {
		return Response{}, err
	}
	return *reply.Response, nil
}

func (p *Peer) InspectReceipt(ctx context.Context, r Request) (Response, error) {
	if r.Token != "" {
		return Response{}, ErrAuthority
	}
	reply, err := p.exchange(ctx, "receipt", r.Binding, &r)
	if err != nil {
		return Response{}, err
	}
	return *reply.Response, nil
}
func (p *Peer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		p.ready = false
		close(p.done)
		p.notifyLocked()
	}
	return nil
}
