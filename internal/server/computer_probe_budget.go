package server

import (
	"errors"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var errProbeOutputBudget = errors.New("synthetic model check exceeded or misreported its output token limit")

// A smoke check requests a small output ceiling. Some legacy runtimes omit
// usage, which remains unmeasured; reported overuse must never become a pass.
func checkSyntheticProbeUsage(response *api.ChatCompletionResponse, maxTokens int) error {
	if response == nil || maxTokens < 1 {
		return errProbeOutputBudget
	}
	usage := response.Usage
	if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 ||
		usage.CompletionTokens > maxTokens ||
		usage.TotalTokens > 0 && usage.PromptTokens > usage.TotalTokens-usage.CompletionTokens {
		return errProbeOutputBudget
	}
	return nil
}
