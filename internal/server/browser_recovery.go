package server

import (
	"errors"

	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/internal/computer"
)

func (b *browserRunTools) browserDiscoveryAvailable(task *agents.Task) bool {
	if b.server.browserHub == nil {
		return false
	}
	features, err := b.server.browserHub.BrowserFeatures(task.Actor, task.Config.ComputerSession)
	return err == nil && features.Discovery == 1
}

func (b *browserRunTools) browserWorkspaceAvailable(task *agents.Task) bool {
	if b.server.browserHub == nil {
		return false
	}
	features, err := b.server.browserHub.BrowserFeatures(task.Actor, task.Config.ComputerSession)
	return err == nil && features.Workspace == 1
}

func (b *browserRunTools) browserStructuredAvailable(task *agents.Task) bool {
	if b.server.browserHub == nil {
		return false
	}
	features, err := b.server.browserHub.BrowserFeatures(task.Actor, task.Config.ComputerSession)
	return err == nil && features.Structured == 1
}

func browserFailure(name string, err error) *agents.ToolFailure {
	d, ok := browserDescriptor(name)
	if !ok || !capabilities.TrustedBrowserOperation(d) {
		return nil
	}
	code := ""
	switch {
	case errors.Is(err, computer.ErrBrowserPrecondition):
		code = "computer_browser_precondition_failed"
	case errors.Is(err, computer.ErrBrowserObservation) && d.Risk == capabilities.RiskLow:
		code = "computer_browser_observation_failed"
	default:
		return nil
	}
	return &agents.ToolFailure{Code: code, Cause: err, BeforeEffects: true,
		Message:  "The browser operation failed without dispatching input.",
		Recovery: "Inspect the existing page using browser_observe, then use only its current references within approved scope. Do not replay earlier dispatched input or claim its outcome without verification."}
}
