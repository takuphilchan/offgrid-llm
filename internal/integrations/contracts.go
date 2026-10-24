package integrations

import (
	"fmt"
	"strings"

	"github.com/takuphilchan/offgrid-llm/internal/integrations/verification"
)

// Check is an observation, not an instruction to install or run anything.
type Check struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ServiceStatus struct {
	Profile
	Ready          bool     `json:"ready"`
	Status         string   `json:"status"`
	ModelID        string   `json:"model_id,omitempty"`
	ContextWindow  int      `json:"context_window"`
	Warnings       []string `json:"warnings"`
	ReadinessScope string   `json:"readiness_scope"`
	Checks         []Check  `json:"checks"`
	HostState      string   `json:"host_state"`
	WorkflowState  string   `json:"workflow_state"`
}

type ListResponse struct {
	Provider     string          `json:"provider"`
	BaseURL      string          `json:"base_url"`
	Integrations []ServiceStatus `json:"integrations"`
}

type SetupResponse struct {
	Integration ServiceStatus `json:"integration"`
	Setup       Setup         `json:"setup"`
}

// ContextCheck treats the authored window as an upper bound promised to the
// external client. Spare service capacity does not invalidate a smaller window.
func ContextCheck(configured, admitted, minimum int) Check {
	check := Check{ID: "context", State: "passed", Code: "context_compatible", Message: "Configured context fits the service allocation"}
	switch {
	case admitted < minimum:
		check.State, check.Code = "blocked", "context_insufficient"
		check.Message = fmt.Sprintf("OffGrid currently admits %d tokens; this integration requires %d. Check model and service capacity before running", admitted, minimum)
	case configured < minimum:
		check.State, check.Code = "blocked", "context_configuration_required"
		check.Message = fmt.Sprintf("Configured context is %d tokens; at least %d is required. Configure this model with an admissible context", configured, minimum)
	case configured > admitted:
		check.State, check.Code = "blocked", "context_capacity_reduced"
		check.Message = fmt.Sprintf("Configured context is %d tokens, but OffGrid currently admits %d. Restore capacity or explicitly reconfigure the context; reinstalling the runtime is unnecessary", configured, admitted)
	}
	return check
}

// HostOptions is shared by managed commands and compatibility aliases. Secrets
// are intentionally excluded from reports and persisted connection records.
type HostOptions struct {
	VerifyContract string
	Verification   *verification.Connection `json:"-"`
	ServerURL      string
	BaseURL        string
	ModelID        string
	APIKey         string
	Force          bool
	Yes            bool
	WithBrowser    bool
	RunDoctor      bool
	DryRun         bool
}

func ParseHostOptions(args []string, defaults HostOptions, passthrough bool) (HostOptions, []string, error) {
	o := defaults
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			for _, value := range args[i+1:] {
				if value == "--verify-contract" || strings.HasPrefix(value, "--verify-contract=") {
					return o, nil, fmt.Errorf("--verify-contract must precede the -- separator")
				}
			}
			if !passthrough && i+1 < len(args) {
				return o, nil, fmt.Errorf("unexpected positional arguments")
			}
			rest = append(rest, args[i+1:]...)
			break
		}
		name, inline, hasInline := strings.Cut(arg, "=")
		if hasInline && (name == "--server" || name == "--base-url" || name == "--model" || name == "--api-key" || name == "--verify-contract") {
			args = append(append(append([]string{}, args[:i]...), name, inline), args[i+1:]...)
			arg = name
		}
		switch arg {
		case "--server", "--base-url", "--model", "-m", "--api-key", "--verify-contract":
			i++
			if i >= len(args) || strings.TrimSpace(args[i]) == "" || strings.HasPrefix(args[i], "--") {
				return o, nil, fmt.Errorf("%s requires a value", arg)
			}
			value := strings.TrimSpace(args[i])
			switch arg {
			case "--verify-contract":
				if !passthrough || o.VerifyContract != "" {
					return o, nil, fmt.Errorf("--verify-contract requires one run invocation and may appear only once")
				}
				o.VerifyContract = value
			case "--server":
				o.ServerURL = strings.TrimRight(value, "/")
			case "--base-url":
				o.BaseURL = strings.TrimRight(value, "/")
			case "--model", "-m":
				o.ModelID = value
			case "--api-key":
				o.APIKey = value
			}
		case "--yes", "-y":
			o.Yes = true
		case "--force":
			o.Force = true
		case "--with-browser":
			o.WithBrowser = true
		case "--skip-browser":
			o.WithBrowser = false
		case "--doctor":
			o.RunDoctor = true
		case "--skip-doctor":
			o.RunDoctor = false
		case "--dry-run":
			o.DryRun = true
		default:
			if !passthrough {
				return o, nil, fmt.Errorf("unknown option: %s", arg)
			}
			rest = append(rest, arg)
		}
	}
	return o, rest, nil
}

func (o HostOptions) Endpoint() string {
	if o.BaseURL != "" {
		return openAIBaseURL(o.BaseURL)
	}
	return openAIBaseURL(o.ServerURL)
}
