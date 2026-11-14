package api

import "time"

// WorkerPolicy is pinned by the service at work admission. It is a resource
// policy, never an authority grant or permission to delegate recursively.
type WorkerPolicy struct {
	Version           int `json:"version"`
	MaxSessions       int `json:"max_sessions"`
	MaxActive         int `json:"max_active"`
	MaxDepth          int `json:"max_depth"`
	InitialModelCalls int `json:"initial_model_calls"`
}

type WorkerAssignment struct {
	Mode               string   `json:"mode,omitempty"`
	Reason             string   `json:"reason,omitempty"`
	Goal               string   `json:"goal"`
	ExpectedResult     string   `json:"expected_result"`
	Tools              []string `json:"tools"`
	DependsOn          []string `json:"depends_on"`
	InstructionVersion uint64   `json:"instruction_version"`
	ModelCalls         int      `json:"model_calls"`
	ProfileID          string   `json:"profile_id,omitempty"`
}

type WorkerSession struct {
	Question          *WorkerQuestion  `json:"question,omitempty"`
	ID                string           `json:"id"`
	Revision          int64            `json:"revision"`
	WorkID            string           `json:"work_id"`
	RootRunID         string           `json:"root_run_id"`
	CurrentRunID      string           `json:"current_run_id"`
	RunIDs            []string         `json:"run_ids"`
	Assignment        WorkerAssignment `json:"assignment"`
	AssignmentVersion uint64           `json:"assignment_version"`
	Status            TaskStatus       `json:"status"`
	Stopped           bool             `json:"stopped"`
	Closed            bool             `json:"closed"`
	ResultReference   string           `json:"result_reference,omitempty"`
}

type WorkerCommand struct {
	RequestID        string               `json:"request_id"`
	TargetID         string               `json:"target_id"`
	ExpectedRevision int64                `json:"expected_revision"`
	Kind             string               `json:"kind"`
	Payload          WorkerCommandPayload `json:"payload"`
}

type WorkerCommandPayload struct {
	Content    string `json:"content,omitempty"`
	QuestionID string `json:"question_id,omitempty"`
}

type WorkerQuestion struct {
	ID                 string    `json:"id"`
	WorkerID           string    `json:"worker_id"`
	RunID              string    `json:"run_id"`
	Content            string    `json:"content"`
	InstructionVersion uint64    `json:"instruction_version"`
	AssignmentVersion  uint64    `json:"assignment_version"`
	State              string    `json:"state"`
	RequestedAt        time.Time `json:"requested_at"`
	AnswerMessageID    string    `json:"answer_message_id,omitempty"`
}

type WorkerMessage struct {
	QuestionID         string     `json:"question_id,omitempty"`
	ID                 string     `json:"id"`
	WorkerID           string     `json:"worker_id"`
	RunID              string     `json:"run_id"`
	Content            string     `json:"content"`
	State              string     `json:"state"`
	InstructionVersion uint64     `json:"instruction_version"`
	AcceptedAt         time.Time  `json:"accepted_at"`
	DeliveredAt        *time.Time `json:"delivered_at,omitempty"`
	AppliedAt          *time.Time `json:"applied_at,omitempty"`
	AbandonedAt        *time.Time `json:"abandoned_at,omitempty"`
	ResolutionReason   string     `json:"resolution_reason,omitempty"`
}

type WorkerMessagePage struct {
	Items      []WorkerMessage `json:"items"`
	NextCursor int64           `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
}
