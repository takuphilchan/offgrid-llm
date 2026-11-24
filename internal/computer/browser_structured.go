package computer

import (
	"encoding/json"
	"slices"
	"strings"
)

func BrowserStructuredTool(name string) bool {
	switch name {
	case "browser_search", "browser_frames", "browser_frame_select":
		return true
	}
	return false
}

func browserStructuredAction(kind string, args json.RawMessage) bool {
	if kind != "computer_prepare" {
		return BrowserStructuredTool(kind)
	}
	var prepared struct {
		Tool string `json:"tool"`
	}
	return json.Unmarshal(args, &prepared) == nil && BrowserStructuredTool(prepared.Tool)
}

// Return a copy, so schema callers cannot change the accepted role vocabulary.
func BrowserSearchRoles() []string {
	return []string{"", "button", "link", "textbox", "checkbox", "radio", "switch", "combobox", "option", "tab", "menuitem"}
}

// Shared by service admission and the hub's direct/prepared dispatch boundary.
// Exact strings remain literal data; selectors, missing fields and ambiguous
// JSON cannot become executable expressions or a different observed identity.
func ValidateBrowserStructuredArguments(name string, args json.RawMessage) error {
	var values map[string]any
	if DecodeControlMessage(args, &values) != nil || values == nil {
		return ErrInvalidControl
	}
	switch name {
	case "browser_frames":
		if len(values) != 0 {
			return ErrInvalidControl
		}
	case "browser_search":
		role, roleOK := values["role"].(string)
		if len(values) != 3 || !roleOK || !slices.Contains(BrowserSearchRoles(), role) {
			return ErrInvalidControl
		}
		any := false
		for _, field := range []string{"role", "name", "text"} {
			value, exists := values[field].(string)
			if !exists || len(value) > 256 {
				return ErrInvalidControl
			}
			any = any || strings.TrimSpace(value) != ""
		}
		if !any {
			return ErrInvalidControl
		}
	case "browser_frame_select":
		if len(values) != 2 {
			return ErrInvalidControl
		}
		for _, field := range []string{"frame_id", "document_id"} {
			value, valid := values[field].(string)
			if !valid || strings.TrimSpace(value) == "" || len(value) > 128 {
				return ErrInvalidControl
			}
		}
	default:
		return ErrInvalidControl
	}
	return nil
}
