package computer

import "time"

// DesktopProtocolVersion is negotiated separately. A v2 window consent can
// never become desktop authority by changing its approval policy or target.
const DesktopProtocolVersion = 3
const DesktopActionBudget = 500
const DesktopConsentDuration = 60 * time.Minute

// DesktopAuthority is issued on the host after explicit local consent. Binding
// identifies the task and signed-in desktop; individual steps bind actual
// process generations/surfaces instead. Workspace is immutable enrollment data.
type DesktopAuthority struct {
	Workspace string         `json:"workspace"`
	Binding   ControlBinding `json:"binding"`
}

func (a DesktopAuthority) Validate() error {
	if !opaqueID(a.Workspace) || a.Binding.Validate() != nil || a.Binding.Target.Driver == "browser" || a.Binding.Target.Surface != "desktop" || a.Binding.Target.ProcessGeneration != a.Binding.Target.ID {
		return ErrControlScope
	}
	return nil
}

func (a DesktopAuthority) permits(binding ControlBinding) bool {
	return a.Validate() == nil && binding.Validate() == nil &&
		a.Binding.Actor == binding.Actor && a.Binding.Task == binding.Task &&
		a.Binding.Companion == binding.Companion && a.Binding.Session == binding.Session &&
		a.Binding.Target.OSSession == binding.Target.OSSession && a.Binding.Target.Driver == binding.Target.Driver
}

func (consent LocalConsent) permits(binding ControlBinding) bool {
	if consent.Desktop == nil {
		return consent.Binding == binding
	}
	return consent.Desktop.Binding == consent.Binding && consent.Desktop.permits(binding) &&
		consent.ExpiresAt.Sub(consent.IssuedAt) > 0 && consent.ExpiresAt.Sub(consent.IssuedAt) <= DesktopConsentDuration
}
