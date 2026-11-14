package agents

import (
	"fmt"
	"math"

	"github.com/takuphilchan/offgrid-llm/internal/execution"
)

// ProcessBudget is service-owned. Reservations survive loss of the start reply,
// host authority and client lifetime. Only typed host receipts settle them.
type ProcessBudget struct {
	Version              int                           `json:"version"`
	PerDecisionNS        int64                         `json:"per_decision_ns"`
	StartsPerDecision    int                           `json:"starts_per_decision"`
	ReservedStarts       int                           `json:"reserved_starts"`
	LegacyReservedStarts int                           `json:"legacy_reserved_starts,omitempty"`
	ChargedNS            int64                         `json:"charged_ns"`
	MeasuredNS           int64                         `json:"measured_ns"`
	UnsettledStarts      int                           `json:"unsettled_starts"`
	Reservations         map[string]ProcessReservation `json:"reservations,omitempty"`
}

type ProcessReservation struct {
	RunID      string              `json:"run_id"`
	Reference  execution.Reference `json:"reference"`
	ReservedNS int64               `json:"reserved_ns"`
	ChargedNS  int64               `json:"charged_ns"`
	MeasuredNS int64               `json:"measured_ns"`
	Handle     string              `json:"handle,omitempty"`
	Settled    bool                `json:"settled"`
}

func ensureProcessBudget(task *Task) error {
	cp := task.Checkpoint
	if cp.ProcessBudget != nil {
		b := cp.ProcessBudget
		if b.Version != 1 || b.PerDecisionNS <= 0 || b.StartsPerDecision < 1 || b.StartsPerDecision > maxToolsPerDecision {
			return fmt.Errorf("Unsupported pinned process budget")
		}
		return nil
	}
	unit, ok := toolBudgetUnit(max(int64(task.Config.TimeoutPerStep), int64(execution.DefaultProcessTimeout)), maxToolsPerDecision)
	prior := max(cp.Iteration, cp.BudgetedModelCalls)
	if !ok || task.Config.MaxIterations < 1 || task.Config.MaxIterations > math.MaxInt/maxToolsPerDecision || int64(task.Config.MaxIterations) > math.MaxInt64/unit || prior < 0 || prior > task.Config.MaxIterations {
		return fmt.Errorf("Process budget is unavailable; no command was dispatched")
	}
	starts := prior * maxToolsPerDecision
	cp.ProcessBudget = &ProcessBudget{Version: 1, PerDecisionNS: unit, StartsPerDecision: maxToolsPerDecision, ReservedStarts: starts, LegacyReservedStarts: starts, ChargedNS: int64(prior) * unit, UnsettledStarts: starts}
	return nil
}

func reserveProcessBudget(task *Task, request execution.Request) error {
	if request.Operation != "process_start" {
		return nil
	}
	spec, err := execution.ParseProcessSpec(request.Arguments)
	if err != nil {
		return err
	}
	if err = ensureProcessBudget(task); err != nil {
		return err
	}
	b := task.Checkpoint.ProcessBudget
	if _, exists := b.Reservations[request.OperationID]; exists {
		return ErrRunConflict
	}
	slots := task.Config.MaxIterations - task.Checkpoint.ReservedChildIterations
	charge := int64(execution.ProcessTimeout(spec))
	if slots < 1 || int64(slots) > math.MaxInt64/b.PerDecisionNS || b.ReservedStarts < 0 || b.ChargedNS < 0 || b.MeasuredNS < 0 || b.UnsettledStarts < 0 || int64(b.ReservedStarts) >= int64(slots)*int64(b.StartsPerDecision) || charge > int64(slots)*b.PerDecisionNS-b.ChargedNS {
		return fmt.Errorf("Aggregate process budget is exhausted; no command was dispatched")
	}
	if b.Reservations == nil {
		b.Reservations = map[string]ProcessReservation{}
	}
	b.Reservations[request.OperationID] = ProcessReservation{RunID: task.ID, Reference: *task.Config.Execution, ReservedNS: charge, ChargedNS: charge}
	b.ReservedStarts++
	b.UnsettledStarts++
	b.ChargedNS += charge
	return nil
}

func processBudgetDecisions(b *ProcessBudget) (int, bool) {
	if b == nil {
		return 0, true
	}
	if b.Version != 1 || b.PerDecisionNS <= 0 || b.StartsPerDecision < 1 || b.StartsPerDecision > maxToolsPerDecision || b.ReservedStarts < 0 || b.ChargedNS < 0 {
		return 0, false
	}
	timeSlots := b.ChargedNS/b.PerDecisionNS + int64(boolInt(b.ChargedNS%b.PerDecisionNS != 0))
	if timeSlots > int64(math.MaxInt) {
		return 0, false
	}
	return max(int(timeSlots), b.ReservedStarts/b.StartsPerDecision+boolInt(b.ReservedStarts%b.StartsPerDecision != 0)), true
}
