package computer

import "net/url"

// DesktopPermission is in-memory host consent, never a renderer preference or
// service-issued grant. The owned worker's lifetime/OS watchdog bounds it.
type DesktopPermission struct {
	Service   string `json:"service"`
	Workspace string `json:"workspace"`
	Actor     string `json:"actor"`
}

func (p DesktopPermission) Validate() error {
	u, err := url.Parse(p.Service)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || (u.Hostname() != "127.0.0.1" && u.Hostname() != "::1") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || !opaqueID(p.Workspace) || !opaqueID(p.Actor) {
		return ErrControlScope
	}
	return nil
}

func (p DesktopPermission) Permits(service string, authority DesktopAuthority, mode ApprovalMode) bool {
	return p.Validate() == nil && authority.Validate() == nil && p.Service == service && p.Workspace == authority.Workspace && p.Actor == authority.Binding.Actor && mode == ApprovalFullTask
}
