// offgrid-computer is an unprivileged, inherited-pipe host worker. It has no
// TCP listener or raw command execution route. Native provider calls remain
// isolated from the OffGrid service and can be terminated by the owning shell.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

type request struct {
	Protocol     int                           `json:"protocol"`
	ID           string                        `json:"id"`
	Kind         string                        `json:"kind"`
	Target       string                        `json:"target,omitempty"`
	Binding      *computer.ControlBinding      `json:"binding,omitempty"`
	Operation    *computer.Operation           `json:"operation,omitempty"`
	Observation  *computer.ControlObservation  `json:"observation,omitempty"`
	Step         *computer.BoundedStep         `json:"step,omitempty"`
	Grant        string                        `json:"grant,omitempty"`
	ApprovalMode computer.ApprovalMode         `json:"approval_mode,omitempty"`
	Automatic    *bool                         `json:"automatic,omitempty"`
	Desktop      *computer.DesktopAuthority    `json:"desktop,omitempty"`
	Prompt       string                        `json:"prompt,omitempty"`
	Application  string                        `json:"application,omitempty"`
	Permission   *computer.DesktopPermission   `json:"permission,omitempty"`
	Service      string                        `json:"service,omitempty"`
	Page         *computer.DocumentPageRequest `json:"page,omitempty"`
}

func (r request) valid() bool {
	if (r.Protocol != 2 && r.Protocol != 3) || r.ID == "" || len(r.ID) > 128 {
		return false
	}
	fields := 0
	if r.Page != nil {
		fields++
	}
	if r.Permission != nil {
		fields++
	}
	if r.Service != "" {
		fields++
	}
	if r.Target != "" {
		fields++
	}
	if r.Binding != nil {
		fields++
	}
	if r.Operation != nil {
		fields++
	}
	if r.Observation != nil {
		fields++
	}
	if r.Step != nil {
		fields++
	}
	if r.Grant != "" {
		fields++
	}
	if r.ApprovalMode != "" {
		fields++
	}
	if r.Automatic != nil {
		fields++
	}
	if r.Desktop != nil {
		fields++
	}
	if r.Prompt != "" {
		fields++
	}
	if r.Application != "" {
		fields++
	}
	if r.Protocol == 3 {
		switch r.Kind {
		case "enable_desktop":
			return fields == 1 && r.Permission != nil && r.Permission.Validate() == nil
		case "release_desktop":
			return fields == 0
		case "desktop_identity", "applications", "surfaces":
			return fields == 0
		case "grant_desktop":
			return (fields == 3 || fields == 4 && r.Service != "") && r.Permission == nil && r.Desktop != nil && r.ApprovalMode.Valid() && r.Prompt != "" && len(r.Prompt) <= 32000
		case "prepare_lifecycle":
			return fields == 1 && r.Operation != nil
		case "verify_application":
			return fields == 1 && r.Application != ""
		case "select", "targets":
			return false
		}
	}
	switch r.Kind {
	case "targets", "observe", "documents", "stop":
		return fields == 0
	case "read_document":
		return r.Observation != nil && (fields == 1 && r.Page == nil || fields == 2 && r.Page != nil && r.Page.Validate() == nil)
	case "select":
		return fields == 3 && r.Target != "" && r.Binding != nil && r.ApprovalMode.Valid()
	case "prepare", "verify":
		return fields == 2 && r.Operation != nil && r.Observation != nil
	case "approve":
		return fields == 2 && r.Step != nil && r.Automatic != nil
	case "execute":
		return fields == 2 && r.Step != nil && r.Grant != ""
	}
	return false
}
func main() {
	if err := run(); err != nil {
		protocol := 2
		if f := flag.Lookup("protocol"); f != nil && f.Value.String() == "3" {
			protocol = 3
		}
		computer.WriteControlFrame(os.Stdout, map[string]any{"protocol": protocol, "id": "startup", "error": nativeErrorCode(err), "result": nil})
		fmt.Fprintln(os.Stderr, "OffGrid native worker stopped:", err)
		os.Exit(1)
	}
}
func run() error {
	directory := flag.String("state", "", "host dispatch-journal directory (not a service workspace)")
	protocol := flag.Int("protocol", 2, "private host protocol (2 or 3)")
	stopControl := flag.String("stop-control", "window", "Stop control owner (window or app); not an authorization")
	flag.Parse()
	if *directory == "" || flag.NArg() != 0 || (*protocol != 2 && *protocol != 3) {
		return fmt.Errorf("use --state with an exclusive host journal directory; transport is private framed stdin/stdout")
	}
	if *stopControl != "window" && (*stopControl != "app" || runtime.GOOS != "windows") {
		return fmt.Errorf("computer_invalid_action")
	}
	root, err := filepath.Abs(*directory)
	if err != nil {
		return err
	}
	owner, err := acquireNativeInputOwner()
	if err != nil {
		return err
	}
	defer owner()
	journal, err := computer.OpenNativeJournal(root)
	if err != nil {
		return err
	}
	defer journal.Close()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	driver, err := newNativeDriver()
	if err != nil {
		return err
	}
	defer driver.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	var active atomic.Pointer[computer.NativeSupervisor]
	// This watchdog is independent of the provider's MTA thread. If COM or a
	// consent dialog stalls, terminate only this owned worker. An executing
	// journal claim survives and cannot be mistaken for a safe retry.
	go func() {
		<-ctx.Done()
		if current := active.Load(); current != nil {
			current.Revoke()
		}
		time.AfterFunc(2*time.Second, func() { os.Exit(2) })
	}()
	stopSurface, err := startNativeStopSurface(cancel, *stopControl == "app")
	if err != nil {
		return err
	}
	defer stopSurface.Close()
	if ctx.Err() != nil {
		return nil
	}
	var desktop *desktopWorker
	if *protocol == 3 {
		controller, err := computer.NewDesktopController(driver)
		if err != nil {
			return err
		}
		desktop = &desktopWorker{driver: driver, controller: controller, journal: journal, ctx: ctx, active: &active}
		defer desktop.stop()
	}
	if err := computer.WriteControlFrame(os.Stdout, map[string]any{"protocol": *protocol, "id": "startup", "result": map[string]any{"ready": true, "native_features": computer.ProviderNativeFeatures(driver)}}); err != nil {
		return err
	}
	requests := make(chan request)
	readErrors := make(chan error, 1)
	go func() {
		for {
			var req request
			if err := computer.ReadControlFrame(os.Stdin, &req); err != nil {
				readErrors <- err
				cancel()
				return
			}
			if req.Protocol == *protocol && req.valid() && req.Kind == "stop" {
				cancel()
				return
			}
			select {
			case requests <- req:
			case <-ctx.Done():
				return
			}
		}
	}()
	var supervisor *computer.NativeSupervisor
	defer func() {
		if supervisor != nil {
			supervisor.Stop()
		}
	}()
	var binding computer.ControlBinding
	var approvalMode computer.ApprovalMode
	var selected computer.NativeTarget
	documents := newDocumentWorker(driver)
	prepared := map[string]computer.PreparedControlAction{}
	for {
		var req request
		select {
		case <-ctx.Done():
			select {
			case err := <-readErrors:
				if err != io.EOF {
					return err
				}
			default:
			}
			return nil
		case err := <-readErrors:
			if err == io.EOF {
				return nil
			}
			return err
		case req = <-requests:
		}
		if ctx.Err() != nil {
			return nil
		}
		var result any
		var callErr error
		if !req.valid() || req.Protocol != *protocol {
			callErr = computer.ErrInvalidControl
		} else if desktop != nil {
			result, callErr = desktop.call(req)
		} else {
			switch req.Kind {
			case "targets":
				if supervisor != nil {
					callErr = computer.ErrControlScope
				} else {
					result, callErr = driver.Targets()
				}
			case "select":
				if supervisor != nil {
					callErr = computer.ErrControlScope
					break
				}
				selected, callErr = driver.Select(req.Target)
				if callErr != nil {
					break
				}
				binding = *req.Binding
				approvalMode = req.ApprovalMode
				binding.Target = selected.Identity
				if callErr = binding.Validate(); callErr != nil {
					break
				}
				policyText := map[computer.ApprovalMode]string{computer.ApprovalAskEveryTime: "Every change requires approval.", computer.ApprovalScopedChanges: "Reversible changes run automatically; consequential actions require approval.", computer.ApprovalFullTask: "Available typed actions run automatically inside this application."}[approvalMode]
				if !nativeConfirm("Allow OffGrid to inspect this selected application for ten minutes?\n\n" + selected.Title + "\n\n" + policyText + " Credentials, payments, privilege/security changes, installation, permanent deletion and uncertain retries remain blocked. Other applications, protected controls, elevated windows and the whole desktop are excluded. OffGrid may focus this application. Use Stop in OffGrid or the emergency shortcut to stop.") {
					callErr = computer.ErrControlApproval
					break
				}
				now := time.Now()
				supervisor = computer.NewNativeSupervisor(journal, computer.LocalConsent{ID: fmt.Sprintf("consent:%d", now.UnixNano()), Binding: binding, ApprovalMode: approvalMode, IssuedAt: now, ExpiresAt: now.Add(10 * time.Minute), Active: true}, func(ctx context.Context, action computer.PreparedControlAction) (computer.ControlResult, error) {
					if err := ctx.Err(); err != nil {
						return computer.ControlResult{ActionID: action.ID, Outcome: "failed"}, err
					}
					if action.Operation.Kind == "document" {
						return documents.dispatch(ctx, action)
					}
					documents.clear()
					if err := driver.FocusSelected(); err != nil {
						return computer.ControlResult{ActionID: action.ID, Outcome: "failed"}, err
					}
					return driver.Dispatch(ctx, action)
				})
				active.Store(supervisor)
				if ctx.Err() != nil {
					supervisor.Revoke()
					return nil
				}
				result = selected
			case "observe":
				if supervisor == nil {
					callErr = computer.ErrControlScope
					break
				}
				if callErr = supervisor.Check(); callErr != nil {
					break
				}
				documents.clear()
				result, callErr = driver.Observe(ctx)
				prepared = map[string]computer.PreparedControlAction{}
			case "documents", "read_document":
				if supervisor == nil {
					callErr = computer.ErrControlScope
					break
				}
				if callErr = supervisor.Check(); callErr != nil {
					break
				}
				prepared = map[string]computer.PreparedControlAction{}
				if req.Kind == "documents" {
					result, callErr = documents.documents(ctx, binding)
				} else {
					result, callErr = documents.read(ctx, binding, *req.Observation, req.Page)
				}
			case "prepare":
				if supervisor == nil {
					callErr = computer.ErrControlScope
					break
				}
				if callErr = supervisor.Check(); callErr != nil {
					break
				}
				var action computer.PreparedControlAction
				if req.Operation.Kind == "document" {
					action, callErr = documents.prepare(ctx, binding, *req.Operation, *req.Observation)
				} else {
					documents.clear()
					action, callErr = driver.Prepare(binding, *req.Operation, *req.Observation)
				}
				if callErr == nil {
					prepared[action.ID] = action
					result = action
				}
			case "verify":
				if supervisor == nil {
					callErr = computer.ErrControlScope
					break
				}
				if callErr = supervisor.Check(); callErr != nil {
					break
				}
				if (req.Operation.Kind != "replace_text" && req.Operation.Kind != "set_checked") || req.Operation.Validate() != nil {
					callErr = computer.ErrInvalidControl
					break
				}
				verifier, ok := driver.(computer.NativeVerifier)
				if !ok {
					callErr = computer.ErrInvalidControl
					break
				}
				var verified bool
				if req.Operation.Kind == "replace_text" {
					verified, callErr = verifier.VerifyText(binding, req.Operation.Element, *req.Operation.Text, *req.Observation)
					if callErr == nil {
						result = map[string]any{"verified": verified, "check": "control_text_equals", "element": req.Operation.Element, "text": *req.Operation.Text, "observation_id": req.Observation.ID, "target": binding.Target}
					}
				} else {
					verified, callErr = verifier.VerifyChecked(binding, req.Operation.Element, *req.Operation.Checked, *req.Observation)
					if callErr == nil {
						result = map[string]any{"verified": verified, "check": "control_checked_equals", "element": req.Operation.Element, "checked": *req.Operation.Checked, "observation_id": req.Observation.ID, "target": binding.Target}
					}
				}
			case "approve":
				if supervisor == nil || req.Step.Binding != binding {
					callErr = computer.ErrControlScope
					break
				}
				for _, action := range req.Step.Actions {
					saved, ok := prepared[action.ID]
					a, _ := json.Marshal(saved)
					b, _ := json.Marshal(action)
					if !ok || string(a) != string(b) {
						callErr = computer.ErrControlApproval
						break
					}
				}
				if callErr != nil {
					break
				}
				result, callErr = supervisor.Approve(*req.Step, func(step computer.BoundedStep) bool {
					for _, action := range step.Actions {
						if !action.ApprovalClass.Valid() || action.ApprovalClass == computer.ActionForbidden {
							return false
						}
						if *req.Automatic && !approvalMode.Allows(action.ApprovalClass) {
							return false
						}
					}
					if *req.Automatic {
						return true
					}
					var detail string
					var err error
					if len(step.Actions) == 1 && step.Actions[0].Operation.Kind == "document" {
						detail, err = documents.summary(step)
					} else {
						detail, err = driver.ApprovalSummary(step)
					}
					return err == nil && nativeConfirm(detail)
				})
			case "execute":
				if supervisor == nil {
					callErr = computer.ErrControlScope
					break
				}
				// Focus is constrained to the application disclosed in the local prompt.
				// The supervisor still authenticates the stored grant before dispatch.
				result, callErr = supervisor.Execute(*req.Step, req.Grant)
			case "stop":
				if supervisor != nil {
					supervisor.Stop()
				}
				result = map[string]bool{"stopped": true}
			}
		}
		response := map[string]any{"protocol": *protocol, "id": req.ID, "result": result}
		if callErr != nil {
			response["error"] = nativeErrorCode(callErr)
		}
		if err := computer.WriteControlFrame(os.Stdout, response); err != nil {
			return err
		}
		if req.Kind == "stop" {
			return nil
		}
	}
}

func nativeErrorCode(err error) string {
	switch {
	case errors.Is(err, computer.ErrUncertain):
		return "computer_uncertain_outcome"
	case errors.Is(err, context.Canceled):
		return "computer_cancelled"
	case errors.Is(err, computer.ErrControlScope):
		return "computer_scope_violation"
	case errors.Is(err, computer.ErrTargetNotSelected):
		return "computer_target_not_selected"
	case errors.Is(err, computer.ErrApplicationNotReady):
		return "computer_application_not_ready"
	case errors.Is(err, computer.ErrDocumentChanged):
		return "computer_document_changed"
	case errors.Is(err, computer.ErrDocumentUnavailable):
		return "computer_document_unavailable"
	case errors.Is(err, computer.ErrDocumentIncomplete):
		return "computer_document_incomplete"
	case errors.Is(err, computer.ErrDocumentRange):
		return "computer_document_range_invalid"
	case errors.Is(err, computer.ErrDocumentComposition):
		return "computer_document_composition_unstable"
	case errors.Is(err, computer.ErrControlApproval):
		return "computer_approval_invalid"
	case errors.Is(err, computer.ErrInvalidControl):
		return "computer_invalid_action"
	case errors.Is(err, computer.ErrStaleObservation):
		return "computer_stale_observation"
	case errors.Is(err, computer.ErrProhibitedAction):
		return "computer_prohibited_action"
	}
	if strings.HasPrefix(err.Error(), "computer_") {
		for _, code := range []string{"computer_journal_corrupt", "computer_journal_incompatible", "computer_action_failed", "computer_permission_denied", "computer_desktop_unavailable", "computer_driver_unavailable", "computer_stop_surface_unavailable", "computer_emergency_shortcut_in_use", "computer_input_in_use", "computer_prohibited_action"} {
			if err.Error() == code {
				return code
			}
		}
	}
	return "computer_provider_unavailable"
}
