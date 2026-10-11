package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

// The v3 authority path is deliberately distinct from the v2 window picker.
// All effects still go through the same independent host supervisor/journal.
type desktopWorker struct {
	driver     computer.NativeAdapter
	controller *computer.DesktopController
	journal    *computer.NativeJournal
	ctx        context.Context
	active     *atomic.Pointer[computer.NativeSupervisor]
	supervisor *computer.NativeSupervisor
	authority  computer.DesktopAuthority
	mode       computer.ApprovalMode
	prepared   map[string]desktopPreparation
	permission *computer.DesktopPermission
	documents  *documentWorker
}
type desktopPreparation struct {
	Action  computer.PreparedControlAction `json:"action"`
	Binding computer.ControlBinding        `json:"binding"`
}

func (d *desktopWorker) stop() {
	if d.supervisor != nil {
		d.supervisor.Stop()
	}
}
func (d *desktopWorker) check() error {
	if d.ctx.Err() != nil || d.supervisor == nil {
		return computer.ErrControlScope
	}
	if err := d.supervisor.Check(); err != nil {
		return err
	}
	return d.controller.Check()
}
func (d *desktopWorker) call(req request) (any, error) {
	if req.Kind == "enable_desktop" {
		if d.ctx.Err() != nil || d.supervisor != nil || d.permission != nil || req.Permission == nil || req.Permission.Validate() != nil || d.controller.Check() != nil {
			return nil, computer.ErrControlScope
		}
		if !nativeConfirm(fmt.Sprintf("Enable Full access for this desktop session?\n\nLet tasks work across supported apps without asking again this session.\n\n60 minutes or 500 changes per task. No direct file access; sensitive actions stay blocked.\n\nStop at any time. Access clears on quit, lock, suspend or account/workspace change.\n\nAccess details\nAlways blocked\nCredentials, payments, financial transactions, privilege/security changes, installation, scripts, permanent deletion and uncertain retries.\n\nPermission is never saved to disk.\n\nConnected workspace\nService: %s\nWorkspace: %s", req.Permission.Service, req.Permission.Workspace)) {
			return nil, computer.ErrControlApproval
		}
		if d.ctx.Err() != nil || d.controller.Check() != nil {
			return nil, computer.ErrControlScope
		}
		permission := *req.Permission
		d.permission = &permission
		return map[string]bool{"enabled": true}, nil
	}
	if req.Kind == "release_desktop" {
		if d.permission == nil || d.ctx.Err() != nil || d.controller.Check() != nil {
			return nil, computer.ErrControlScope
		}
		d.stop()
		d.active.Store(nil)
		d.supervisor = nil
		d.authority = computer.DesktopAuthority{}
		d.prepared = nil
		d.documents.clear()
		controller, err := computer.NewDesktopController(d.driver)
		if err != nil {
			return nil, err
		}
		d.controller = controller
		return map[string]bool{"released": true}, d.controller.Check()
	}
	if req.Kind == "desktop_identity" {
		if d.supervisor != nil {
			return nil, computer.ErrControlScope
		}
		return d.controller.Identity(), d.controller.Check()
	}
	if req.Kind == "grant_desktop" {
		if d.supervisor != nil || req.Desktop.Validate() != nil || req.Desktop.Binding.Target != d.controller.Identity() || d.controller.Check() != nil {
			return nil, computer.ErrControlScope
		}
		if d.permission != nil && !d.permission.Permits(req.Service, *req.Desktop, req.ApprovalMode) {
			return nil, computer.ErrControlScope
		}
		policy := map[computer.ApprovalMode]string{computer.ApprovalFullTask: "Full task access: available typed actions run automatically.", computer.ApprovalScopedChanges: "Approve scoped changes: reversible actions run automatically; consequential actions need review.", computer.ApprovalAskEveryTime: "Ask every time: each change needs review."}[req.ApprovalMode]
		prompt := req.Prompt
		if len(prompt) > 2000 {
			prompt = prompt[:2000] + "…"
		}
		if d.permission == nil && !nativeConfirm(fmt.Sprintf("Allow OffGrid desktop access for this task?\n\nTask: %q\n\n%s\n\nOffGrid can discover, open and switch supported applications on this signed-in desktop for 60 minutes or 500 changes. Credentials, payments, financial transactions, privilege/security changes, installations, scripts, permanent deletion and uncertain retries remain blocked. Direct file access is not granted.\n\nUse Stop in OffGrid or the emergency shortcut at any time.", prompt, policy)) {
			return nil, computer.ErrControlApproval
		}
		if d.ctx.Err() != nil || d.controller.Check() != nil {
			return nil, computer.ErrControlScope
		}
		d.authority = *req.Desktop
		d.mode = req.ApprovalMode
		d.prepared = map[string]desktopPreparation{}
		d.documents = newDocumentWorker(d.driver)
		now := time.Now()
		d.supervisor = computer.NewNativeSupervisor(d.journal, computer.LocalConsent{ID: fmt.Sprintf("desktop-consent:%d", now.UnixNano()), Binding: d.authority.Binding, Desktop: &d.authority, ApprovalMode: d.mode, IssuedAt: now, ExpiresAt: now.Add(computer.DesktopConsentDuration), Active: true}, func(ctx context.Context, a computer.PreparedControlAction) (computer.ControlResult, error) {
			if err := ctx.Err(); err != nil {
				return computer.ControlResult{ActionID: a.ID, Outcome: "failed"}, err
			}
			if err := d.controller.Check(); err != nil {
				return computer.ControlResult{ActionID: a.ID, Outcome: "failed"}, err
			}
			if a.Operation.Kind == "open_application" || a.Operation.Kind == "focus_surface" {
				d.documents.clear()
				return d.controller.Dispatch(ctx, a)
			}
			binding, err := d.controller.Binding(d.authority.Binding)
			if err != nil {
				return computer.ControlResult{ActionID: a.ID, Outcome: "failed"}, err
			}
			if a.Operation.Kind == "document" {
				if d.documents == nil || binding != d.documents.binding {
					return computer.ControlResult{ActionID: a.ID, Outcome: "failed"}, computer.ErrControlScope
				}
				// Document input must validate the existing focus, never steal it
				// back to conceal a switch since preparation.
				return d.documents.dispatch(ctx, a)
			}
			d.documents.clear()
			if err := d.driver.FocusSelected(); err != nil {
				return computer.ControlResult{ActionID: a.ID, Outcome: "failed"}, err
			}
			return d.driver.Dispatch(ctx, a)
		})
		d.active.Store(d.supervisor)
		if d.ctx.Err() != nil {
			d.supervisor.Revoke()
			return nil, computer.ErrControlScope
		}
		return map[string]any{"granted": true, "binding": d.authority.Binding, "expires_at": now.Add(computer.DesktopConsentDuration), "remaining_actions": computer.DesktopActionBudget}, nil
	}
	if err := d.check(); err != nil {
		return nil, err
	}
	switch req.Kind {
	case "applications":
		return d.controller.Applications()
	case "surfaces":
		d.prepared = map[string]desktopPreparation{}
		d.documents.clear()
		return d.controller.Surfaces()
	case "verify_application":
		d.prepared = map[string]desktopPreparation{}
		d.documents.clear()
		return d.controller.VerifyApplication(d.ctx, req.Application)
	case "observe":
		d.prepared = map[string]desktopPreparation{}
		d.documents.clear()
		if _, err := d.controller.Binding(d.authority.Binding); err != nil {
			return nil, err
		}
		return d.driver.Observe(d.ctx)
	case "documents", "read_document":
		d.prepared = map[string]desktopPreparation{}
		binding, err := d.controller.Binding(d.authority.Binding)
		if err != nil {
			return nil, err
		}
		if req.Kind == "documents" {
			return d.documents.documents(d.ctx, binding)
		}
		return d.documents.read(d.ctx, binding, *req.Observation, req.Page)
	case "prepare_lifecycle", "prepare":
		binding := d.authority.Binding
		var a computer.PreparedControlAction
		var err error
		if req.Kind == "prepare_lifecycle" {
			d.documents.clear()
			a, err = d.controller.Prepare(*req.Operation)
			if err == nil {
				binding, err = d.controller.LifecycleBinding(binding, *req.Operation)
			}
		} else {
			binding, err = d.controller.Binding(binding)
			if err != nil {
				return nil, err
			}
			if req.Operation.Validate() != nil {
				return nil, computer.ErrInvalidControl
			}
			if req.Operation.Kind == "document" {
				a, err = d.documents.prepare(d.ctx, binding, *req.Operation, *req.Observation)
			} else {
				d.documents.clear()
				observation := *req.Observation
				if refresher, ok := d.driver.(computer.NativeObservationRefresher); ok {
					observation, err = refresher.RefreshObservation(d.ctx, binding, observation)
					if err != nil {
						return nil, err
					}
				}
				a, err = d.driver.Prepare(binding, *req.Operation, observation)
			}
		}
		if err != nil {
			return nil, err
		}
		p := desktopPreparation{Action: a, Binding: binding}
		d.prepared[a.ID] = p
		return p, nil
	case "verify":
		binding, err := d.controller.Binding(d.authority.Binding)
		if err != nil {
			return nil, err
		}
		if req.Operation.Validate() != nil {
			return nil, computer.ErrInvalidControl
		}
		verifier, ok := d.driver.(computer.NativeVerifier)
		if !ok {
			return nil, computer.ErrInvalidControl
		}
		result := map[string]any{"target": binding.Target, "observation_id": req.Observation.ID, "element": req.Operation.Element}
		observation := *req.Observation
		if refresher, ok := d.driver.(computer.NativeObservationRefresher); ok {
			observation, err = refresher.RefreshObservation(d.ctx, binding, observation)
			if err != nil {
				return nil, err
			}
		}
		var verified bool
		switch req.Operation.Kind {
		case "replace_text":
			verified, err = verifier.VerifyText(binding, req.Operation.Element, *req.Operation.Text, observation)
			result["check"] = "control_text_equals"
			result["text"] = *req.Operation.Text
		case "set_checked":
			verified, err = verifier.VerifyChecked(binding, req.Operation.Element, *req.Operation.Checked, observation)
			result["check"] = "control_checked_equals"
			result["checked"] = *req.Operation.Checked
		default:
			return nil, computer.ErrInvalidControl
		}
		result["verified"] = verified
		return result, err
	case "approve":
		if len(req.Step.Actions) != 1 {
			return nil, computer.ErrControlApproval
		}
		a := req.Step.Actions[0]
		p, ok := d.prepared[a.ID]
		actual, _ := json.Marshal(a)
		saved, _ := json.Marshal(p.Action)
		if !ok || string(actual) != string(saved) || req.Step.Binding != p.Binding {
			return nil, computer.ErrControlApproval
		}
		return d.supervisor.Approve(*req.Step, func(step computer.BoundedStep) bool {
			if !a.ApprovalClass.Valid() || a.ApprovalClass == computer.ActionForbidden {
				return false
			}
			if *req.Automatic {
				return d.mode.Allows(a.ApprovalClass)
			}
			var summary string
			var err error
			if a.Operation.Kind == "open_application" || a.Operation.Kind == "focus_surface" {
				summary, err = d.controller.ApprovalSummary(a)
			} else if a.Operation.Kind == "document" {
				summary, err = d.documents.summary(step)
			} else {
				summary, err = d.driver.ApprovalSummary(step)
			}
			return err == nil && nativeConfirm(summary)
		})
	case "execute":
		result, err := d.supervisor.Execute(*req.Step, req.Grant)
		d.prepared = map[string]desktopPreparation{}
		return result, err
	}
	return nil, computer.ErrInvalidControl
}
