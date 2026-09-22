package computer

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
