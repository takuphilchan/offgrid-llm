package agents

import (
	"fmt"
	"math"
	"time"
)

// InputBudget separates measured prompt usage from conservatively charged
// capacity. The ceiling is pinned; retries and work attempts never widen it.
type InputBudget struct {
	Window          int   `json:"window"`
	ModelCalls      int   `json:"model_calls"`
	ProbeCalls      int   `json:"probe_calls"`
	LastSettledCall int   `json:"last_settled_call"`
	ChargedTokens   int64 `json:"charged_tokens"`
	MeasuredTokens  int64 `json:"measured_tokens"`
	UnmeasuredCalls int   `json:"unmeasured_calls"`
}

func (r *Runner) pinInputContext(task *Task) {
	if task.Checkpoint.InputBudget != nil {
		task.Config.ContextWindow = task.Checkpoint.InputBudget.Window
	} else if task.Config.ContextWindow == 0 && r.ContextWindow != nil {
		task.Config.ContextWindow = r.ContextWindow(task)
	}
}

func reserveInputBudget(task *Task, probe bool) error {
	cp := task.Checkpoint
	window := task.Config.ContextWindow
	if cp.InputBudget != nil {
		window = cp.InputBudget.Window
	} else if window == 0 {
		window = 4096 // Same conservative fallback as context preparation.
	}
	slots := task.Config.MaxIterations - cp.ReservedChildIterations
	if window <= 0 || slots < 1 || int64(slots) > math.MaxInt64/int64(window) {
		return fmt.Errorf("Task input-token budget is unavailable; no inference was dispatched")
	}
	var next InputBudget
	if cp.InputBudget == nil {
		prior := max(cp.Iteration, cp.BudgetedModelCalls)
		if prior < 0 || cp.SyntheticProbeCalls < 0 || prior > slots || cp.SyntheticProbeCalls > slots-prior {
			return fmt.Errorf("Task input-token budget is exhausted; no inference was dispatched")
		}
		next = InputBudget{Window: window, ModelCalls: prior, ProbeCalls: cp.SyntheticProbeCalls, ChargedTokens: int64(prior+cp.SyntheticProbeCalls) * int64(window), UnmeasuredCalls: prior + cp.SyntheticProbeCalls}
	} else {
		next = *cp.InputBudget
	}
	if next.ModelCalls != max(cp.Iteration, cp.BudgetedModelCalls) || next.ProbeCalls != cp.SyntheticProbeCalls || next.UnmeasuredCalls < 0 || next.ChargedTokens < 0 || next.MeasuredTokens < 0 || next.MeasuredTokens > next.ChargedTokens || next.ChargedTokens > int64(slots-1)*int64(window) {
		return fmt.Errorf("Task input-token budget is exhausted or inconsistent; no inference was dispatched")
	}
	next.ChargedTokens += int64(window)
	next.UnmeasuredCalls++
	if probe {
		next.ProbeCalls++
	} else {
		next.ModelCalls++
	}
	cp.InputBudget = &next
	task.Config.ContextWindow = window
	return nil
}

func validateInputUsage(task *Task, promptTokens int) error {
	b := task.Checkpoint.InputBudget
	if b == nil {
		return nil
	} // Historical callers retain their prior contract.
	if b.Window < 1 || promptTokens > b.Window || b.UnmeasuredCalls < 1 || b.ModelCalls != task.Checkpoint.BudgetedModelCalls || b.ModelCalls <= b.LastSettledCall || b.ChargedTokens < int64(b.Window) || b.MeasuredTokens < 0 || int64(promptTokens) > math.MaxInt64-b.MeasuredTokens {
		return fmt.Errorf("Model input usage exceeds its pinned allocation or has already settled; no action was executed")
	}
	return nil
}

func settleInputUsage(task *Task, promptTokens int) {
	b := task.Checkpoint.InputBudget
	if b == nil {
		return
	}
	b.LastSettledCall = b.ModelCalls
	if promptTokens == 0 {
		return
	}
	b.UnmeasuredCalls--
	b.MeasuredTokens += int64(promptTokens)
	b.ChargedTokens -= int64(b.Window - promptTokens)
}

func recordContextPreparation(task *Task, elapsed time.Duration) {
	cp := task.Checkpoint
	cp.ContextPreparations++
	if elapsed >= 0 && int64(elapsed) <= math.MaxInt64-cp.MeasuredContextPreparationNS {
		cp.MeasuredContextPreparationNS += int64(elapsed)
	}
}
