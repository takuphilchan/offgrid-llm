package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/integrations/verification"
	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// HostAdapter contains only external-runtime operations. Manager owns ordering,
// selection, readiness, persistence, consent and evidence for every adapter.
type HostAdapter interface {
	Profile() Profile
	Inspect(context.Context, HostOptions) (HostState, error)
	InstallRuntime(context.Context, HostOptions) error
	InstallPlugin(context.Context, HostOptions, HostState) error
	Configure(context.Context, HostOptions, HostState, SetupResponse, *Transaction) error
	Smoke(context.Context, HostOptions, HostState, SetupResponse) error
	Run(context.Context, HostOptions, HostState, SetupResponse, []string) error
}

// Optional, revision-pinned hook support. Unknown adapters retain ordinary runs
// but cannot claim explicit outcome verification.
type VerificationAdapter interface {
	CheckVerification(context.Context, HostOptions, HostState, []string) (string, error)
}

type HostState struct {
	Installed     bool           `json:"installed"`
	Executable    string         `json:"executable,omitempty"`
	Version       string         `json:"version,omitempty"`
	Plugin        bool           `json:"plugin_loaded"`
	PluginPath    string         `json:"plugin_path,omitempty"`
	PluginDigest  string         `json:"plugin_digest,omitempty"`
	BaseURL       string         `json:"base_url,omitempty"`
	DefaultModel  string         `json:"default_model,omitempty"`
	Models        map[string]int `json:"models,omitempty"`
	ContextWindow int            `json:"context_window,omitempty"`
	ConfigError   string         `json:"config_error,omitempty"`
	ConfigFiles   []string       `json:"-"`
	ConfigDigest  string         `json:"config_digest,omitempty"`
	// Runtime-specific per-invocation overrides allow a different installed
	// model to run without rewriting the user's saved default.
	DynamicModels bool `json:"dynamic_models"`
}

type Evidence struct {
	State       string `json:"state"`
	Fingerprint string `json:"fingerprint,omitempty"`
	CheckedAt   string `json:"checked_at,omitempty"`
}

type HostReport struct {
	Verification    *verification.Receipt `json:"verification,omitempty"`
	ID              string                `json:"id"`
	Ready           bool                  `json:"ready"`
	Phase           string                `json:"phase"`
	ModelID         string                `json:"model_id,omitempty"`
	SelectionSource string                `json:"selection_source,omitempty"`
	Runtime         HostState             `json:"runtime"`
	Service         ServiceStatus         `json:"service"`
	Checks          []Check               `json:"checks"`
	Smoke           Evidence              `json:"smoke"`
	Workflow        Evidence              `json:"workflow"`
	RecoveryPath    string                `json:"recovery_path,omitempty"`
}

type SetupClient interface {
	Setup(context.Context, string, string, string) (SetupResponse, error)
}

type Manager struct {
	Adapters map[string]HostAdapter
	Service  SetupClient
	Store    *ConnectionStore
	Confirm  func(context.Context, string, string) (bool, error)
	Progress func(string)
}

type inspection struct {
	report     HostReport
	setup      SetupResponse
	connection Connection
	options    HostOptions
}

func (m *Manager) adapter(id string) (HostAdapter, error) {
	a, ok := m.Adapters[id]
	if !ok || a == nil {
		return nil, fmt.Errorf("unsupported managed integration %q", id)
	}
	if m.Service == nil || m.Store == nil {
		return nil, fmt.Errorf("integration manager dependencies unavailable")
	}
	return a, nil
}

func (m *Manager) inspect(ctx context.Context, id string, o HostOptions) (inspection, error) {
	r := inspection{report: HostReport{ID: id, Phase: "inspect", Checks: []Check{}, Smoke: Evidence{State: "unverified"}, Workflow: Evidence{State: "unverified"}}}
	a, err := m.adapter(id)
	if err != nil {
		return r, err
	}
	r.connection, err = m.Store.Load(id, o)
	if err != nil {
		return r, err
	}
	if o.BaseURL == "" && r.connection.BaseURL != "" {
		o.BaseURL = r.connection.BaseURL
	}
	if r.connection.BaseURL != "" && r.connection.BaseURL != o.Endpoint() {
		r.connection = Connection{Version: 1}
	}
	r.options = o
	host, hostErr := a.Inspect(ctx, o)
	r.report.Runtime = host
	selected := o.ModelID
	r.report.SelectionSource = "explicit"
	if selected == "" {
		selected, r.report.SelectionSource = r.connection.ModelID, "managed_connection"
	}
	if selected == "" {
		selected, r.report.SelectionSource = host.DefaultModel, "external_configuration"
	}
	if selected == "" {
		r.report.SelectionSource = "service_catalog"
	}
	r.setup, err = m.Service.Setup(ctx, id, selected, o.Endpoint())
	if err == nil && selected != "" && r.setup.Integration.ModelID != selected {
		err = fmt.Errorf("service returned a different model than the selected model; no execution was started")
	}
	r.report.Service = r.setup.Integration
	r.report.ModelID = r.setup.Integration.ModelID
	if r.report.ModelID == "" {
		r.report.ModelID = selected
	}
	add := func(id string, ok bool, code, message string) {
		state := "passed"
		if !ok {
			state = "blocked"
		}
		r.report.Checks = append(r.report.Checks, Check{ID: id, State: state, Code: code, Message: message})
	}
	add("service", err == nil, "service_available", "OffGrid service is reachable")
	if err != nil {
		r.report.Checks[len(r.report.Checks)-1].Code = "service_unavailable"
		r.report.Checks[len(r.report.Checks)-1].Message = err.Error()
	}
	add("runtime", host.Installed && hostErr == nil, "runtime_inspected", "External runtime is installed and its version is readable")
	if hostErr != nil {
		r.report.Checks[len(r.report.Checks)-1].Code = "runtime_inspection_failed"
		r.report.Checks[len(r.report.Checks)-1].Message = hostErr.Error()
	}
	add("plugin", host.Plugin, "plugin_required", "The installed OffGrid provider must match the trusted bundle; a model test verifies runtime loading")
	modelOK := err == nil && r.setup.Integration.Ready && r.setup.Integration.ContextWindow >= a.Profile().MinimumContext
	if err == nil {
		message := strings.Join(r.setup.Integration.Warnings, "; ")
		if message == "" {
			message = "Selected model must be installed and meet this integration's context requirements"
		}
		if r.setup.Integration.ContextWindow < a.Profile().MinimumContext {
			message = ContextCheck(0, r.setup.Integration.ContextWindow, a.Profile().MinimumContext).Message
		}
		add("model", modelOK, "model_compatibility", message)
	}
	configured := host.ConfigError == "" && strings.TrimRight(host.BaseURL, "/") == o.Endpoint()
	add("configuration", configured, "configuration_required", "External provider must point to the selected OffGrid endpoint")
	if host.ConfigError != "" {
		r.report.Checks[len(r.report.Checks)-1].Message = host.ConfigError
	}
	if modelOK {
		configuredContext := host.Models[r.report.ModelID]
		if configuredContext == 0 {
			configuredContext = host.ContextWindow
		}
		if host.DynamicModels && configuredContext == 0 {
			configuredContext = r.setup.Integration.ContextWindow
		}
		r.report.Checks = append(r.report.Checks, ContextCheck(configuredContext, r.setup.Integration.ContextWindow, a.Profile().MinimumContext))
	}
	r.report.Ready = true
	for _, c := range r.report.Checks {
		if c.State == "blocked" {
			r.report.Ready = false
		}
	}
	if r.report.Ready {
		r.report.Phase = "preflight_ready"
	}
	if r.connection.Smoke.State == "passed" {
		r.report.Smoke = r.connection.Smoke
		if !r.report.Ready || r.connection.ContractRevision != a.Profile().ContractRevision || r.connection.Smoke.Fingerprint != reportFingerprint(r.report, o) {
			r.report.Smoke.State = "stale"
		}
	}
	return r, nil
}

func (m *Manager) Status(ctx context.Context, id string, o HostOptions) (HostReport, error) {
	r, err := m.inspect(ctx, id, o)
	return r.report, err
}

func blockedReport(r HostReport) error {
	var reasons []string
	for _, c := range r.Checks {
		if c.State == "blocked" {
			reasons = append(reasons, c.Message)
		}
	}
	return fmt.Errorf("%s preflight blocked: %s", r.ID, strings.Join(reasons, "; "))
}

func reportFingerprint(r HostReport, o HostOptions) string {
	data, _ := json.Marshal(struct {
		Endpoint, Model, Version, Plugin, Config, Contract string
		Context                                            int
	}{
		o.Endpoint(), r.ModelID, r.Runtime.Version, r.Runtime.PluginDigest, r.Runtime.ConfigDigest, r.Service.ContractRevision, r.Service.ContextWindow})
	return contentDigest(data)
}

func (m *Manager) Test(ctx context.Context, id string, o HostOptions) (HostReport, error) {
	r, err := m.inspect(ctx, id, o)
	if err != nil {
		return r.report, err
	}
	if !r.report.Ready {
		return r.report, blockedReport(r.report)
	}
	a, _ := m.adapter(id)
	if err := m.prepare(ctx, id, &r.report); err != nil {
		return r.report, err
	}
	testCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()
	r.report.Phase = "smoke_test"
	o = r.options
	if err = a.Smoke(testCtx, o, r.report.Runtime, r.setup); err != nil {
		return r.report, err
	}
	// Do not certify a config that changed while inference was running.
	after, err := m.inspect(ctx, id, withModel(o, r.report.ModelID))
	if err != nil {
		return r.report, err
	}
	if !after.report.Ready || reportFingerprint(after.report, o) != reportFingerprint(r.report, o) {
		return after.report, fmt.Errorf("integration changed during the smoke test; inspect status before testing again")
	}
	r.report.Smoke = Evidence{State: "passed", Fingerprint: reportFingerprint(r.report, o), CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	if err = m.Store.RecordSmoke(id, o, r.report.Smoke, a.Profile().ContractRevision); err != nil {
		return r.report, err
	}
	r.report.Phase = "smoke_passed"
	return r.report, nil
}

func withModel(o HostOptions, id string) HostOptions { o.ModelID = id; return o }

// Warm the selected model before either smoke or full execution. A short
// auxiliary request from an external runtime must not own cold loading.
func (m *Manager) prepare(ctx context.Context, id string, report *HostReport) error {
	preparation, ok := m.Service.(interface {
		Prepare(context.Context, string) error
	})
	if !ok {
		return nil
	}
	report.Phase = "preparing_model"
	if m.Progress != nil {
		m.Progress(fmt.Sprintf("Preparing %s for %s; cold model loading can take time (Ctrl+C cancels)", report.ModelID, id))
	}
	if err := preparation.Prepare(ctx, report.ModelID); err != nil {
		var unavailable *serviceclient.Error
		if errors.As(err, &unavailable) && unavailable.Status == http.StatusNotFound {
			if m.Progress != nil {
				m.Progress("This service does not support model preparation; continuing with external-runtime loading")
			}
			return nil
		}
		report.Phase = "preparation_failed"
		if errors.Is(err, context.Canceled) {
			report.Phase = "cancelled"
		}
		if errors.Is(err, context.DeadlineExceeded) {
			report.Phase = "timed_out"
		}
		return err
	}
	return nil
}

func (m *Manager) Run(ctx context.Context, id string, o HostOptions, args []string) (HostReport, error) {
	r, err := m.inspect(ctx, id, o)
	if err != nil {
		return r.report, err
	}
	if !r.report.Ready {
		return r.report, blockedReport(r.report)
	}
	a, _ := m.adapter(id)
	var session *verification.Session
	if o.VerifyContract != "" {
		r.report.Phase = "verification_preflight"
		adapter, ok := a.(VerificationAdapter)
		if !ok {
			return r.report, fmt.Errorf("this integration does not support verification contracts")
		}
		revision, err := adapter.CheckVerification(ctx, r.options, r.report.Runtime, args)
		if err != nil {
			return r.report, err
		}
		contract, err := verification.LoadContract(o.VerifyContract)
		if err != nil {
			return r.report, err
		}
		session, err = verification.Start(ctx, contract, revision)
		if err != nil {
			return r.report, err
		}
		defer session.Close()
		connection := session.Connection()
		r.options.Verification = &connection
	}
	if err := m.prepare(ctx, id, &r.report); err != nil {
		return r.report, err
	}
	r.report.Phase = "running"
	if m.Progress != nil {
		m.Progress(fmt.Sprintf("Launching %s with %s; waiting for the external runtime", id, r.report.ModelID))
	}
	err = a.Run(ctx, r.options, r.report.Runtime, r.setup, args)
	switch {
	case errors.Is(err, context.Canceled):
		r.report.Phase = "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		r.report.Phase = "timed_out"
	case err != nil:
		r.report.Phase = "failed"
	default:
		r.report.Phase = "process_exited"
	}
	if session != nil {
		exit := "completed"
		if err != nil {
			exit = "failed"
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			exit = "cancelled"
		}
		receipt := session.Finish(ctx, exit)
		r.report.Verification = &receipt
		if receipt.State != "passed" {
			if err == nil {
				r.report.Phase = "verification_failed"
			}
			err = errors.Join(err, fmt.Errorf("verification %s: %s; inspect the workspace before retrying", receipt.State, receipt.Code))
		}
	}
	return r.report, err
}
