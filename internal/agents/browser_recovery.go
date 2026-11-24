package agents

import "github.com/takuphilchan/offgrid-llm/internal/capabilities"

func trustedBrowserRepair(d capabilities.Descriptor, failure *ToolFailure) bool {
	if !capabilities.TrustedBrowserOperation(d) || !failure.BeforeEffects {
		return false
	}
	return failure.Code == "computer_browser_precondition_failed" ||
		(failure.Code == "computer_browser_observation_failed" && d.Risk == capabilities.RiskLow)
}
