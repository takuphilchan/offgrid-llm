package computer

import (
	"net/url"
	"strings"
)

// BrowserFileBinding is a private companion-transport assertion, not a new
// executor grant. Public session listings deliberately do not expose Actor.
type BrowserFileBinding struct {
	Version     int    `json:"version"`
	ActorID     string `json:"actor_id"`
	WorkspaceID string `json:"workspace_id"`
	RunID       string `json:"run_id"`
	InputID     string `json:"input_id"`
	SessionID   string `json:"session_id"`
}

func browserBindingID(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func browserBindingHex(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'f' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func validBrowserTaskBinding(actor, workspace, run, input string) bool {
	return browserBindingID(actor) && browserBindingID(workspace) && strings.HasPrefix(run, "run-") && browserBindingHex(strings.TrimPrefix(run, "run-")) && strings.HasPrefix(input, "input-") && len(input) <= 128 && browserBindingID(strings.TrimPrefix(input, "input-"))
}

func (b BrowserFileBinding) Validate() error {
	if b.Version != 1 || !validBrowserTaskBinding(b.ActorID, b.WorkspaceID, b.RunID, b.InputID) || !browserBindingHex(b.SessionID) {
		return ErrControlScope
	}
	return nil
}

func (s *BrowserSession) FileBinding() (*BrowserFileBinding, error) {
	if s.BrowserFeatures.Transfers == 0 {
		return nil, nil
	}
	if s.Driver != "browser" || s.BrowserFeatures.Validate() != nil || s.run != s.RunID {
		return nil, ErrControlScope
	}
	binding := &BrowserFileBinding{Version: 1, ActorID: s.Actor, WorkspaceID: s.browserWorkspace, RunID: s.RunID, InputID: s.InputID, SessionID: s.ID}
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	return binding, nil
}

// A saved requested URL constrains initial origin. Its path/query are not
// copied into private bindings or logs; companion scope validation stays shared.
func browserRequestedOrigin(raw string) (string, error) {
	if raw == "" || raw == "offgrid-demo://research" {
		return raw, nil
	}
	if len(raw) > 2048 {
		return "", ErrControlScope
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
		return "", ErrControlScope
	}
	host := strings.ToLower(u.Host)
	if u.Port() == "443" {
		host = strings.ToLower(u.Hostname())
	}
	origin := "https://" + host
	if validateBrowserOrigins(origin, []string{origin}) != nil {
		return "", ErrControlScope
	}
	return origin, nil
}
