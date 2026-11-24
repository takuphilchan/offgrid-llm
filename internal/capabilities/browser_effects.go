package capabilities

// BrowserEffectsRevision pins the reviewed companion effect boundary. A name,
// low-risk hint or third-party error alone never proves pre-dispatch safety.
const BrowserEffectsRevision = "browser-effects-v1"

func TrustedBrowserRead(d Descriptor) bool { return d.Risk == RiskLow && TrustedBrowserOperation(d) }

func TrustedBrowserOperation(d Descriptor) bool {
	if d.Source != "local-companion" || d.Namespace != "computer" || d.Kind != Computer || d.Revision != BrowserEffectsRevision {
		return false
	}
	switch d.Name {
	case "browser_observe", "browser_verify", "browser_capture", "browser_inspect", "browser_tabs", "browser_search", "browser_frames", "browser_frame_select":
		return d.Risk == RiskLow
	case "browser_navigate", "browser_click", "browser_fill", "browser_select", "browser_set_checked", "browser_download", "browser_upload", "browser_tab_open", "browser_tab_select", "browser_tab_close":
		return d.Risk == RiskHigh
	}
	return false
}
