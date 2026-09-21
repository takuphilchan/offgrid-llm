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
	Protocol    int                          `json:"protocol"`
	ID          string                       `json:"id"`
	Kind        string                       `json:"kind"`
	Target      string                       `json:"target,omitempty"`
	Binding     *computer.ControlBinding     `json:"binding,omitempty"`
	Operation   *computer.Operation          `json:"operation,omitempty"`
	Observation *computer.ControlObservation `json:"observation,omitempty"`
	Step        *computer.BoundedStep        `json:"step,omitempty"`
	Grant       string                       `json:"grant,omitempty"`
}

func (r request) valid() bool {
	if r.Protocol != 2 || r.ID == "" || len(r.ID) > 128 {
		return false
	}
	fields := 0
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
	switch r.Kind {
	case "targets", "observe", "stop":
		return fields == 0
	case "select":
		return fields == 2 && r.Target != "" && r.Binding != nil
	case "prepare":
		return fields == 2 && r.Operation != nil && r.Observation != nil
	case "approve":
		return fields == 1 && r.Step != nil
	case "execute":
		return fields == 2 && r.Step != nil && r.Grant != ""
	}
	return false
}
func main() {
	if err := run(); err != nil {
		computer.WriteControlFrame(os.Stdout, map[string]any{"protocol": 2, "id": "startup", "error": nativeErrorCode(err), "result": nil})
		fmt.Fprintln(os.Stderr, "OffGrid native worker stopped:", err)
		os.Exit(1)
	}
}
func run() error {
	directory := flag.String("state", "", "host dispatch-journal directory (not a service workspace)")
	flag.Parse()
	if *directory == "" || flag.NArg() != 0 {
		return fmt.Errorf("use --state with an exclusive host journal directory; transport is private framed stdin/stdout")
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
	stopSurface, err := startNativeStopSurface(cancel)
	if err != nil {
		return err
	}
	defer stopSurface.Close()
	if ctx.Err() != nil {
		return nil
	}
	if err := computer.WriteControlFrame(os.Stdout, map[string]any{"protocol": 2, "id": "startup", "result": map[string]bool{"ready": true}}); err != nil {
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
			if req.valid() && req.Kind == "stop" {
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
	var selected computer.NativeTarget
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
		if !req.valid() {
			callErr = computer.ErrInvalidControl
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
				binding.Target = selected.Identity
				if callErr = binding.Validate(); callErr != nil {
					break
				}
				if !nativeConfirm("Allow OffGrid to inspect this selected application for ten minutes?\n\n" + selected.Title + "\n\nChanges require a separate exact-action approval. Other applications, marked password controls, elevated windows and the whole desktop are excluded. Sensitive text in ordinary controls cannot always be recognized. OffGrid may focus this application. Use the local Stop window or emergency shortcut to stop.") {
					callErr = computer.ErrControlApproval
					break
				}
				now := time.Now()
				supervisor = computer.NewNativeSupervisor(journal, computer.LocalConsent{ID: fmt.Sprintf("consent:%d", now.UnixNano()), Binding: binding, IssuedAt: now, ExpiresAt: now.Add(10 * time.Minute), Active: true}, func(ctx context.Context, action computer.PreparedControlAction) (computer.ControlResult, error) {
					if err := ctx.Err(); err != nil {
						return computer.ControlResult{ActionID: action.ID, Outcome: "failed"}, err
					}
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
				result, callErr = driver.Observe(ctx)
				prepared = map[string]computer.PreparedControlAction{}
			case "prepare":
				if supervisor == nil {
					callErr = computer.ErrControlScope
					break
				}
				var action computer.PreparedControlAction
				action, callErr = driver.Prepare(binding, *req.Operation, *req.Observation)
				if callErr == nil {
					prepared[action.ID] = action
					result = action
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
					detail, err := driver.ApprovalSummary(step)
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
		response := map[string]any{"protocol": 2, "id": req.ID, "result": result}
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
	case errors.Is(err, computer.ErrControlApproval):
		return "computer_approval_invalid"
	case errors.Is(err, computer.ErrInvalidControl):
		return "computer_invalid_action"
	case errors.Is(err, computer.ErrStaleObservation):
		return "computer_stale_observation"
	}
	if strings.HasPrefix(err.Error(), "computer_") {
		for _, code := range []string{"computer_journal_corrupt", "computer_journal_incompatible", "computer_action_failed", "computer_permission_denied", "computer_desktop_unavailable", "computer_driver_unavailable", "computer_stop_surface_unavailable", "computer_emergency_shortcut_in_use", "computer_input_in_use"} {
			if err.Error() == code {
				return code
			}
		}
	}
	return "computer_provider_unavailable"
}
