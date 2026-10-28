package agents

import "encoding/json"

// A successful plan update is not substantive progress. Do not offer another
// consecutive plan update before an observation/action or a user correction.
// Failed plan validation remains correctable, and no tool dispatch is forced.
func planJustRecorded(task *Task) bool {
	if task.Checkpoint == nil || len(task.Checkpoint.Messages) == 0 {
		return false
	}
	last := task.Checkpoint.Messages[len(task.Checkpoint.Messages)-1]
	if last.Role != "tool" || last.Name != "task_plan" {
		return false
	}
	var result struct {
		Saved bool `json:"saved"`
	}
	return json.Unmarshal([]byte(last.StringContent()), &result) == nil && result.Saved
}
