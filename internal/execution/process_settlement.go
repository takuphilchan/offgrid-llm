package execution

import "errors"

var errProcessCleanupUncertain = errors.New("execution_process_cleanup_unconfirmed")

// Descendant evidence is separate from the command leader's observed exit.
// Missing/unknown/future evidence never settles a new budget or cleanup lease.
type ProcessDescendants struct {
	Version int    `json:"version"`
	State   string `json:"state"`
}

func ProcessDescendantsSettled(p ProcessResult) bool {
	if p.Descendants == nil || p.Descendants.Version != 1 || p.Descendants.State != "settled" {
		return false
	}
	return processLeaderExited(p)
}

func processLeaderExited(p ProcessResult) bool {
	if p.ExitCode == nil {
		return false
	}
	switch p.State {
	case "exited", "failed", "cancelled", "timed_out":
		return true
	}
	return false
}

func ProcessDescendantState(p ProcessResult) string {
	if p.Descendants == nil {
		return "unavailable"
	}
	if p.Descendants.Version != 1 {
		return "invalid"
	}
	switch p.Descendants.State {
	case "pending":
		if p.State == "running" && p.ExitCode == nil {
			return "pending"
		}
	case "unknown":
		return "unknown"
	case "settled":
		if ProcessDescendantsSettled(p) {
			return "settled"
		}
	}
	return "invalid"
}
