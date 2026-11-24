package computer

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// All protocol-specific scope is populated before publishing the session.
// Caller holds h.mu across code consumption, validation and publication.
func (h *BrowserHub) pairLocked(code, label, driver string, target *TargetIdentity, mode ApprovalMode, desktop bool, features NativeFeatures, browserFeatures BrowserFeatures, origins []string) (*BrowserSession, string, error) {
	p, ok := h.pairs[code]
	if !ok || time.Now().After(p.expires) || !mode.Valid() {
		return nil, "", ErrSession
	}
	if p.browser != nil {
		if err := h.checkBrowserPairLocked(p, label, driver, mode, desktop, browserFeatures, origins); err != nil {
			return nil, "", err
		}
	} else if browserFeatures.Transfers != 0 || desktop != (p.run != "") || (desktop && p.mode != mode) {
		return nil, "", ErrControlScope
	}
	for _, s := range h.sessions {
		if h.live(s) {
			return nil, "", fmt.Errorf("stop the existing computer session before pairing another")
		}
	}
	token := randomID() + randomID()
	s := &BrowserSession{ID: randomID(), Actor: p.actor, Origin: label, Driver: driver, Target: target, ApprovalMode: mode, ExpiresAt: time.Now().Add(10 * time.Minute), Remaining: 100, token: sha256.Sum256([]byte(token)), seen: time.Now(), replies: make(chan BrowserReply, 1)}
	s.NativeFeatures = features
	s.BrowserFeatures = browserFeatures
	s.BrowserOrigins = append([]string(nil), origins...)
	if p.browser != nil {
		s.run, s.RunID, s.InputID = p.run, p.run, p.input
		s.browserWorkspace = p.workspace
	}
	if desktop {
		s.run, s.RunID, s.InputID = p.run, p.run, p.input
		s.Desktop = &DesktopAuthority{Workspace: p.workspace, Binding: ControlBinding{Actor: p.actor, Task: p.run, Companion: s.ID, Session: s.ID, Target: *target}}
		s.ExpiresAt = time.Now().Add(DesktopConsentDuration)
		s.Remaining = DesktopActionBudget
	}
	if _, err := s.FileBinding(); err != nil {
		return nil, "", err
	}
	delete(h.pairs, code)
	// Validate every private binding before atomically replacing old authority.
	h.sessions = map[string]*BrowserSession{}
	h.captures = map[string]transientCapture{}
	h.sessions[s.ID] = s
	copy := *s
	copy.BrowserOrigins = append([]string(nil), s.BrowserOrigins...)
	if s.Target != nil {
		targetCopy := *s.Target
		copy.Target = &targetCopy
	}
	copy.State = "ready"
	if desktop {
		copy.State = "awaiting_consent"
	}
	return &copy, token, nil
}
