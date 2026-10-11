package computer

import (
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
)

// A locally stopped companion relinquishes only its own token. No actor-wide
// stop and no user authentication bypass are exposed by this transport route.
func (h *BrowserHub) RevokeCompanion(token string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.tokenSession(token)
	if err != nil {
		return err
	}
	s.stopped = true
	select {
	case s.replies <- BrowserReply{Error: "session stopped"}:
	default:
	}
	return nil
}

func desktopTool(name string) bool {
	switch name {
	case "computer_applications", "computer_open_application", "computer_windows", "computer_focus_window", "computer_verify_application":
		return true
	}
	return false
}
func desktopReadOnlyTool(name string) bool {
	effect, known := capabilities.NativeOperationEffect(name, capabilities.NativeEffectsRevision)
	return known && effect == capabilities.NativeRead
}

// ActivateDesktop is companion-only, called after the native authority granted
// consent. Enrollment alone never makes a session usable by the runner.
func (h *BrowserHub) ActivateDesktop(token string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.tokenSession(token)
	if err != nil {
		return err
	}
	if s.Desktop == nil || s.Desktop.Validate() != nil {
		return ErrControlScope
	}
	if !s.authorized {
		s.authorized = true
		s.ExpiresAt = time.Now().Add(DesktopConsentDuration)
	}
	return nil
}

func (h *BrowserHub) StopTask(actor, run string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.sessions {
		if s.Actor == actor && s.run == run {
			s.stopped = true
			select {
			case s.replies <- BrowserReply{Error: "session stopped"}:
			default:
			}
		}
	}
	for code, p := range h.pairs {
		if p.actor == actor && p.run == run {
			delete(h.pairs, code)
		}
	}
}

func (h *BrowserHub) DesktopScope(actor, id, run, input, workspace string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.session(actor, id)
	if err != nil {
		return false, err
	}
	if s.Desktop == nil {
		return false, nil
	}
	if !s.authorized || s.run != run || s.InputID != input || s.Desktop.Workspace != workspace {
		return false, ErrControlScope
	}
	return true, nil
}

// Keep the existing BrowserHub name as an internal compatibility adapter while
// callers move to driver-independent sessions. There is one queue and one
// active controller, not parallel browser/native ownership.
func sessionAllowsTool(driver, tool string) bool {
	if driver == "" || driver == "browser" {
		switch tool {
		case "browser_observe", "browser_capture", "browser_navigate", "browser_click", "browser_fill", "browser_select", "browser_set_checked", "browser_download", "browser_upload", "browser_verify", "computer_prepare":
			return true
		}
		return false
	}
	switch driver {
	case "windows-uia", "macos-accessibility", "linux-atspi":
		switch tool {
		case "computer_observe", "computer_replace_text", "computer_activate", "computer_set_checked", "computer_shortcut", "computer_verify", "computer_verify_checked", "computer_prepare":
			return true
		}
	}
	return false
}

func (h *BrowserHub) ApprovalMode(actor, id string) (ApprovalMode, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.session(actor, id)
	if err != nil {
		return "", err
	}
	return s.ApprovalMode, nil
}

// Driver returns only an actor-owned live session's transport identity. It does
// not authorize an action or reveal another user's selected application.
func (h *BrowserHub) Driver(actor, id string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.session(actor, id)
	if err != nil {
		return "", err
	}
	return s.Driver, nil
}
