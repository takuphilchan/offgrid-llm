package computer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// DesktopProvider supplies OS-verified identities. A model never chooses a
// process by title/PID alone. Drivers without this interface remain v2-only.
type DesktopProvider interface {
	DesktopIdentity() (TargetIdentity, error)
	TargetExecutable(NativeTarget) (string, error)
}

type NativeApplication struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases,omitempty"`
}

// Aliases come only from additional OS registrations for the same executable;
// never derive them from model text or publish executable paths as aliases.
func addApplicationAlias(app *NativeApplication, name string) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 256 || strings.ContainsAny(name, "\\/\x00\r\n") || strings.EqualFold(app.Name, name) || len(app.Aliases) >= 8 {
		return
	}
	for _, alias := range app.Aliases {
		if strings.EqualFold(alias, name) {
			return
		}
	}
	app.Aliases = append(app.Aliases, name)
}

type installedApplication struct {
	NativeApplication
	path            string
	identity        os.FileInfo
	kind            applicationIdentityKind
	packageIdentity *packagedApplication
	// Supplied only by OS catalog construction; nil never validates a package.
	validateRegistration           func(packagedApplication) error
	validateExecutableRegistration func() error
}

// DesktopController separates task scope from the adapter's current target.
// It is owned by one native worker thread. Only NativeSupervisor dispatches its
// prepared lifecycle operations, after durable intent and policy checks.
type DesktopController struct {
	driver       NativeAdapter
	provider     DesktopProvider
	identity     TargetIdentity
	apps         map[string]installedApplication
	targets      map[string]NativeTarget
	observed     time.Time
	selected     NativeTarget
	selectedApp  string
	prepared     map[string]PreparedControlAction
	launch       func(context.Context, installedApplication) error
	discover     func() ([]installedApplication, error)
	activations  map[string]time.Time
	measurements *NativeMeasurements
}

// SetMeasurements enables local, text-free fixture timing on the owning worker.
// It does not publish metrics or change application permissions.
func (c *DesktopController) SetMeasurements(m *NativeMeasurements) { c.measurements = m }

func NewDesktopController(driver NativeAdapter) (*DesktopController, error) {
	provider, ok := driver.(DesktopProvider)
	if !ok {
		return nil, fmt.Errorf("computer_driver_unavailable")
	}
	identity, err := provider.DesktopIdentity()
	if err != nil {
		return nil, err
	}
	if identity.Validate() != nil || identity.Surface != "desktop" || identity.ProcessGeneration != identity.ID {
		return nil, ErrControlScope
	}
	apps, err := installedNativeApplications()
	if err != nil {
		return nil, err
	}
	c := &DesktopController{driver: driver, provider: provider, identity: identity, apps: map[string]installedApplication{}, targets: map[string]NativeTarget{}, prepared: map[string]PreparedControlAction{}, launch: launchNativeApplication, discover: installedNativeApplications}
	for _, app := range apps {
		c.apps[app.ID] = app
	}
	return c, nil
}

func (c *DesktopController) Identity() TargetIdentity { return c.identity }

func (c *DesktopController) Check() error {
	current, err := c.provider.DesktopIdentity()
	if err != nil || current != c.identity {
		return ErrControlScope
	}
	return nil
}

func (c *DesktopController) Applications() ([]NativeApplication, error) {
	started := time.Now()
	defer func() { c.measurements.Add("discovery", time.Since(started)) }()
	if err := c.Check(); err != nil {
		return nil, err
	}
	if c.discover != nil {
		found, err := c.discover()
		if err != nil {
			return nil, err
		}
		fresh := make(map[string]installedApplication, len(found))
		for _, app := range found {
			for id, previous := range c.apps {
				if checkInstalledApplication(previous) == nil && sameInstalledApplication(app, previous) {
					app.ID = id
					break
				}
			}
			fresh[app.ID] = app
		}
		for id, action := range c.prepared {
			if action.Operation.Kind == "open_application" {
				if _, exists := fresh[action.Operation.Application]; !exists {
					delete(c.prepared, id)
				}
			}
		}
		c.apps = fresh
	}
	apps := []NativeApplication{}
	for _, app := range c.apps {
		if checkInstalledApplication(app) == nil {
			entry := app.NativeApplication
			entry.Aliases = append([]string(nil), entry.Aliases...)
			apps = append(apps, entry)
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Name == apps[j].Name {
			return apps[i].ID < apps[j].ID
		}
		return apps[i].Name < apps[j].Name
	})
	return apps, nil
}

func cleanApplicationEnvironment(env []string) []string {
	result := []string{}
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "OFFGRID_") || strings.HasPrefix(key, "ELECTRON_") || key == "NODE_OPTIONS" || key == "NODE_PATH" || key == "PYTHONPATH" || key == "PYTHONHOME" || strings.HasPrefix(key, "LD_") || strings.HasPrefix(key, "DYLD_") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func checkInstalledApplication(app installedApplication) error {
	switch app.kind {
	case "", applicationExecutable:
		if app.packageIdentity != nil || app.validateRegistration != nil {
			return ErrControlScope
		}
		if app.kind == applicationExecutable && (app.validateExecutableRegistration == nil || app.validateExecutableRegistration() != nil) {
			return ErrControlScope
		}
	case applicationPackaged:
		if app.validateExecutableRegistration != nil || app.packageIdentity == nil || !app.packageIdentity.valid() || app.validateRegistration == nil || app.validateRegistration(*app.packageIdentity) != nil {
			return ErrControlScope
		}
	default:
		return ErrControlScope
	}
	info, err := os.Lstat(app.path)
	if err != nil || !info.Mode().IsRegular() || app.identity == nil || !os.SameFile(info, app.identity) || info.Size() != app.identity.Size() || !info.ModTime().Equal(app.identity.ModTime()) {
		return ErrControlScope
	}
	return validateInstalledApplicationPath(app.path)
}

func (c *DesktopController) applicationFor(target NativeTarget) string {
	if target.Identity.OSSession != c.identity.OSSession || target.Identity.Driver != c.identity.Driver {
		return ""
	}
	path, err := c.provider.TargetExecutable(target)
	if err != nil {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	for id, app := range c.apps {
		if app.identity == nil || !os.SameFile(info, app.identity) || checkInstalledApplication(app) != nil {
			continue
		}
		if app.kind == applicationPackaged {
			provider, ok := c.provider.(packagedDesktopProvider)
			if !ok {
				continue
			}
			current, err := provider.TargetApplication(target)
			if err != nil || current.AUMID != app.packageIdentity.AUMID || current.FullName != app.packageIdentity.FullName {
				continue
			}
		}
		return id
	}
	return ""
}

type DesktopSurface struct {
	NativeTarget
	Application string `json:"application_id"`
}

func (c *DesktopController) Surfaces() ([]DesktopSurface, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	c.prepared = map[string]PreparedControlAction{}
	c.targets = map[string]NativeTarget{}
	c.selected = NativeTarget{}
	c.selectedApp = ""
	targets, err := c.driver.Targets()
	if err != nil {
		return nil, err
	}
	result := []DesktopSurface{}
	for _, target := range targets {
		if id := c.applicationFor(target); id != "" {
			c.targets[target.Identity.ID] = target
			result = append(result, DesktopSurface{NativeTarget: target, Application: id})
		}
	}
	c.observed = time.Now()
	return result, nil
}

func (c *DesktopController) Binding(authority ControlBinding) (ControlBinding, error) {
	if c.Check() != nil || authority.Validate() != nil || authority.Target != c.identity {
		return ControlBinding{}, ErrControlScope
	}
	if c.selected == (NativeTarget{}) && c.selectedApp == "" {
		return ControlBinding{}, ErrTargetNotSelected
	}
	if c.selected.Identity.Validate() != nil || c.applicationFor(c.selected) != c.selectedApp {
		return ControlBinding{}, ErrControlScope
	}
	authority.Target = c.selected.Identity
	return authority, nil
}

func (c *DesktopController) Prepare(op Operation) (PreparedControlAction, error) {
	if op.Validate() != nil || c.Check() != nil {
		return PreparedControlAction{}, ErrInvalidControl
	}
	var identity, precondition string
	switch op.Kind {
	case "open_application":
		app, ok := c.apps[op.Application]
		if !ok {
			return PreparedControlAction{}, ErrInvalidControl
		}
		if err := checkInstalledApplication(app); err != nil {
			return PreparedControlAction{}, err
		}
		identity = app.ID
		precondition = applicationPrecondition(app)
	case "focus_surface":
		target, ok := c.targets[op.Target]
		if !ok || time.Since(c.observed) > 15*time.Second || c.applicationFor(target) == "" {
			return PreparedControlAction{}, ErrStaleObservation
		}
		identity = target.Identity.ID
		precondition = nativeHash(target.Identity)
	default:
		return PreparedControlAction{}, ErrInvalidControl
	}
	a := PreparedControlAction{ID: nativeID(), Operation: op, ControlIdentity: identity, Precondition: precondition, ExpectedChange: nativeHash(op), ApprovalClass: ActionReversible}
	c.prepared[a.ID] = a
	return a, nil
}

func (c *DesktopController) IsPrepared(a PreparedControlAction) bool {
	saved, ok := c.prepared[a.ID]
	return ok && nativeHash(saved) == nativeHash(a)
}

func (c *DesktopController) LifecycleBinding(authority ControlBinding, op Operation) (ControlBinding, error) {
	if op.Kind == "open_application" {
		return authority, nil
	}
	target, ok := c.targets[op.Target]
	if op.Kind != "focus_surface" || !ok || c.applicationFor(target) == "" {
		return ControlBinding{}, ErrControlScope
	}
	authority.Target = target.Identity
	return authority, nil
}

func (c *DesktopController) ApprovalSummary(a PreparedControlAction) (string, error) {
	if !c.IsPrepared(a) {
		return "", ErrControlApproval
	}
	if a.Operation.Kind == "open_application" {
		return "Open installed application: " + c.apps[a.Operation.Application].Name + "?", nil
	}
	if a.Operation.Kind == "focus_surface" {
		return "Focus the discovered application window?", nil
	}
	return "", ErrInvalidControl
}

func (c *DesktopController) Dispatch(ctx context.Context, a PreparedControlAction) (ControlResult, error) {
	fail := func(err error) (ControlResult, error) { return ControlResult{ActionID: a.ID, Outcome: "failed"}, err }
	if !c.IsPrepared(a) {
		return fail(ErrControlApproval)
	}
	delete(c.prepared, a.ID)
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := c.Check(); err != nil {
		return fail(err)
	}
	c.selected = NativeTarget{}
	c.selectedApp = ""
	switch a.Operation.Kind {
	case "open_application":
		app := c.apps[a.Operation.Application]
		if err := checkInstalledApplication(app); err != nil {
			return fail(err)
		}
		if applicationPrecondition(app) != a.Precondition {
			return fail(ErrStaleObservation)
		}
		if err := c.launch(ctx, app); err != nil {
			if errors.Is(err, ErrUncertain) {
				return ControlResult{ActionID: a.ID, Outcome: "uncertain", Uncertain: true}, err
			}
			return fail(err)
		}
		// A successful OS launch is only dispatch, not application readiness.
		if c.activations == nil {
			c.activations = map[string]time.Time{}
		}
		c.activations[app.ID] = time.Now()
		return ControlResult{ActionID: a.ID, Outcome: "dispatched"}, nil
	case "focus_surface":
		target, ok := c.targets[a.Operation.Target]
		if !ok || nativeHash(target.Identity) != a.Precondition || time.Since(c.observed) > 15*time.Second {
			return fail(ErrStaleObservation)
		}
		app := c.applicationFor(target)
		if app == "" {
			return fail(ErrControlScope)
		}
		selected, err := c.driver.Select(target.Identity.ID)
		if err != nil || selected.Identity != target.Identity {
			return fail(ErrControlScope)
		}
		if err := c.driver.FocusSelected(); err != nil {
			return ControlResult{ActionID: a.ID, Outcome: "uncertain", Uncertain: true}, ErrUncertain
		}
		c.selected = selected
		c.selectedApp = app
		return ControlResult{ActionID: a.ID, Outcome: "dispatched"}, nil
	}
	return fail(ErrInvalidControl)
}

type ApplicationEvidence struct {
	Verified    bool            `json:"verified"`
	Check       string          `json:"check"`
	Application string          `json:"application_id"`
	Readiness   string          `json:"readiness,omitempty"`
	Target      *TargetIdentity `json:"target,omitempty"`
	View        *NativeView     `json:"view,omitempty"`
}

// VerifyApplication is bounded read-only readiness/read-back. It never launches
// or focuses: retrying this check cannot repeat an uncertain mutation.
func (c *DesktopController) VerifyApplication(ctx context.Context, id string) (ApplicationEvidence, error) {
	started := time.Now()
	defer func() { c.measurements.Add("readiness", time.Since(started)) }()
	app, ok := c.apps[id]
	if !ok {
		return ApplicationEvidence{}, ErrInvalidControl
	}
	result := ApplicationEvidence{Check: "application_open", Application: id, Readiness: "starting"}
	if err := c.Check(); err != nil {
		return ApplicationEvidence{}, err
	}
	if checkInstalledApplication(app) != nil {
		result.Readiness = "unsupported_identity"
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			if err == context.Canceled {
				return ApplicationEvidence{}, err
			}
			if result.Readiness == "starting" {
				if activated := c.activations[id]; activated.IsZero() || time.Since(activated) > 30*time.Second {
					result.Readiness = "surface_unavailable"
				}
			}
			return result, nil
		}
		if checkInstalledApplication(app) != nil {
			result.Readiness = "unsupported_identity"
			return result, nil
		}
		surfaces, err := c.Surfaces()
		if err != nil {
			return ApplicationEvidence{}, err
		}
		for _, surface := range surfaces {
			if surface.Application != id {
				continue
			}
			selected, err := c.driver.Select(surface.Identity.ID)
			if err != nil {
				continue
			}
			view, err := c.driver.Observe(ctx)
			if err != nil || view.Observation.Validate(selected.Identity, time.Now()) != nil || c.applicationFor(selected) != id {
				result.Readiness = "observation_unavailable"
				continue
			}
			c.selected = selected
			c.selectedApp = id
			return ApplicationEvidence{Verified: true, Check: "application_open", Application: id, Readiness: "ready", Target: &selected.Identity, View: &view}, nil
		}
		timer := time.NewTimer(150 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
