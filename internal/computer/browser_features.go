package computer

import (
	"encoding/json"
	"net"
	"net/url"
	"strings"
)

type BrowserFeatures struct {
	Discovery  int `json:"discovery,omitempty"`
	Workspace  int `json:"workspace,omitempty"`
	Structured int `json:"structured,omitempty"`
	Transfers  int `json:"transfers,omitempty"`
}

func (f *BrowserFeatures) UnmarshalJSON(data []byte) error {
	var wire struct {
		Discovery  json.RawMessage `json:"discovery,omitempty"`
		Workspace  json.RawMessage `json:"workspace,omitempty"`
		Structured json.RawMessage `json:"structured,omitempty"`
		Transfers  json.RawMessage `json:"transfers,omitempty"`
	}
	if DecodeControlMessage(data, &wire) != nil {
		return ErrInvalidControl
	}
	*f = BrowserFeatures{}
	if wire.Discovery != nil && (json.Unmarshal(wire.Discovery, &f.Discovery) != nil || f.Discovery != 1) {
		return ErrInvalidControl
	}
	if wire.Workspace != nil && (json.Unmarshal(wire.Workspace, &f.Workspace) != nil || f.Workspace != 1) {
		return ErrInvalidControl
	}
	if wire.Structured != nil && (json.Unmarshal(wire.Structured, &f.Structured) != nil || f.Structured != 1) {
		return ErrInvalidControl
	}
	if wire.Transfers != nil && (json.Unmarshal(wire.Transfers, &f.Transfers) != nil || f.Transfers != 1) {
		return ErrInvalidControl
	}
	return f.Validate()
}
func (f BrowserFeatures) Validate() error {
	if f.Discovery != 0 && f.Discovery != 1 {
		return ErrInvalidControl
	}
	if f.Workspace != 0 && (f.Workspace != 1 || f.Discovery != 1) {
		return ErrInvalidControl
	}
	if f.Structured != 0 && (f.Structured != 1 || f.Discovery != 1 || f.Workspace != 1) {
		return ErrInvalidControl
	}
	if f.Transfers != 0 && (f.Transfers != 1 || f.Structured != 1 || f.Discovery != 1 || f.Workspace != 1) {
		return ErrInvalidControl
	}
	return nil
}
func (h *BrowserHub) PairBrowserFeatures(code, origin string, protocol int, mode ApprovalMode, features BrowserFeatures, scopes ...[]string) (*BrowserSession, string, error) {
	if features.Validate() != nil {
		return nil, "", ErrInvalidControl
	}
	var origins []string
	if len(scopes) > 1 {
		return nil, "", ErrInvalidControl
	}
	if len(scopes) == 1 {
		origins = scopes[0]
	}
	if len(origins) > 0 && features.Workspace != 1 {
		return nil, "", ErrControlScope
	}
	if features.Workspace == 1 {
		if len(origins) == 0 {
			origins = []string{origin}
		}
		if validateBrowserOrigins(origin, origins) != nil {
			return nil, "", ErrControlScope
		}
	}
	u, err := url.Parse(origin)
	if protocol != ProtocolVersion || (origin != "offgrid-demo://research" && (err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/"))) {
		return nil, "", ErrControlScope
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pairLocked(code, u.Scheme+"://"+u.Host, "browser", nil, mode, false, NativeFeatures{}, features, origins)
}

func BrowserWorkspaceTool(name string) bool {
	switch name {
	case "browser_tabs", "browser_tab_open", "browser_tab_select", "browser_tab_close":
		return true
	}
	return false
}

func browserScopeAllows(s *BrowserSession, kind string, args json.RawMessage) bool {
	if kind == "computer_prepare" {
		var prepared struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if json.Unmarshal(args, &prepared) != nil {
			return false
		}
		if BrowserStructuredTool(prepared.Tool) && DecodeControlMessage(args, &prepared) != nil {
			return false
		}
		kind, args = prepared.Tool, prepared.Arguments
	}
	if BrowserStructuredTool(kind) && (s.BrowserFeatures.Structured != 1 || ValidateBrowserStructuredArguments(kind, args) != nil) {
		return false
	}
	if BrowserWorkspaceTool(kind) && s.BrowserFeatures.Workspace != 1 {
		return false
	}
	if kind == "browser_inspect" && s.BrowserFeatures.Discovery != 1 {
		return false
	}
	if kind != "browser_navigate" && kind != "browser_tab_open" {
		return true
	}
	var request struct {
		URL string `json:"url"`
	}
	if json.Unmarshal(args, &request) != nil {
		return false
	}
	u, err := url.Parse(request.URL)
	if err != nil || u.User != nil {
		return false
	}
	// The fixture's random loopback port is known only to its local companion.
	if s.Origin == "offgrid-demo://research" {
		return u.Scheme == "http" && u.Hostname() == "127.0.0.1"
	}
	allowed := s.BrowserOrigins
	if len(allowed) == 0 {
		allowed = []string{s.Origin}
	}
	for _, origin := range allowed {
		if u.Scheme+"://"+u.Host == origin {
			return true
		}
	}
	return false
}

func validateBrowserOrigins(initial string, origins []string) error {
	if len(origins) == 0 || len(origins) > 16 {
		return ErrControlScope
	}
	seen := map[string]bool{}
	for _, origin := range origins {
		if len(origin) > 512 {
			return ErrControlScope
		}
		if origin == "offgrid-demo://research" && initial == origin && len(origins) == 1 {
			return nil
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" || u.Hostname() == "" || u.Host != strings.ToLower(u.Host) || !strings.Contains(u.Hostname(), ".") || net.ParseIP(u.Hostname()) != nil {
			return ErrControlScope
		}
		for _, suffix := range []string{".localhost", ".local", ".internal", ".test", ".invalid"} {
			if strings.HasSuffix(u.Hostname(), suffix) {
				return ErrControlScope
			}
		}
		if seen[origin] {
			return ErrControlScope
		}
		seen[origin] = true
	}
	if !seen[initial] {
		return ErrControlScope
	}
	return nil
}
func (h *BrowserHub) BrowserFeatures(actor, id string) (BrowserFeatures, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, err := h.session(actor, id)
	if err != nil {
		return BrowserFeatures{}, err
	}
	return s.BrowserFeatures, nil
}
