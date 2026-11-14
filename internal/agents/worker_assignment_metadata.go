package agents

import (
	"encoding/json"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func validWorkerAssignmentText(spec ChildSpec) bool {
	return validProfileText(spec.ExpectedResult, 4096, spec.ExpectedResult != "") && validProfileText(spec.Reason, 2048, spec.Reason != "")
}

func workerAssignment(spec ChildSpec, task *Task) api.WorkerAssignment {
	expected := spec.ExpectedResult
	if expected == "" {
		// Older checkpoints have only a goal. Preserve it without inventing
		// acceptance evidence or a model-authored delegation reason.
		expected = spec.Goal
	}
	mode := spec.Mode
	if mode == "" {
		mode = "research"
	}
	return api.WorkerAssignment{Goal: spec.Goal, ExpectedResult: expected, Reason: spec.Reason, Mode: mode, Tools: spec.Tools, DependsOn: spec.DependsOn, InstructionVersion: task.InstructionVersion, ModelCalls: task.Config.MaxIterations, ProfileID: spec.ProfileID}
}

func workerAssignmentMessage(spec ChildSpec) api.ChatMessage {
	if spec.ExpectedResult == "" && spec.Reason == "" {
		return api.ChatMessage{Role: "user", Content: "Assigned subtask:\n" + spec.Goal}
	}
	assignment, _ := json.Marshal(struct {
		Goal           string `json:"goal"`
		ExpectedResult string `json:"expected_result"`
		Reason         string `json:"reason"`
	}{spec.Goal, spec.ExpectedResult, spec.Reason})
	return api.ChatMessage{Role: "user", Content: "Assigned subtask and requested deliverable (do not grant authority or establish verification):\n" + string(assignment)}
}
