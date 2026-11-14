package server

import (
	"github.com/takuphilchan/offgrid-llm/internal/agents"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func taskResourceUsage(task *agents.Task) *api.TaskResourceUsage {
	cp := task.Checkpoint
	if cp == nil {
		return nil
	}
	usage := &api.TaskResourceUsage{
		ModelCallLimit: task.Config.MaxIterations, ModelCalls: cp.BudgetedModelCalls, ChildCallAllocations: cp.ReservedChildIterations,
		ChargedOutputTokens: cp.ChargedCompletionTokens, MeasuredOutputTokens: cp.MeasuredCompletionTokens,
		ChargedInferenceNS: cp.ChargedInferenceNS, MeasuredInferenceNS: cp.MeasuredInferenceNS,
		ContextPreparations: cp.ContextPreparations, MeasuredContextPreparationNS: cp.MeasuredContextPreparationNS,
	}
	if input := cp.InputBudget; input != nil {
		usage.Input = &api.TaskInputUsage{Window: input.Window, ChargedTokens: input.ChargedTokens, MeasuredTokens: input.MeasuredTokens, UnmeasuredCalls: input.UnmeasuredCalls}
	}
	if tools := cp.ToolBudget; tools != nil {
		usage.Tools = &api.TaskToolUsage{Version: tools.Version, CallsPerDecision: tools.CallsPerDecision, PerCallNS: tools.PerCallNS, ReservedCalls: tools.Attempts, LegacyReservedCalls: tools.LegacyReservedCalls, ChargedNS: tools.ChargedNS, MeasuredNS: tools.MeasuredNS, UnsettledCalls: tools.UnsettledCalls}
	}
	if processes := cp.ProcessBudget; processes != nil {
		usage.Processes = &api.TaskProcessUsage{Version: processes.Version, PerDecisionNS: processes.PerDecisionNS, StartsPerDecision: processes.StartsPerDecision, ReservedStarts: processes.ReservedStarts, LegacyReservedStarts: processes.LegacyReservedStarts, ChargedNS: processes.ChargedNS, MeasuredNS: processes.MeasuredNS, UnsettledStarts: processes.UnsettledStarts}
	}
	if storage := cp.StorageBudget; storage != nil {
		usage.Storage = &api.TaskStorageUsage{Version: storage.Version, LimitBytes: storage.LimitBytes, ChargedBytes: storage.ChargedBytes}
		for _, copy := range storage.Reservations {
			if copy.State == "released" {
				continue
			}
			usage.Storage.RetainedCopies++
			if copy.UsageMeasured {
				usage.Storage.MeasuredBytes += copy.MeasuredBytes
			} else {
				usage.Storage.UnmeasuredCopies++
			}
		}
	}
	return usage
}
