package computer

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"
)

var ErrSession = errors.New("computer session unavailable; pair the local companion again")
var ErrUncertain = errors.New("computer action outcome is uncertain; inspect the browser before reconciling this task")

type BrowserAction struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Arguments json.RawMessage `json:"arguments"`
}
type BrowserReply struct {
	ID     string `json:"id"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}
type BrowserSession struct {
	ID        string    `json:"id"`
	Origin    string    `json:"origin"`
	ExpiresAt time.Time `json:"expires_at"`
	Remaining int       `json:"remaining_actions"`
	State     string    `json:"state"`
	RunID     string    `json:"run_id,omitempty"`
	Actor     string    `json:"-"`
	token     [32]byte
	run       string
	pending   *BrowserAction
	replies   chan BrowserReply
	delivered bool
	seen      time.Time
	stopped   bool
}
type pairing struct {
	actor   string
	expires time.Time
}

// BrowserHub carries commands only. Agent checkpoints authorize and persist
// intent; the companion independently checks local consent and records dispatch.
// Restart invalidates all credentials and never resumes browser input.
type BrowserHub struct {
	mu       sync.Mutex
	pairs    map[string]pairing
	sessions map[string]*BrowserSession
}

func NewBrowserHub() *BrowserHub {
	return &BrowserHub{pairs: map[string]pairing{}, sessions: map[string]*BrowserSession{}}
}

func (h *BrowserHub) PairCode(actor string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, p := range h.pairs {
		if p.actor == actor || time.Now().After(p.expires) {
			delete(h.pairs, k)
		}
	}
	if actor == "" || len(h.pairs) >= 16 {
		return "", ErrSession
	}
	code := randomID() + randomID()
	h.pairs[code] = pairing{actor: actor, expires: time.Now().Add(2 * time.Minute)}
	return code, nil
}
func (h *BrowserHub) Pair(code, origin string, protocol int) (*BrowserSession, string, error) {
	u, err := url.Parse(origin)
	if origin != "offgrid-demo://research" && (err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/")) {
		return nil, "", fmt.Errorf("select one HTTPS origin without a path or credentials")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.pairs[code]
	if !ok || time.Now().After(p.expires) || protocol != ProtocolVersion {
		return nil, "", ErrSession
	}
	for _, s := range h.sessions {
		if h.live(s) {
			return nil, "", fmt.Errorf("stop the existing browser session before pairing another")
		}
	}
	delete(h.pairs, code)
	// Old tokens, queues and observations must not accumulate across pairings.
	h.sessions = map[string]*BrowserSession{}
	token := randomID() + randomID()
	s := &BrowserSession{ID: randomID(), Actor: p.actor, Origin: u.Scheme + "://" + u.Host, ExpiresAt: time.Now().Add(10 * time.Minute), Remaining: 100, token: sha256.Sum256([]byte(token)), seen: time.Now(), replies: make(chan BrowserReply, 1)}
	h.sessions[s.ID] = s
	copy := *s
	copy.State = "ready"
	return &copy, token, nil
}
func (h *BrowserHub) live(s *BrowserSession) bool {
	return !s.stopped && time.Now().Before(s.ExpiresAt) && time.Since(s.seen) < 30*time.Second
}
func (h *BrowserHub) session(actor, id string) (*BrowserSession, error) {
	s := h.sessions[id]
	if s == nil || s.Actor != actor || !h.live(s) {
		return nil, ErrSession
	}
	return s, nil
}
func (h *BrowserHub) List(actor string) []BrowserSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := []BrowserSession{}
	for _, s := range h.sessions {
		if s.Actor == actor && h.live(s) {
			copy := *s
			copy.State, copy.RunID = "ready", s.run
			if s.run != "" {
				copy.State = "in_use"
			}
			if s.Remaining <= 0 {
				copy.State = "exhausted"
			}
			out = append(out, copy)
		}
	}
	return out
}

// Reserve before launching inference so simultaneous submissions cannot both
// acknowledge work on the same single-task browser session.
func (h *BrowserHub) Reserve(actor, id, run string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.session(actor, id)
	if err != nil {
		return err
	}
	if run == "" || s.run != "" || s.Remaining <= 0 {
		return ErrSession
	}
	s.run = run
	return nil
}
func (h *BrowserHub) Check(actor, id, run string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.session(actor, id)
	if err != nil {
		return err
	}
	if s.Remaining <= 0 || (s.run != "" && s.run != run) {
		return ErrSession
	}
	return nil
}
func (h *BrowserHub) Stop(actor string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.sessions {
		if actor == "" || s.Actor == actor {
			s.stopped = true
			select {
			case s.replies <- BrowserReply{Error: "session stopped"}:
			default:
			}
		}
	}
	for code, p := range h.pairs {
		if actor == "" || p.actor == actor {
			delete(h.pairs, code)
		}
	}
}
func (h *BrowserHub) tokenSession(token string) (*BrowserSession, error) {
	digest := sha256.Sum256([]byte(token))
	for _, s := range h.sessions {
		if subtle.ConstantTimeCompare(digest[:], s.token[:]) == 1 && h.live(s) {
			return s, nil
		}
	}
	return nil, ErrSession
}
func (h *BrowserHub) Poll(token string) (*BrowserAction, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.tokenSession(token)
	if err != nil {
		return nil, err
	}
	s.seen = time.Now()
	if s.pending == nil || s.delivered {
		return nil, nil
	}
	s.delivered = true
	copy := *s.pending
	return &copy, nil
}

func (h *BrowserHub) Heartbeat(token string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.tokenSession(token)
	if err != nil {
		return err
	}
	s.seen = time.Now()
	return nil
}
func (h *BrowserHub) Reply(token string, reply BrowserReply) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.tokenSession(token)
	if err != nil {
		return err
	}
	s.seen = time.Now()
	if s.pending == nil || !s.delivered || s.pending.ID != reply.ID {
		return ErrSession
	}
	if len(reply.Result) > 48<<10 || len(reply.Error) > 1024 {
		return fmt.Errorf("computer result too large")
	}
	select {
	case s.replies <- reply:
		return nil
	default:
		return ErrSession
	}
}
func (h *BrowserHub) Execute(ctx context.Context, actor, id, run, call, kind string, args json.RawMessage) (string, error) {
	if call == "" || run == "" || len(args) > 16<<10 {
		return "", ErrSession
	}
	h.mu.Lock()
	s, err := h.session(actor, id)
	if err != nil {
		h.mu.Unlock()
		return "", err
	}
	if s.pending != nil || s.Remaining <= 0 || (s.run != "" && s.run != run) {
		h.mu.Unlock()
		return "", ErrSession
	}
	s.run = run
	s.Remaining--
	actionID := run + ":" + call
	s.pending = &BrowserAction{ID: actionID, Kind: kind, Arguments: append(json.RawMessage{}, args...)}
	s.delivered = false
	h.mu.Unlock()
	defer func() { h.mu.Lock(); s.pending = nil; s.delivered = false; h.mu.Unlock() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case reply := <-s.replies:
			if reply.ID != actionID || reply.Error != "" {
				h.Stop(actor)
				return "", ErrUncertain
			}
			return reply.Result, nil
		case <-ctx.Done():
			h.Stop(actor)
			return "", ErrUncertain
		case <-ticker.C:
			h.mu.Lock()
			live := h.live(s)
			h.mu.Unlock()
			if !live {
				h.Stop(actor)
				return "", ErrUncertain
			}
		}
	}
}
