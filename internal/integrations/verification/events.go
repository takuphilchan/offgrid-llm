package verification

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
)

// Before checks a proposal; it does not authorize or dispatch it. The external
// runtime must still apply its own permissions. Rejected arguments are never
// repaired, stripped or retried by this verifier.
func (v *Verifier) Before(ctx context.Context, id Identity, op Operation) Decision {
	v.mu.Lock()
	defer v.mu.Unlock()
	deny := func(code string) Decision { return Decision{Code: code} }
	if v.closed || v.receipt != nil {
		return deny("verification_closed")
	}
	if ctx.Err() != nil {
		v.fail("cancelled")
		return deny("cancelled")
	}
	if !v.identity(id) {
		v.fail("event_identity_invalid")
		return deny("event_identity_invalid")
	}
	if _, exists := v.calls[id]; exists {
		v.fail("event_replayed")
		return deny("event_replayed")
	}
	if len(v.calls) >= maxCalls {
		v.fail("event_limit_exceeded")
		return deny("event_limit_exceeded")
	}
	record := &call{}
	v.calls[id] = record
	if v.failure != "" {
		return deny(v.failure)
	}
	if !validPath(op.Path) || (op.Kind != "read" && op.Kind != "write") || len(op.Content) > maxFileBytes || (op.Kind == "read" && len(op.Content) != 0) {
		return deny("operation_invalid")
	}
	record.kind, record.path = op.Kind, op.Path
	if v.pending != nil {
		return deny("operation_in_flight")
	}
	after, err := v.capture(ctx, v.written)
	if err != nil {
		v.fail("workspace_changed_or_unreadable")
		return deny(v.failure)
	}
	if op.Kind == "write" {
		if v.contract.Kind == NoChanges {
			v.fail("write_attempt_in_no_changes_run")
			return deny(v.failure)
		}
		if op.Path != v.contract.Destination {
			return deny("write_outside_expected_destination")
		}
		if v.written {
			return deny("write_already_completed")
		}
		if !v.sourceRead {
			return deny("complete_source_read_required")
		}
		expected := v.baseline[v.contract.Source]
		if int64(len(op.Content)) != expected.size || sha256.Sum256(op.Content) != expected.digest {
			// Diagnose only an independently proven extra display prefix. Never
			// modify the proposal or disclose source bytes to the model.
			for n := 1; n <= len(op.Content) && n <= 32; n++ {
				if op.Content[n-1] == '|' && int64(len(op.Content)-n) == expected.size && sha256.Sum256(op.Content[n:]) == expected.digest {
					prefix := op.Content[:n-1]
					if len(prefix) > 0 && bytes.IndexFunc(prefix, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
						return Decision{Code: "content_mismatch", Message: fmt.Sprintf("The write was blocked before execution. Your proposed content has exactly %d extra leading display bytes (%q). Retry the write with only these extra leading bytes removed, then read back the destination. Keep every remaining byte, including any genuine numeric prefix, Unicode and whitespace.", n, string(op.Content[:n]))}
					}
				}
			}
			return deny("content_mismatch")
		}
	} else {
		if file, ok := after[op.Path]; !ok || file.directory {
			return deny("read_target_unavailable")
		}
	}
	record.allowed = true
	v.pending = record
	return Decision{Allowed: true, Code: "proposal_matches_contract"}
}

// After receives only normalized completion metadata from a supported hook.
// Successful tool prose is insufficient: files are independently inspected.
func (v *Verifier) After(ctx context.Context, id Identity, completion Completion) Decision {
	v.mu.Lock()
	defer v.mu.Unlock()
	reject := func(code string) Decision { v.fail(code); return Decision{Code: code} }
	if v.closed || v.receipt != nil {
		return Decision{Code: "verification_closed"}
	}
	if ctx.Err() != nil {
		return reject("cancelled")
	}
	if !v.identity(id) {
		return reject("event_identity_invalid")
	}
	record, exists := v.calls[id]
	if !exists || record.terminal {
		return reject("completion_missing_or_replayed")
	}
	record.terminal = true
	if !record.allowed {
		if completion.Status != "blocked" {
			return reject("rejected_operation_dispatched")
		}
		return Decision{Code: "blocked_operation_confirmed"}
	}
	if record != v.pending {
		return reject("completion_out_of_order")
	}
	v.pending = nil
	if completion.Status != "completed" {
		if completion.Status == "blocked" {
			// This status must mean the trusted host rejected before dispatch,
			// not an error string inferred from assistant/tool prose.
			if _, err := v.capture(ctx, v.written); err != nil {
				return reject("workspace_changed_or_unreadable")
			}
			return Decision{Code: "host_rejected_before_dispatch"}
		}
		if record.kind == "write" {
			return reject("write_effects_unknown")
		}
		if completion.Status != "failed" && completion.Status != "unknown" {
			return reject("completion_invalid")
		}
		return Decision{Code: "read_not_completed"}
	}
	after, err := v.capture(ctx, v.written || record.kind == "write")
	if err != nil {
		return reject("workspace_changed_or_unreadable")
	}
	if record.kind == "write" {
		if !v.destinationMatches(after) {
			return reject("destination_mismatch")
		}
		v.written = true
	} else if completion.CompleteRead {
		if record.path == v.contract.Source {
			v.sourceRead = true
		}
		if record.path == v.contract.Destination && v.written {
			if !v.destinationMatches(after) {
				return reject("destination_mismatch")
			}
			v.readBack = true
		}
	}
	return Decision{Allowed: true, Code: "completion_observed"}
}

func (v *Verifier) destinationMatches(after snapshot) bool {
	want := v.baseline[v.contract.Source]
	got, ok := after[v.contract.Destination]
	return ok && !got.directory && got.size == want.size && got.digest == want.digest
}

// Finish must be called after the owned external invocation has stopped.
// exit is completed, failed or cancelled. A receipt is invocation-local evidence,
// not model qualification; repeated Finish calls return the same frozen receipt.
func (v *Verifier) Finish(ctx context.Context, exit string) Receipt {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.receipt != nil {
		return *v.receipt
	}
	r := Receipt{Kind: v.contract.Kind, ObservedCalls: len(v.calls)}
	defer func() { v.receipt = &r }()
	switch {
	case ctx.Err() != nil || exit == "cancelled" || v.failure == "cancelled":
		r.State, r.Code = "cancelled", "invocation_cancelled"
	case v.closed:
		r.State, r.Code = "unverified", "verification_closed"
	case exit != "completed":
		r.State, r.Code = "failed", "external_invocation_failed"
	case v.failure != "":
		r.State, r.Code = "failed", v.failure
	default:
		after, err := v.capture(ctx, v.written)
		if err != nil {
			r.State, r.Code = "failed", "workspace_changed_or_unreadable"
			break
		}
		r.CheckedEntries = len(after)
		switch {
		case v.session == "":
			r.State, r.Code = "unverified", "supported_hooks_not_bound"
		case v.pending != nil || v.hasMissingCompletion():
			r.State, r.Code = "unverified", "operation_completion_unknown"
		case v.contract.Kind == ExactCopy && (!v.sourceRead || !v.written || !v.readBack):
			r.State, r.Code = "unverified", "dependent_read_write_readback_required"
		case v.contract.Kind == ExactCopy && !v.destinationMatches(after):
			r.State, r.Code = "failed", "destination_mismatch"
		default:
			r.State, r.Code = "passed", "contracted_files_verified"
		}
	}
	return r
}

func (v *Verifier) hasMissingCompletion() bool {
	for _, record := range v.calls {
		if !record.terminal {
			return true
		}
	}
	return false
}
