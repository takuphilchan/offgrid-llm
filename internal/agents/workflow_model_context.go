package agents

// Host bindings and digests are execution/evidence state, not model arguments.
// Preserve full durable checks; project only intent, progress and saved step
// references into each model call. This avoids repeating opaque hashes every turn.
func modelWorkflowChecks(task *Task) any {
	if task.Workflow.Version != DocumentWorkflowEvidenceVersion {
		return task.Workflow.Checks
	}
	type checkView struct {
		Expectation OutcomeExpectation `json:"expectation"`
		State       string             `json:"state"`
		Reason      string             `json:"reason,omitempty"`
		Evidence    []int              `json:"evidence_steps,omitempty"`
	}
	checks := make([]checkView, 0, len(task.Workflow.Checks))
	for _, check := range task.Workflow.Checks {
		checks = append(checks, checkView{Expectation: check.Expectation, State: check.State, Reason: check.Reason, Evidence: check.EvidenceSteps})
	}
	return checks
}
