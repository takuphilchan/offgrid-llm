package agents

import (
	"fmt"
	"math"
	"time"
)

const maxToolsPerDecision = 16

// ToolBudget charges service-observed tool invocation time. A process that
// outlives its start invocation requires separate host process accounting.
type ToolBudget struct {
	Version             int              `json:"version"`
	CallsPerDecision    int              `json:"calls_per_decision"`
	PerCallNS           int64            `json:"per_call_ns"`
	Attempts            int              `json:"attempts"`
	LegacyReservedCalls int              `json:"legacy_reserved_calls,omitempty"`
	ChargedNS           int64            `json:"charged_ns"`
	MeasuredNS          int64            `json:"measured_ns"`
	UnsettledCalls      int              `json:"unsettled_calls"`
	Pending             *ToolReservation `json:"pending,omitempty"`
	LastSettlement      *ToolSettlement  `json:"last_settlement,omitempty"`
}

type ToolReservation struct {
	Sequence int    `json:"sequence"`
	CallID   string `json:"call_id"`
}

type ToolSettlement struct {
	Sequence  int    `json:"sequence"`
	CallID    string `json:"call_id"`
	ElapsedNS int64  `json:"elapsed_ns"`
	Settled   bool   `json:"settled"`
}

func toolBudgetUnit(perCall int64, calls int) (int64, bool) {
	if perCall <= 0 || calls < 1 || perCall > math.MaxInt64/int64(calls) {
		return 0, false
	}
	return perCall * int64(calls), true
}

func ensureToolBudget(task *Task) error {
	cp := task.Checkpoint
	if cp.ToolBudget != nil {
		if cp.ToolBudget.Version != 1 || cp.ToolBudget.CallsPerDecision < 1 || cp.ToolBudget.CallsPerDecision > maxToolsPerDecision {
			return fmt.Errorf("Unsupported pinned tool runtime budget")
		}
		return nil
	}
	perCall := int64(task.Config.TimeoutPerStep)
	unit, ok := toolBudgetUnit(perCall, maxToolsPerDecision)
	prior := max(cp.Iteration, cp.BudgetedModelCalls)
	if !ok || task.Config.MaxIterations < 1 || task.Config.MaxIterations > math.MaxInt/maxToolsPerDecision || int64(task.Config.MaxIterations) > math.MaxInt64/unit || prior < 0 || prior > task.Config.MaxIterations {
		return fmt.Errorf("Tool runtime budget is unavailable; no operation was dispatched")
	}
	// Missing historical invocation ledgers are not evidence of zero usage.
	// Initialize before the first new model call, so new work starts at zero.
	attempts := prior * maxToolsPerDecision
	observed := 0
	for _, step := range task.Steps {
		if step.Type == "action" || step.Type == "tool_error" || step.Type == "reconciled" {
			observed++
		}
	}
	attempts = max(attempts, observed)
	if attempts > task.Config.MaxIterations*maxToolsPerDecision {
		return fmt.Errorf("Historical tool runtime allowance is exhausted")
	}
	cp.ToolBudget = &ToolBudget{Version: 1, CallsPerDecision: maxToolsPerDecision, PerCallNS: perCall, Attempts: attempts, LegacyReservedCalls: attempts, ChargedNS: int64(attempts) * perCall, UnsettledCalls: attempts}
	return nil
}

func reserveToolBudget(task *Task, callID string) error {
	if err := ensureToolBudget(task); err != nil {
		return err
	}
	b := task.Checkpoint.ToolBudget
	unit, ok := toolBudgetUnit(b.PerCallNS, b.CallsPerDecision)
	slots := task.Config.MaxIterations - task.Checkpoint.ReservedChildIterations
	if !ok || slots < 1 || int64(slots) > math.MaxInt64/unit || b.Attempts < 0 || b.ChargedNS < 0 || b.MeasuredNS < 0 || b.UnsettledCalls < 0 || int64(b.Attempts) >= int64(slots)*int64(b.CallsPerDecision) || b.ChargedNS > int64(slots)*unit-b.PerCallNS || callID == "" || task.Checkpoint.ExecutingCall != "" {
		return fmt.Errorf("Aggregate tool runtime budget is exhausted or unresolved; no operation was dispatched")
	}
	// A recovered read may be retried only after its old dispatch was cleared.
	// Its old unknown reservation stays charged; the retry is a new attempt.
	b.Attempts++
	b.ChargedNS += b.PerCallNS
	b.UnsettledCalls++
	b.Pending = &ToolReservation{Sequence: b.Attempts, CallID: callID}
	return nil
}

func settleToolBudget(task *Task, callID string, elapsed time.Duration, settled bool) {
	b := task.Checkpoint.ToolBudget
	if b == nil || b.Pending == nil || b.Pending.Sequence != b.Attempts || b.Pending.CallID != callID {
		return
	}
	measured := max(int64(elapsed), 0)
	if measured > math.MaxInt64-b.MeasuredNS {
		return
	}
	b.MeasuredNS += measured
	if measured > b.PerCallNS {
		// Report actual overruns even when a non-cooperative tool ignored its
		// deadline. Never advertise the timeout as a measured execution cap.
		if measured-b.PerCallNS > math.MaxInt64-b.ChargedNS {
			b.ChargedNS = math.MaxInt64
		} else {
			b.ChargedNS += measured - b.PerCallNS
		}
	} else if settled {
		b.ChargedNS -= b.PerCallNS - measured
	}
	if settled {
		b.UnsettledCalls--
	}
	b.LastSettlement = &ToolSettlement{Sequence: b.Pending.Sequence, CallID: callID, ElapsedNS: measured, Settled: settled}
	b.Pending = nil
}

// Merge only the receipt for the exact durable pending reservation. Lifecycle
// state, instructions and host receipts remain owned by the current snapshot.
func retainToolSettlement(current, proposed *Checkpoint) {
	if current == nil || proposed == nil || current.ToolBudget == nil || proposed.ToolBudget == nil {
		return
	}
	a, b := current.ToolBudget, proposed.ToolBudget
	if a.Pending == nil || b.LastSettlement == nil || b.Pending != nil || a.Attempts != b.Attempts || a.Version != b.Version || a.CallsPerDecision != b.CallsPerDecision || a.PerCallNS != b.PerCallNS || a.Pending.Sequence != b.LastSettlement.Sequence || a.Pending.CallID != b.LastSettlement.CallID {
		return
	}
	copy := *b
	current.ToolBudget = &copy
}
