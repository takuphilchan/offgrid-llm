package execution

import (
	"context"
	"crypto/sha256"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Pairing struct {
	Code      string    `json:"code"`
	Actor     string    `json:"actor"`
	Workspace string    `json:"workspace"`
	Protocol  int       `json:"protocol"`
	Version   string    `json:"version"`
	ExpiresAt time.Time `json:"expires_at"`
}

type enrollmentPair struct {
	actor, workspace string
	expires          time.Time
}
type enrolledPeer struct {
	peer              *Peer
	config            Context
	registered        sync.Once
	registrationError error
}

// Enrollment delegates no user credential to a worker. A short-lived pairing
// ticket pins actor/workspace, and a separately generated host token protects
// only that approved connection. Local approval and asset verification are
// independently enforced by the native supervisor before enrollment.
type Enrollment struct {
	RecordReceipt func(context.Context, Request, Response) error
	MayUse        func(actor, workspace string) bool
	mu            sync.Mutex
	hosts         *Hosts
	version       string
	pairs         map[[32]byte]enrollmentPair
	peers         map[string]*enrolledPeer
	closed        bool
	now           func() time.Time
}

func NewEnrollment(hosts *Hosts, version string) *Enrollment {
	return &Enrollment{hosts: hosts, version: CanonicalVersion(version), pairs: map[[32]byte]enrollmentPair{}, peers: map[string]*enrolledPeer{}, now: time.Now}
}

func (e *Enrollment) Pair(actor, workspace string) (Pairing, error) {
	if !validIdentity(actor) || !validIdentity(workspace) {
		return Pairing{}, ErrAuthority
	}
	e.Sweep()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return Pairing{}, ErrHostUnavailable
	}
	for key, previous := range e.pairs {
		if previous.actor == actor && previous.workspace == workspace {
			delete(e.pairs, key)
		}
	}
	if len(e.pairs) >= 16 {
		return Pairing{}, ErrLimit
	}
	code, err := opaqueToken()
	if err != nil {
		return Pairing{}, err
	}
	expires := e.now().Add(2 * time.Minute)
	e.pairs[sha256.Sum256([]byte(code))] = enrollmentPair{actor, workspace, expires}
	return Pairing{code, actor, workspace, ProtocolVersion, e.version, expires}, nil
}

// Validate foreign metadata without opening a host path on the service or
// translating it into a container/WSL path. The native worker validates the
// actual filesystem object using its own platform before publishing metadata.
func validPublishedContext(config Context, pack Pack) bool {
	b := config.Binding
	if config.Version != ProtocolVersion || reference(b).Validate() != nil || b.Job != BootstrapJob || !validBackend(config) || config.Backend == "isolated_linux" && pack.Platform != "linux" || !validIdentity(b.Actor) || !validIdentity(b.Workspace) || len(config.Root) > 32768 || !utf8.ValidString(config.Root) || strings.ContainsRune(config.Root, 0) || len(config.Directory) > 32768 || !utf8.ValidString(config.Directory) || strings.ContainsRune(config.Directory, 0) {
		return false
	}
	root := config.Root
	if pack.Platform == "windows" {
		drive := len(root) >= 3 && ((root[0] >= 'A' && root[0] <= 'Z') || (root[0] >= 'a' && root[0] <= 'z')) && root[1] == ':' && (root[2] == '\\' || root[2] == '/')
		if !drive && !strings.HasPrefix(root, `\\`) {
			return false
		}
	} else if !strings.HasPrefix(root, "/") {
		return false
	}
	directory := strings.ReplaceAll(config.Directory, `\`, "/")
	clean := path.Clean(directory)
	return directory != "" && !strings.Contains(directory, ":") && !strings.HasPrefix(clean, "/") && clean != ".." && !strings.HasPrefix(clean, "../")
}

func (e *Enrollment) Enroll(code, token string, config Context, pack Pack) (Reference, error) {
	if pack.Validate(e.version) != nil || !validPublishedContext(config, pack) || !validCredential(code) || !validCredential(token) {
		return Reference{}, ErrProtocol
	}
	e.Sweep()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return Reference{}, ErrHostUnavailable
	}
	key := sha256.Sum256([]byte(code))
	pair, ok := e.pairs[key]
	if !ok || !e.now().Before(pair.expires) || pair.actor != config.Binding.Actor || pair.workspace != config.Binding.Workspace {
		return Reference{}, ErrAuthority
	}
	if len(e.peers) >= 32 || e.peers[config.Binding.Session] != nil {
		return Reference{}, ErrLimit
	}
	owned := 0
	for _, entry := range e.peers {
		if entry.config.Binding.Actor == pair.actor && entry.config.Binding.Workspace == pair.workspace {
			owned++
		}
	}
	if owned >= 4 {
		return Reference{}, ErrLimit
	}
	peer, err := newPeer(config, token, e.now)
	if err != nil {
		return Reference{}, err
	}
	peer.RecordReceipt = e.RecordReceipt
	delete(e.pairs, key)
	e.peers[config.Binding.Session] = &enrolledPeer{peer: peer, config: config}
	return reference(config.Binding), nil
}

func (e *Enrollment) entry(session string) (*enrolledPeer, error) {
	e.mu.Lock()
	entry := e.peers[session]
	if e.closed || entry == nil {
		e.mu.Unlock()
		return nil, ErrHostUnavailable
	}
	denied := e.MayUse != nil && !e.MayUse(entry.config.Binding.Actor, entry.config.Binding.Workspace)
	e.mu.Unlock()
	if denied {
		e.remove(session, entry)
		return nil, ErrAuthority
	}
	return entry, nil
}

func (e *Enrollment) Poll(ctx context.Context, session, token string) (BridgeMessage, error) {
	entry, err := e.entry(session)
	if err != nil {
		return BridgeMessage{}, err
	}
	return entry.peer.Poll(ctx, token)
}

func (e *Enrollment) Reply(ctx context.Context, session, token string, reply BridgeReply) (bool, error) {
	entry, err := e.entry(session)
	if err != nil {
		return false, err
	}
	ready, err := entry.peer.Reply(token, reply)
	if err != nil {
		return false, err
	}
	if ready {
		// Serialize publication with owner/host revocation. Otherwise a reply
		// which passed Check immediately before removal could reinsert a closed
		// endpoint afterwards, leaking stale metadata and host capacity.
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.closed || e.peers[session] != entry {
			return false, ErrHostUnavailable
		}
		entry.registered.Do(func() { entry.registrationError = e.hosts.RegisterApproved(ctx, entry.config, entry.peer) })
		if entry.registrationError != nil {
			entry.peer.Close()
			return false, entry.registrationError
		}
	}
	return ready, nil
}

func (e *Enrollment) Heartbeat(session, token string) error {
	entry, err := e.entry(session)
	if err != nil {
		return err
	}
	return entry.peer.authenticate(token)
}

func (e *Enrollment) RevokeHost(session, token string) error {
	entry, err := e.entry(session)
	if err != nil {
		return err
	}
	if err = entry.peer.authenticate(token); err != nil {
		return err
	}
	e.remove(session, entry)
	return nil
}

func (e *Enrollment) RevokeOwner(actor, workspace string, ref Reference) error {
	entry, err := e.entry(ref.Session)
	if err != nil {
		return err
	}
	if entry.config.Binding.Actor != actor || entry.config.Binding.Workspace != workspace || reference(entry.config.Binding) != ref {
		return ErrAuthority
	}
	e.remove(ref.Session, entry)
	return nil
}

func (e *Enrollment) remove(session string, entry *enrolledPeer) {
	e.mu.Lock()
	if e.peers[session] == entry {
		delete(e.peers, session)
	}
	e.mu.Unlock()
	entry.peer.Close()
	_ = e.hosts.Revoke(entry.config.Binding.Actor, entry.config.Binding.Workspace, reference(entry.config.Binding))
}

func (e *Enrollment) Sweep() {
	e.mu.Lock()
	expired := map[string]*enrolledPeer{}
	for key, pair := range e.pairs {
		if !e.now().Before(pair.expires) {
			delete(e.pairs, key)
		}
	}
	for session, entry := range e.peers {
		entry.peer.mu.Lock()
		dead := entry.peer.closed || e.now().Sub(entry.peer.seen) > HostHeartbeatTimeout
		entry.peer.mu.Unlock()
		if dead {
			expired[session] = entry
		}
	}
	e.mu.Unlock()
	for session, entry := range expired {
		e.remove(session, entry)
	}
}

func (e *Enrollment) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.Sweep()
		case <-ctx.Done():
			e.Close()
			return
		}
	}
}

func (e *Enrollment) Close() error {
	e.mu.Lock()
	e.closed = true
	entries := e.peers
	e.peers = map[string]*enrolledPeer{}
	e.pairs = map[[32]byte]enrollmentPair{}
	e.mu.Unlock()
	for session, entry := range entries {
		e.remove(session, entry)
	}
	return nil
}
