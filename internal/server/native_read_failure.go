package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// A rejected proposal is not a launch. Preserve conservative recovery after any
// recorded host mutation, but do not prohibit the very first authorized launch.
func nativeReadFailureForTask(task *agents.Task, failure *agents.ToolFailure) *agents.ToolFailure {
	if task == nil || task.Config.ComputerScope != "desktop" || failure.Recovery == "" || (failure.Code != "computer_target_not_selected" && failure.Code != "computer_application_not_ready") {
		return failure
	}
	for i, step := range task.Steps {
		if i < task.Config.ComputerStepOffset || step.Type != "action" {
			continue
		}
		if descriptor, ok := browserDescriptor(step.ToolName); ok && !capabilities.TrustedNativeRead(descriptor) {
			return failure
		}
	}
	copy := *failure
	copy.Message = "No usable application target was found. This read made no changes; no desktop mutation is recorded for this task."
	copy.Recovery = "Discover the intended application with computer_applications, declare its application_open check, then call computer_open_application alone for the first launch. Wait for its actual result before computer_verify_application. Alternatively inspect computer_windows and focus an existing observed target. Rejected batches did not open anything. Never repeat a dispatched mutation."
	return &copy
}

func applicationReadinessFailure(result string, args json.RawMessage) *agents.ToolFailure {
	var response struct {
		Verified    *bool  `json:"verified"`
		State       string `json:"readiness"`
		Check       string `json:"check"`
		Application string `json:"application_id"`
	}
	var request struct {
		Application string `json:"application_id"`
	}
	if json.Unmarshal(args, &request) != nil || json.Unmarshal([]byte(result), &response) != nil || response.Verified == nil || response.Check != "application_open" || request.Application == "" || response.Application != request.Application {
		return nativeReadFailure(computer.ErrInvalidControl)
	}
	if *response.Verified {
		if response.State != "" && response.State != "ready" {
			return nativeReadFailure(computer.ErrInvalidControl)
		}
		return nil
	}
	failure := nativeReadFailure(computer.ErrApplicationNotReady)
	switch response.State {
	case "":
		failure.Readiness = "surface_unavailable" // older host
	case "starting", "surface_unavailable", "observation_unavailable":
		failure.Readiness = response.State
	case "unsupported_identity":
		failure.Readiness, failure.Recovery = response.State, ""
		failure.Message = "The installed application's identity changed or is unsupported. This read made no changes. Refresh application discovery before starting further work; do not repeat an earlier launch."
	default:
		return nativeReadFailure(computer.ErrInvalidControl)
	}
	return failure
}

// Called only after matching the service-authored, versioned native descriptor.
// Never expose raw provider messages or infer lack of effects for a mutation.
func nativeReadFailure(err error) *agents.ToolFailure {
	f := &agents.ToolFailure{Code: "computer_read_failed", Message: "The application read failed without changing its target. Earlier task actions remain recorded.", Cause: err}
	switch {
	case errors.Is(err, computer.ErrTargetNotSelected):
		f.Code = "computer_target_not_selected"
		f.Message = "No application target is selected yet. This observation made no changes; an earlier launch may already have succeeded."
		f.Recovery = "Verify the already-launched application with computer_verify_application. If switching is required, inspect computer_windows and focus the intended observed target. Do not relaunch or repeat input."
	case errors.Is(err, computer.ErrApplicationNotReady):
		f.Code = "computer_application_not_ready"
		f.Message = "The application has not produced a verified usable surface. This readiness check made no changes."
		f.Recovery = "Inspect readiness with computer_verify_application for the same application_id. Do not launch again. A catalog listing or another application's window does not verify this launch."
	case errors.Is(err, computer.ErrDocumentChanged), errors.Is(err, computer.ErrStaleObservation):
		f.Code = "computer_stale_observation"
		if errors.Is(err, computer.ErrDocumentChanged) {
			f.Code = "computer_document_changed"
		}
		f.Message = "The document or its observation changed. This read made no changes; previous input was not undone."
		f.Recovery = "Observe the intended target again before preparing new work. Do not repeat earlier input to make verification pass."
	case errors.Is(err, computer.ErrControlScope):
		f.Code, f.Message = "computer_scope_violation", "The application or task authority could not be verified. This read made no changes. Renew access explicitly; earlier task actions remain recorded."
	case errors.Is(err, computer.ErrDocumentUnavailable):
		f.Code, f.Message = "computer_document_unavailable", "This application does not expose a supported document operation. This read made no changes; earlier task actions remain recorded."
	case errors.Is(err, computer.ErrDocumentIncomplete):
		f.Code, f.Message = "computer_document_incomplete", "The provider could not obtain a complete document snapshot. This read made no changes; earlier task actions remain recorded."
	case errors.Is(err, computer.ErrDocumentComposition):
		f.Code, f.Message = "computer_document_composition_unstable", "Text composition is active or cannot be checked. This read made no changes."
		f.Recovery = "Wait for composition to finish, then inspect documents and read again. Do not repeat earlier input."
	case errors.Is(err, computer.ErrDocumentRange):
		f.Code, f.Message = "computer_document_range_invalid", "The requested range does not match the current complete snapshot. This read made no changes."
	case errors.Is(err, computer.ErrSession):
		f.Code = "computer_session_unavailable"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, computer.ErrUncertain):
		f.Code, f.Message = "computer_read_interrupted", "The application read stopped before a result was available. This read made no changes; earlier task actions remain recorded."
	}
	return f
}
