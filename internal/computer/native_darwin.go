//go:build darwin && cgo

package computer

/*
#cgo LDFLAGS: -framework ApplicationServices -framework AppKit -framework Foundation -lproc
#include <stdlib.h>
char *offgrid_ax_call(const char *request);
void offgrid_ax_close(void);
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unsafe"
)

// NativeDarwin uses AXUIElement provider operations, not AppleScript, keyboard
// events or a personal browser session. Permission must already be granted to
// the packaged companion identity; the driver never changes OS privacy settings.
type NativeDarwin struct {
	selected NativeTarget
	observed ControlObservation
}

var _ NativeAdapter = (*NativeDarwin)(nil)

func axCall(kind string, args any, into any) error {
	data, err := json.Marshal(map[string]any{"kind": kind, "args": args})
	if err != nil {
		return ErrInvalidControl
	}
	input := C.CString(string(data))
	defer C.free(unsafe.Pointer(input))
	output := C.offgrid_ax_call(input)
	if output == nil {
		return errors.New("computer_provider_unavailable")
	}
	defer C.free(unsafe.Pointer(output))
	var envelope struct {
		Error  string          `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if json.Unmarshal([]byte(C.GoString(output)), &envelope) != nil {
		return ErrInvalidControl
	}
	if envelope.Error != "" {
		switch envelope.Error {
		case "computer_scope_violation":
			return ErrControlScope
		case "computer_approval_invalid":
			return ErrControlApproval
		case "computer_stale_observation":
			return ErrStaleObservation
		case "computer_invalid_action":
			return ErrInvalidControl
		case "computer_uncertain_outcome":
			return ErrUncertain
		case "computer_permission_denied":
			return errors.New("computer_permission_denied")
		default:
			return errors.New("computer_provider_unavailable")
		}
	}
	if into != nil && json.Unmarshal(envelope.Result, into) != nil {
		return ErrInvalidControl
	}
	return nil
}
func NewNativeDarwin() (*NativeDarwin, error) {
	if err := axCall("permissions", nil, nil); err != nil {
		return nil, err
	}
	return &NativeDarwin{}, nil
}
func (d *NativeDarwin) Close() { C.offgrid_ax_close() }
func (d *NativeDarwin) Targets() ([]NativeTarget, error) {
	var targets []NativeTarget
	err := axCall("targets", nil, &targets)
	return targets, err
}
func (d *NativeDarwin) Select(id string) (NativeTarget, error) {
	var target NativeTarget
	err := axCall("select", map[string]string{"target": id}, &target)
	if err == nil {
		d.selected = target
	}
	return target, err
}
func (d *NativeDarwin) Observe(ctx context.Context) (NativeView, error) {
	if err := ctx.Err(); err != nil {
		return NativeView{}, err
	}
	var view NativeView
	err := axCall("observe", nil, &view)
	if err == nil {
		d.observed = view.Observation
	}
	return BoundNativeView(view), err
}
func (d *NativeDarwin) VerifyText(binding ControlBinding, element, expected string, observation ControlObservation) (bool, error) {
	if binding.Validate() != nil || binding.Target != d.selected.Identity || observation != d.observed || observation.Validate(binding.Target, time.Now()) != nil {
		return false, ErrStaleObservation
	}
	var result bool
	err := axCall("verify_text", map[string]any{"element": element, "text": expected, "observation": observation}, &result)
	return result, err
}

func (d *NativeDarwin) VerifyChecked(binding ControlBinding, element string, expected bool, observation ControlObservation) (bool, error) {
	if binding.Validate() != nil || binding.Target != d.selected.Identity || observation != d.observed || observation.Validate(binding.Target, time.Now()) != nil {
		return false, ErrStaleObservation
	}
	var result bool
	err := axCall("verify_checked", map[string]any{"element": element, "checked": expected, "observation": observation}, &result)
	return result, err
}
func (d *NativeDarwin) Prepare(binding ControlBinding, op Operation, observation ControlObservation) (PreparedControlAction, error) {
	if binding.Validate() != nil || binding.Target != d.selected.Identity || op.Validate() != nil {
		return PreparedControlAction{}, ErrControlScope
	}
	if observation != d.observed || observation.Validate(binding.Target, time.Now()) != nil {
		return PreparedControlAction{}, ErrStaleObservation
	}
	if op.Kind != "replace_text" && op.Kind != "activate" && op.Kind != "shortcut" && op.Kind != "set_checked" {
		return PreparedControlAction{}, ErrInvalidControl
	}
	var action PreparedControlAction
	err := axCall("prepare", map[string]any{"operation": op, "observation": observation}, &action)
	return action, err
}
func (d *NativeDarwin) ApprovalSummary(step BoundedStep) (string, error) {
	if _, err := step.Digest(); err != nil {
		return "", err
	}
	if step.Binding.Target != d.selected.Identity {
		return "", ErrControlScope
	}
	var summary string
	err := axCall("approval_summary", step, &summary)
	return summary, err
}
func (d *NativeDarwin) FocusSelected() error { return axCall("focus", nil, nil) }
func (d *NativeDarwin) Dispatch(ctx context.Context, action PreparedControlAction) (ControlResult, error) {
	result := ControlResult{ActionID: action.ID, Outcome: "failed"}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	err := axCall("dispatch", action, &result)
	d.observed = ControlObservation{}
	if err != nil && !errors.Is(err, ErrControlScope) && !errors.Is(err, ErrControlApproval) && !errors.Is(err, ErrStaleObservation) && !errors.Is(err, ErrInvalidControl) {
		result.Outcome = "uncertain"
		result.Uncertain = true
		err = ErrUncertain
	}
	return result, err
}
