package api

import "time"

// Input is accepted independently of inference readiness. A missing focus uses
// the conversation's current focus; an explicit empty focus is unbound input.
type ConversationInboxInput struct {
	KnowledgeSelections []ConversationKnowledgeSelection `json:"knowledge_selections,omitempty"`
	UseKnowledge        bool                             `json:"use_knowledge,omitempty"`
	SourceSession       string                           `json:"source_session,omitempty"`
	Attachments         []ConversationAttachment         `json:"attachments,omitempty"`
	Images              []SessionImage                   `json:"images,omitempty"`
	KnowledgeDocuments  []string                         `json:"knowledge_documents,omitempty"`
	QuestionID          string                           `json:"question_id,omitempty"`
	Content             string                           `json:"content"`
	Model               string                           `json:"model,omitempty"`
	Intent              string                           `json:"intent,omitempty"`
	FocusedWorkID       *string                          `json:"focused_work_id,omitempty"`
	Origin              string                           `json:"origin,omitempty"`
}

type ConversationKnowledgeSelection struct {
	DocumentID string   `json:"document_id"`
	Revision   string   `json:"revision"`
	Page       int      `json:"page,omitempty"`
	BlockIDs   []string `json:"block_ids,omitempty"`
}

// A source selection is data, never a grant. Page is one-based within the
// immutable extraction. Zero selects the document's ordinary retrieval scope.
type ConversationAttachment struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Page     int    `json:"page,omitempty"`
}

// Derived from the focused work's saved primary clarification, never a grant.
type ConversationQuestion struct {
	ID      string `json:"id"`
	WorkID  string `json:"work_id"`
	RunID   string `json:"run_id"`
	Content string `json:"content"`
}

type ConversationInputCommand struct {
	RequestID        string                 `json:"request_id"`
	TargetID         string                 `json:"target_id"`
	ExpectedRevision int64                  `json:"expected_revision"`
	Kind             string                 `json:"kind"`
	Payload          ConversationInboxInput `json:"payload"`
}

type ConversationInboxEntry struct {
	Question                      *ConversationQuestion      `json:"question,omitempty"`
	ContextUnavailable            bool                       `json:"context_unavailable,omitempty"`
	StoppedAt                     *time.Time                 `json:"stopped_at,omitempty"`
	HoldReleasedAt                *time.Time                 `json:"hold_released_at,omitempty"`
	RevalidatedAt                 *time.Time                 `json:"revalidated_at,omitempty"`
	RevalidatedInstructionVersion uint64                     `json:"revalidated_instruction_version,omitempty"`
	Decision                      *ConversationDecision      `json:"decision,omitempty"`
	DecisionIssue                 *ConversationDeliveryIssue `json:"decision_issue,omitempty"`
	DeliveryIssue                 *ConversationDeliveryIssue `json:"delivery_issue,omitempty"`
	ID                            string                     `json:"id"`
	Sequence                      int64                      `json:"sequence"`
	ConversationID                string                     `json:"conversation_id"`
	ConversationRevision          int64                      `json:"conversation_revision"`
	RequestID                     string                     `json:"request_id"`
	WorkID                        string                     `json:"work_id,omitempty"`
	CreatedWorkID                 string                     `json:"created_work_id,omitempty"`
	InstructionVersion            uint64                     `json:"instruction_version,omitempty"`
	Input                         ConversationInboxInput     `json:"input"`
	State                         string                     `json:"state"`
	RunID                         string                     `json:"run_id,omitempty"`
	AcceptedAt                    time.Time                  `json:"accepted_at"`
	DeliveredAt                   *time.Time                 `json:"delivered_at,omitempty"`
	SettledAt                     *time.Time                 `json:"settled_at,omitempty"`
	RunStatus                     TaskStatus                 `json:"run_status,omitempty"`
	ResultReference               string                     `json:"result_reference,omitempty"`
	Incorporation                 string                     `json:"incorporation,omitempty"`
	AppliedAt                     *time.Time                 `json:"applied_at,omitempty"`
	AppliedInstructionVersion     uint64                     `json:"applied_instruction_version,omitempty"`
}

// ExpectedRevision refers to the containing conversation; TargetID is an input.
type ConversationInputControlCommand struct {
	RequestID        string                          `json:"request_id"`
	TargetID         string                          `json:"target_id"`
	ExpectedRevision int64                           `json:"expected_revision"`
	Kind             string                          `json:"kind"`
	Payload          ConversationInputControlPayload `json:"payload"`
}

type ConversationInputControlPayload struct {
	InstructionVersion uint64 `json:"instruction_version,omitempty"`
}

// A decision is a durable proposal, not an application receipt. AppliedAt and
// AppliedInstructionVersion remain the authority for completed incorporation.
type ConversationDecision struct {
	Kind               string `json:"kind"`
	InstructionVersion uint64 `json:"instruction_version"`
	ExpectedResult     string `json:"expected_result,omitempty"`
}

type ConversationDeliveryIssue struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	RetryAt time.Time `json:"retry_at"`
}

type ConversationInboxPage struct {
	Items      []ConversationInboxEntry `json:"items"`
	NextCursor int64                    `json:"next_cursor"`
	HasMore    bool                     `json:"has_more"`
}
