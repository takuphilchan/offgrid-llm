package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Admission stays off until primary routing and client answer flows are wired.
// Saved questions remain inspectable/answerable independently of new admission.
func configureWorkerQuestions(config *AgentConfig, parent *Task) {
	if parent.WorkID == "" || parent.Config.WorkerQuestionsVersion != 1 {
		return
	}
	config.WorkerQuestionsVersion, config.TaskFirst = 1, true
	config.SystemPrompt += " You may use worker_question for necessary assignment clarification through the primary assistant. This internal question is permitted; it does not contact external recipients or grant access."
}

func workerQuestionTool() api.Tool {
	return api.Tool{Type: "function", Function: api.FunctionDef{Name: "worker_question", Description: "Ask the primary assistant a necessary clarification about this assignment. Preserve useful partial findings in the question. The worker waits durably without holding inference capacity; an answer continues this same run and allowance. This does not request host permission or authorize another worker.", Parameters: map[string]interface{}{"type": "object", "additionalProperties": false, "required": []string{"question"}, "properties": map[string]interface{}{"question": map[string]interface{}{"type": "string", "minLength": 1, "maxLength": 4000}}}}}
}

func validateWorkerQuestion(task *Task) error {
	q := task.WorkerQuestion
	if q == nil {
		return nil
	}
	if task.Worker == nil || task.ParentID == "" || q.WorkerID != task.Worker.SessionID || q.RunID != task.ID || !validTaskID(q.ID) || !validProfileText(q.Content, 4000, true) || q.RequestedAt.IsZero() || q.InstructionVersion == 0 || q.InstructionVersion > task.InstructionVersion || q.AssignmentVersion > task.AssignmentVersion {
		return ErrRunStorage
	}
	if q.State != "pending" && q.State != "answered" {
		return ErrRunStorage
	}
	if (q.State == "answered") != (q.AnswerMessageID != "") {
		return ErrRunStorage
	}
	return nil
}

func (r *Runner) requestWorkerQuestion(task *Task, call api.ToolCall) error {
	if task.Config.WorkerQuestionsVersion != 1 || task.Worker == nil || task.ParentID == "" || task.Status != TaskRunning || task.ControlPending || task.ControlAction == "stop" || hasConversationHold(task) {
		return ErrRunConflict
	}
	var args struct {
		Question string `json:"question"`
	}
	if err := decodeRuntimeArguments(call.Function.Arguments, &args); err != nil {
		return err
	}
	if !validProfileText(args.Question, 4000, true) {
		return ErrControlCommand
	}
	if task.WorkerQuestion != nil && task.WorkerQuestion.State == "pending" {
		return ErrRunConflict
	}
	hash := sha256.Sum256([]byte(task.ID + "\x00" + call.ID))
	q := &api.WorkerQuestion{ID: "question-" + hex.EncodeToString(hash[:]), WorkerID: task.Worker.SessionID, RunID: task.ID, Content: args.Question, InstructionVersion: task.InstructionVersion, AssignmentVersion: task.AssignmentVersion, State: "pending", RequestedAt: time.Now().UTC()}
	task.WorkerQuestion = q
	task.PendingInput = &InputRequest{ID: q.ID, CallID: call.ID, Kind: "worker_question", Mode: "clarification", Target: q.Content}
	task.Status, task.SchedulerReady = TaskInput, false
	data, _ := json.Marshal(q)
	task.Steps = append(task.Steps, Step{ID: len(task.Steps) + 1, Type: "worker_question", ToolName: call.Function.Name, ToolArgs: call.Function.Arguments, ToolResult: string(data), Timestamp: q.RequestedAt})
	return r.persistModelProgress(task)
}

func answerWorkerQuestion(task, parent *Task, command api.WorkerCommand) error {
	q, cp, input := task.WorkerQuestion, task.Checkpoint, task.PendingInput
	if q == nil || q.State != "pending" || q.ID != command.Payload.QuestionID || task.Status != TaskInput || input == nil || input.Kind != "worker_question" || input.ID != q.ID || cp == nil || cp.ExecutingCall != "" || cp.ExecutionIntent != nil || len(cp.Calls) != 1 || cp.Calls[0].ID != input.CallID || cp.Calls[0].Function.Name != "worker_question" || q.InstructionVersion != parent.InstructionVersion || q.AssignmentVersion != task.AssignmentVersion {
		return ErrRunConflict
	}
	answer, _ := json.Marshal(map[string]string{"question_id": q.ID, "answer": command.Payload.Content, "status": "answer_accepted"})
	cp.Messages = append(cp.Messages, api.ChatMessage{Role: "tool", ToolCallID: input.CallID, Name: "worker_question", Content: string(answer)})
	cp.Calls = nil
	q.State, q.AnswerMessageID = "answered", command.RequestID
	task.PendingInput = nil
	task.Status, task.SchedulerReady = TaskPending, false
	return nil
}

// Explicit graph continuation retains the question checkpoint. It grants no
// host authority and schedules no model call; only an accepted answer can make
// this worker eligible for the dependency coordinator again.
func continueWorkerQuestionWait(task *Task) error {
	if reviewingWorkerQuestion(task) {
		if validateWorkerQuestionReview(task) != nil || !validWorkerClarificationInput(task) {
			return ErrRunConflict
		}
		task.ControlEpoch++
		task.ControlAction, task.ErrorCode, task.Error = "", "", ""
		task.SchedulerReady = false
		return nil
	}
	q, cp, input := task.WorkerQuestion, task.Checkpoint, task.PendingInput
	if validateWorkerQuestion(task) != nil || q == nil || q.State != "pending" || input == nil || input.Kind != "worker_question" || input.ID != q.ID || cp == nil || cp.ExecutionIntent != nil || len(cp.Calls) != 1 || cp.Calls[0].ID != input.CallID || cp.Calls[0].Function.Name != "worker_question" || q.InstructionVersion != task.InstructionVersion || q.AssignmentVersion != task.AssignmentVersion {
		return ErrRunConflict
	}
	task.ControlEpoch++
	task.ControlAction, task.ErrorCode, task.Error = "", "", ""
	task.SchedulerReady = false
	return nil
}
