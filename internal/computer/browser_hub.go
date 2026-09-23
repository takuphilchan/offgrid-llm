package computer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"sync"
	"time"
)

var ErrSession = errors.New("computer session unavailable; pair the local companion again")
var ErrUncertain = errors.New("computer action outcome is uncertain; inspect the selected application before reconciling this task")

type BrowserAction struct {
	ID            string          `json:"id"`
	Kind          string          `json:"kind"`
	Arguments     json.RawMessage `json:"arguments"`
	Binding       *ControlBinding `json:"binding,omitempty"`
	Authorization string          `json:"authorization,omitempty"`
}
type BrowserReply struct {
	ID     string `json:"id"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}
type BrowserSession struct {
	ID           string          `json:"id"`
	Origin       string          `json:"origin"`
	ExpiresAt    time.Time       `json:"expires_at"`
	Remaining    int             `json:"remaining_actions"`
	State        string          `json:"state"`
	RunID        string          `json:"run_id,omitempty"`
	Actor        string          `json:"-"`
	Driver       string          `json:"driver,omitempty"`
	Target       *TargetIdentity `json:"target,omitempty"`
	ApprovalMode ApprovalMode    `json:"approval_mode"`
	token        [32]byte
	run          string
	pending      *BrowserAction
	replies      chan BrowserReply
	delivered    bool
	seen         time.Time
	stopped      bool
}
type pairing struct {
	actor   string
	expires time.Time
}

type transientCapture struct {
	actor, session, run string
	content             []byte
	expires             time.Time
}

// BrowserHub carries commands only. Agent checkpoints authorize and persist
// intent; the companion independently checks local consent and records dispatch.
// Restart invalidates all credentials and never resumes browser input.
type BrowserHub struct {
	mu       sync.Mutex
	pairs    map[string]pairing
	sessions map[string]*BrowserSession
	captures map[string]transientCapture
}

func NewBrowserHub() *BrowserHub {
	return &BrowserHub{pairs: map[string]pairing{}, sessions: map[string]*BrowserSession{}, captures: map[string]transientCapture{}}
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
func (h *BrowserHub) Pair(code, origin string, protocol int, modes ...ApprovalMode) (*BrowserSession, string, error) {
	mode := ApprovalAskEveryTime
	if len(modes) == 1 {
		mode = modes[0]
	} else if len(modes) > 1 {
		return nil, "", ErrControlApproval
	}
	u, err := url.Parse(origin)
	if origin != "offgrid-demo://research" && (err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/")) {
		return nil, "", fmt.Errorf("select one HTTPS origin without a path or credentials")
	}
	if protocol != ProtocolVersion {
		return nil, "", ErrSession
	}
	return h.pair(code, u.Scheme+"://"+u.Host, "browser", nil, mode)
}

// PairNative is the protocol-2 path. A browser-v1 grant is never translated
// into application authority. The target is issued by the host; the host must
// still obtain independent local consent before observing or changing it.
func (h *BrowserHub) PairNative(code, title string, target TargetIdentity, protocol int, modes ...ApprovalMode) (*BrowserSession, string, error) {
	mode := ApprovalAskEveryTime
	if len(modes) == 1 {
		mode = modes[0]
	} else if len(modes) > 1 {
		return nil, "", ErrControlApproval
	}
	if protocol != ControlProtocolVersion || !target.valid() || target.Driver == "browser" || len(title) == 0 || len(title) > 1024 {
		return nil, "", ErrControlScope
	}
	return h.pair(code, title, target.Driver, &target, mode)
}

func (h *BrowserHub) pair(code, label, driver string, target *TargetIdentity, mode ApprovalMode) (*BrowserSession, string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.pairs[code]
	if !ok || time.Now().After(p.expires) || !mode.Valid() {
		return nil, "", ErrSession
	}
	for _, s := range h.sessions {
		if h.live(s) {
			return nil, "", fmt.Errorf("stop the existing computer session before pairing another")
		}
	}
	delete(h.pairs, code)
	// Old tokens, queues and observations must not accumulate across pairings.
	h.sessions = map[string]*BrowserSession{}
	h.captures = map[string]transientCapture{}
	token := randomID() + randomID()
	s := &BrowserSession{ID: randomID(), Actor: p.actor, Origin: label, Driver: driver, Target: target, ApprovalMode: mode, ExpiresAt: time.Now().Add(10 * time.Minute), Remaining: 100, token: sha256.Sum256([]byte(token)), seen: time.Now(), replies: make(chan BrowserReply, 1)}
	h.sessions[s.ID] = s
	copy := *s
	if s.Target != nil {
		targetCopy := *s.Target
		copy.Target = &targetCopy
	}
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
			if s.Target != nil {
				targetCopy := *s.Target
				copy.Target = &targetCopy
			}
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
	for reference, capture := range h.captures {
		if actor == "" || capture.actor == actor {
			delete(h.captures, reference)
		}
	}
}

// StoreCapture accepts an image only from the authenticated companion while
// the matching capture action is pending. The image remains in memory briefly;
// task history receives only an opaque reference.
func (h *BrowserHub) StoreCapture(token, actionID, observationID, mediaType string, width, height int, encoded string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.tokenSession(token)
	if err != nil || s.Driver != "browser" || s.pending == nil || !s.delivered || s.pending.ID != actionID || s.pending.Kind != "browser_capture" || !opaqueID(observationID) || mediaType != "image/png" {
		return "", ErrControlScope
	}
	var arguments struct {
		Observation string `json:"observation_id"`
	}
	if json.Unmarshal(s.pending.Arguments, &arguments) != nil || arguments.Observation != observationID || width < 1 || height < 1 || width > 8192 || height > 8192 || int64(width)*int64(height) > 32<<20 || len(encoded) > base64.StdEncoding.EncodedLen(5<<20) {
		return "", ErrInvalidControl
	}
	content, err := base64.StdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(content) == 0 || len(content) > 5<<20 {
		return "", ErrInvalidControl
	}
	configuration, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil || configuration.Width != width || configuration.Height != height {
		return "", ErrInvalidControl
	}
	for reference, capture := range h.captures {
		if time.Now().After(capture.expires) || capture.session == s.ID {
			delete(h.captures, reference)
		}
	}
	reference := randomID()
	expires := s.ExpiresAt
	h.captures[reference] = transientCapture{actor: s.Actor, session: s.ID, run: s.run, content: append([]byte(nil), content...), expires: expires}
	return reference, nil
}

// ResolveCapture consumes one observation-bound image. It never exposes data
// to the renderer, task event stream, durable checkpoint, or another actor.
func (h *BrowserHub) ResolveCapture(actor, session, run, reference string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	capture, ok := h.captures[reference]
	if !ok || capture.actor != actor || capture.session != session || capture.run != run || time.Now().After(capture.expires) {
		delete(h.captures, reference)
		return "", ErrSession
	}
	delete(h.captures, reference)
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(capture.content), nil
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
	copy.Arguments = append(json.RawMessage{}, s.pending.Arguments...)
	if s.pending.Binding != nil {
		binding := *s.pending.Binding
		copy.Binding = &binding
	}
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
	return h.ExecuteAuthorized(ctx, actor, id, run, call, kind, args, "")
}

func (h *BrowserHub) ExecuteAuthorized(ctx context.Context, actor, id, run, call, kind string, args json.RawMessage, authorization string) (string, error) {
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
	// Dispatch is restricted by the enrolled driver, not by the model's choice
	// of tool name. Even internal callers cannot send native actions to v1.
	if !sessionAllowsTool(s.Driver, kind) {
		h.mu.Unlock()
		return "", ErrControlScope
	}
	s.run = run
	if kind != "computer_prepare" {
		s.Remaining--
	}
	actionID := run + ":" + call
	s.pending = &BrowserAction{ID: actionID, Kind: kind, Arguments: append(json.RawMessage{}, args...), Authorization: authorization}
	if s.Target != nil {
		s.pending.Binding = &ControlBinding{Actor: actor, Task: run, Companion: s.ID, Session: s.ID, Target: *s.Target}
	}
	s.delivered = false
	h.mu.Unlock()
	defer func() { h.mu.Lock(); s.pending = nil; s.delivered = false; h.mu.Unlock() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case reply := <-s.replies:
			if reply.ID != actionID {
				h.Stop(actor)
				return "", ErrUncertain
			}
			if reply.Error != "" {
				// A reply proves the companion is still connected. Validation,
				// policy, and stale-observation failures happen before native input
				// dispatch and must not be presented as a lost host or tear down an
				// otherwise healthy controller. Unknown and scope-breaking failures
				// remain terminal because their outcome cannot be established here.
				switch reply.Error {
				case "computer_stale_observation":
					return "", fmt.Errorf("Selected application changed or its observation expired. Inspect it again before proposing new work: %w", ErrStaleObservation)
				case "computer_invalid_action", "computer_observation_required":
					return "", fmt.Errorf("The proposed control or action was invalid: %w", ErrInvalidControl)
				case "computer_approval_invalid":
					return "", fmt.Errorf("Local approval was declined, expired, or did not match the requested change: %w", ErrControlApproval)
				case "computer_prohibited_action":
					return "", ErrProhibitedAction
				case "computer_stale_target":
					h.Stop(actor)
					return "", fmt.Errorf("Selected application changed or its observation expired. Inspect it again before proposing new work: %w", ErrStaleObservation)
				case "computer_scope_violation":
					h.Stop(actor)
					return "", fmt.Errorf("The selected application or task scope could not be verified: %w", ErrControlScope)
				default:
					h.Stop(actor)
					return "", ErrUncertain
				}
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
