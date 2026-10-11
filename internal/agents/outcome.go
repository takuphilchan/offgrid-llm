package agents

// TaskOutcome reports only evidence we actually have, never inferred intent or
// model-authored claims of success. Legacy unknown work and delegated outcomes
// need their existing evidence review, not a guessed success classification.
func TaskOutcome(task *Task) string {
	if task == nil || !task.Config.TaskFirst || task.Status != TaskCompleted || len(task.ChildHistory) != 0 {
		return ""
	}
	for _, step := range task.Steps {
		switch step.Type {
		case "task_metadata", "rejected", "input_resolved", "completion_rejected", "thought", "answer":
			// Planning, access bookkeeping and prose are not executed work.
		default:
			return ""
		}
	}
	return "answer_only"
}
