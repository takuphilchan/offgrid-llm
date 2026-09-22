package computer

import (
	"regexp"
	"strings"
)

type ApprovalMode string

const (
	ApprovalAskEveryTime  ApprovalMode = "ask_every_time"
	ApprovalScopedChanges ApprovalMode = "scoped_changes"
	ApprovalFullTask      ApprovalMode = "full_task"
)

type ActionClass string

const (
	ActionReadOnly      ActionClass = "read_only"
	ActionReversible    ActionClass = "reversible"
	ActionConsequential ActionClass = "consequential"
	ActionForbidden     ActionClass = "forbidden"
)

func (class ActionClass) Valid() bool {
	switch class {
	case ActionReadOnly, ActionReversible, ActionConsequential, ActionForbidden:
		return true
	}
	return false
}

func (mode ApprovalMode) Valid() bool {
	switch mode {
	case ApprovalAskEveryTime, ApprovalScopedChanges, ApprovalFullTask:
		return true
	}
	return false
}

func (mode ApprovalMode) Allows(class ActionClass) bool {
	if !mode.Valid() || class == ActionForbidden {
		return false
	}
	switch mode {
	case ApprovalScopedChanges:
		return class == ActionReadOnly || class == ActionReversible
	case ApprovalFullTask:
		return class != ActionForbidden
	default:
		return class == ActionReadOnly
	}
}

var forbiddenControl = regexp.MustCompile(`(?i)\b(password|passcode|credential|secret|token|one[ -]?time|verification code|credit card|card number|cvv|cvc|payment|pay now|purchase|buy now|checkout|bank|wire|transfer money|install|update software|administrator|admin access|security setting|firewall|antivirus|permanent(?:ly)? delete|delete forever|erase)\b`)
var reversibleControl = regexp.MustCompile(`(?i)\b(save draft|save|apply|add|create draft|update draft)\b`)

// ClassifyOperation is deliberately conservative. Labels and roles come from
// the local platform driver, never from the model or service request.
func ClassifyOperation(op Operation, label, role string) ActionClass {
	context := strings.TrimSpace(label + " " + role)
	if forbiddenControl.MatchString(context) {
		return ActionForbidden
	}
	switch op.Kind {
	case "observe", "capture", "focus":
		return ActionReadOnly
	case "replace_text", "select", "set_checked", "scroll":
		return ActionReversible
	case "shortcut":
		switch op.Shortcut {
		case "copy", "undo", "redo", "select_all", "save", "escape", "tab", "reverse_tab", "left", "right", "up", "down", "page_up", "page_down", "home", "end", "paste":
			return ActionReversible
		default:
			return ActionConsequential
		}
	case "activate":
		if reversibleControl.MatchString(context) {
			return ActionReversible
		}
		return ActionConsequential
	case "navigate":
		return ActionReversible
	case "upload", "download", "create_file", "copy_file", "rename_file", "move_file", "trash_file", "overwrite_file", "coordinate_activate":
		return ActionConsequential
	default:
		return ActionForbidden
	}
}
