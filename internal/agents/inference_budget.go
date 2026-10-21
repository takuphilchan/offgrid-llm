package agents

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var ErrInferenceBudget = errors.New("task inference budget exhausted")

// BeginSyntheticProbe reserves the check and registers it with the same Stop
// and Shutdown path as the root job. The caller must release the claim before
// continuing the saved task. No computer input is dispatched by this scope.
func (r *Runner) BeginSyntheticProbe(ctx context.Context, id, actor string, maxOutput int, maxTime time.Duration) (context.Context, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing || ctx.Err() != nil {
		return nil, nil, ErrRunConflict
	}
	if _, active := r.active[id]; active {
		return nil, nil, ErrRunConflict
	}
	_, err := r.manager.updateTask(id, func(task *Task) error {
		if task.Actor != actor {
			return ErrTaskNotFound
		}
		if task.DeletedAt != nil || task.ParentID != "" || task.Status != TaskInput || task.PendingInput == nil || task.PendingInput.Kind != "computer" || task.Checkpoint == nil {
			return ErrRunConflict
		}
		if err := reserveSyntheticProbeBudget(task, maxOutput, maxTime); err != nil {
			return fmt.Errorf("%w: %v", ErrInferenceBudget, err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	probeCtx, cancel := context.WithCancel(ctx)
	r.active[id] = cancel
	r.workers.Add(1)
	var once sync.Once
	finish := func() {
		once.Do(func() {
			cancel()
			r.mu.Lock()
			delete(r.active, id)
			r.mu.Unlock()
			r.workers.Done()
		})
	}
	return probeCtx, finish, nil
}

// ReserveSyntheticProbeBudget charges a task-specific, no-effects capability
// check before the service starts local probe inference. It cannot add grants,
// acquire a desktop lease, or change a child budget. Repeated checks consume
// the same root budget rather than being free after a failed preflight.
func (r *Runner) ReserveSyntheticProbeBudget(id, actor string, maxOutput int, maxTime time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closing {
		return ErrRunConflict
	}
	_, err := r.manager.updateTask(id, func(task *Task) error {
		if task.Actor != actor {
			return ErrTaskNotFound
		}
		if task.DeletedAt != nil || task.ParentID != "" || task.Status != TaskInput || task.PendingInput == nil || task.PendingInput.Kind != "computer" || task.Checkpoint == nil {
			return ErrRunConflict
		}
		if err := reserveSyntheticProbeBudget(task, maxOutput, maxTime); err != nil {
			return fmt.Errorf("%w: %v", ErrInferenceBudget, err)
		}
		return nil
	})
	return err
}

// reserveModelBudget durably charges the worst-case output and wall time of an
// attempted generation before dispatch. Completion can refund measured unused
// capacity; a crash or non-cooperative provider leaves the conservative charge.
// Child jobs receive disjoint decision slots, so their maxima fit the root cap.
func reserveModelBudget(task *Task) error {
	if task == nil || task.Checkpoint == nil || task.Config.MaxTokens < 0 || task.Config.TimeoutPerStep <= 0 {
		return fmt.Errorf("Model budget is unavailable; no generation was dispatched")
	}
	if task.Config.MaxTokens == 0 {
		// Legacy saved jobs omitted this field and relied on the standard
		// generation ceiling. Pin it before persisting the reservation.
		task.Config.MaxTokens = DefaultAgentConfig().MaxTokens
	}
	cp := task.Checkpoint
	perCall := int64(task.Config.MaxTokens)
	perCallNS := int64(task.Config.TimeoutPerStep)
	slots := task.Config.MaxIterations - cp.ReservedChildIterations
	if slots < 1 || int64(slots) > math.MaxInt64/perCall || int64(slots) > math.MaxInt64/perCallNS {
		return fmt.Errorf("Task's aggregate inference budget is exhausted; no generation was dispatched")
	}
	// Old checkpoints lack a generation-attempt ledger. Charge their accepted
	// decisions conservatively rather than granting them a fresh root budget.
	if cp.BudgetedModelCalls == 0 && cp.Iteration > 0 {
		cp.BudgetedModelCalls = cp.Iteration
		if cp.ChargedCompletionTokens > math.MaxInt64-int64(cp.Iteration)*perCall || cp.ChargedInferenceNS > math.MaxInt64-int64(cp.Iteration)*perCallNS {
			return fmt.Errorf("Task's aggregate inference budget is exhausted; no generation was dispatched")
		}
		cp.ChargedCompletionTokens += int64(cp.Iteration) * perCall
		cp.ChargedInferenceNS += int64(cp.Iteration) * perCallNS
	}
	if cp.BudgetedModelCalls < 0 || cp.ChargedCompletionTokens < 0 || cp.ChargedInferenceNS < 0 ||
		max(cp.Iteration, cp.BudgetedModelCalls) >= slots ||
		cp.ChargedCompletionTokens > int64(slots)*perCall-perCall ||
		cp.ChargedInferenceNS > int64(slots)*perCallNS-perCallNS {
		return fmt.Errorf("Task's aggregate inference budget is exhausted; no generation was dispatched")
	}
	cp.BudgetedModelCalls++
	cp.ChargedCompletionTokens += perCall
	cp.ChargedInferenceNS += perCallNS
	return nil
}

// reserveSyntheticProbeBudget joins a task-specific computer or perception
// preflight to the same durable root output/time ledger. The probe's own
// deadline remains independent of model admission; this reservation is its
// maximum cost, not a claim of measured token usage.
func reserveSyntheticProbeBudget(task *Task, maxOutput int, maxTime time.Duration) error {
	if task == nil || task.ParentID != "" || task.Checkpoint == nil || maxOutput < 1 || maxTime <= 0 || task.Config.MaxTokens < 0 || task.Config.TimeoutPerStep <= 0 {
		return fmt.Errorf("Task probe budget is unavailable; no model check was dispatched")
	}
	if task.Config.MaxTokens == 0 {
		task.Config.MaxTokens = DefaultAgentConfig().MaxTokens
	}
	cp := task.Checkpoint
	slots := task.Config.MaxIterations - cp.ReservedChildIterations
	if slots < 1 || int64(slots) > math.MaxInt64/int64(task.Config.MaxTokens) || int64(slots) > math.MaxInt64/int64(task.Config.TimeoutPerStep) {
		return fmt.Errorf("Task probe budget is exhausted; no model check was dispatched")
	}
	tokenLimit := int64(slots) * int64(task.Config.MaxTokens)
	timeLimit := int64(slots) * int64(task.Config.TimeoutPerStep)
	if cp.ChargedCompletionTokens < 0 || cp.ChargedInferenceNS < 0 || cp.ChargedCompletionTokens > tokenLimit-int64(maxOutput) || cp.ChargedInferenceNS > timeLimit-int64(maxTime) {
		return fmt.Errorf("Task probe budget is exhausted; no model check was dispatched")
	}
	cp.ChargedCompletionTokens += int64(maxOutput)
	cp.ChargedInferenceNS += int64(maxTime)
	cp.SyntheticProbeCalls++
	cp.SyntheticProbeTokens += int64(maxOutput)
	cp.SyntheticProbeNS += int64(maxTime)
	return nil
}

func settleModelTime(task *Task, elapsed time.Duration) {
	if task == nil || task.Checkpoint == nil || task.Checkpoint.BudgetedModelCalls == 0 {
		return
	}
	if elapsed < 0 {
		elapsed = 0
	}
	cp := task.Checkpoint
	spent := min(int64(elapsed), int64(task.Config.TimeoutPerStep))
	cp.ChargedInferenceNS -= int64(task.Config.TimeoutPerStep) - spent
	if int64(elapsed) <= math.MaxInt64-cp.MeasuredInferenceNS {
		cp.MeasuredInferenceNS += int64(elapsed)
	}
}

// recordDecisionUsage checks measured output before a model decision can be
// interpreted as an action. Iteration reservations bound the number of calls
// across a root and its children; this records which part of that output budget
// the runtime actually measured. Missing usage is unknown, never zero usage.
func recordDecisionUsage(task *Task, usage api.Usage) error {
	if task == nil || task.Checkpoint == nil {
		return fmt.Errorf("Model usage cannot be recorded without a task checkpoint")
	}
	if usage.PromptTokens < 0 || usage.CompletionTokens < 0 || usage.TotalTokens < 0 ||
		usage.TotalTokens > 0 && usage.PromptTokens > usage.TotalTokens-usage.CompletionTokens {
		return fmt.Errorf("Model returned inconsistent token usage; no action was executed")
	}
	if task.Config.MaxTokens > 0 && usage.CompletionTokens > task.Config.MaxTokens {
		return fmt.Errorf("Model exceeded the task's per-decision output token budget; no action was executed")
	}
	if usage.CompletionTokens == 0 {
		task.Checkpoint.UnmeasuredModelCalls++
		return nil
	}
	spent := task.Checkpoint.MeasuredCompletionTokens
	if spent < 0 || int64(usage.CompletionTokens) > math.MaxInt64-spent {
		return fmt.Errorf("Model token usage accounting overflowed; no action was executed")
	}
	spent += int64(usage.CompletionTokens)
	if task.Config.MaxTokens > 0 {
		slots := task.Config.MaxIterations - task.Checkpoint.ReservedChildIterations
		if slots < 0 || int64(slots) > math.MaxInt64/int64(task.Config.MaxTokens) ||
			spent > int64(slots)*int64(task.Config.MaxTokens) {
			return fmt.Errorf("Task's aggregate output token budget is exhausted; no action was executed")
		}
	}
	task.Checkpoint.MeasuredCompletionTokens = spent
	if task.Checkpoint.BudgetedModelCalls > 0 {
		// This response belongs to the latest pre-dispatch reservation. Missing
		// usage above kept the full charge instead of presenting zero cost.
		task.Checkpoint.ChargedCompletionTokens -= int64(task.Config.MaxTokens - usage.CompletionTokens)
	}
	return nil
}
