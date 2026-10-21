package agents

import (
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/inference"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Rejected model output is never copied into executable history. Corrective
// generation consumes iterations and is persisted before another model call.
// Earlier tool effects, pending checks and authorization remain unchanged.
func (r *Runner) rejectModelDecision(task *Task, failure *inference.DecisionValidationError) error {
	cp := task.Checkpoint
	cp.DecisionRepairs++
	cp.Iteration++
	feedback := "The last model decision was rejected before any call from that response was dispatched: " + failure.Error() + ". Return a complete answer or a valid call using the currently available tool schema. Use one call at a time for computer work. Do not repeat earlier dispatched actions or invent verification."
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "user", Content: feedback})
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "rejected", Content: feedback, Timestamp: time.Now().UTC()})
	if cp.DecisionRepairs+cp.ProposalRepairs >= maxProposalRepairs {
		task.Status, task.ErrorCode = TaskInterrupted, "model_decision_repair_limit"
		task.Error = "Paused after repeated invalid model decisions. Rejected responses executed no tools; earlier work remains saved. Review the model/runtime before explicitly continuing."
		setRunPhase(task, "interrupted", "")
	}
	return r.persist(task)
}
