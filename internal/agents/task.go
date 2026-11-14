package agents

import (
	"context"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// TaskStatus represents the status of a task
type TaskStatus = api.TaskStatus

const (
	TaskPending           TaskStatus = "pending"
	TaskRunning           TaskStatus = "running"
	TaskWaiting           TaskStatus = "waiting_for_approval"
	TaskInput             TaskStatus = "waiting_for_input"
	TaskConversationInput TaskStatus = "waiting_for_conversation"
	TaskCompleted         TaskStatus = "completed"
	TaskFailed            TaskStatus = "failed"
	TaskCancelled         TaskStatus = "cancelled"
	TaskInterrupted       TaskStatus = "interrupted"
	TaskUncertain         TaskStatus = "uncertain"
)

// Task represents an agent task
type Task struct {
	WorkerQuestion        *api.WorkerQuestion      `json:"worker_question,omitempty"`
	ConversationTurn      *ConversationTurnBinding `json:"conversation_turn,omitempty"`
	ConversationHold      *ConversationHold        `json:"conversation_hold,omitempty"`
	Worker                *WorkerBinding           `json:"worker,omitempty"`
	AssignmentVersion     uint64                   `json:"assignment_version,omitempty"`
	WorkerMessageRequests map[string]bool          `json:"worker_message_requests,omitempty"`
	DependencyResults     map[string]string        `json:"dependency_results,omitempty"`
	WorkID                string                   `json:"work_id,omitempty"`
	PreviousWorkerIDs     []string                 `json:"previous_worker_ids,omitempty"`
	ControlAction         string                   `json:"control_action,omitempty"`
	ControlPending        bool                     `json:"control_pending,omitempty"`
	ProcessStopPending    bool                     `json:"process_stop_pending,omitempty"`

	InstructionVersion     uint64              `json:"instruction_version,omitempty"`
	ControlEpoch           uint64              `json:"control_epoch,omitempty"`
	Origin                 *ConversationOrigin `json:"origin,omitempty"`
	AdmissionHeld          bool                `json:"admission_held,omitempty"`
	SchedulerReady         bool                `json:"scheduler_ready,omitempty"`
	ParentID               string              `json:"parent_id,omitempty"`
	ChildHistory           []ChildLink         `json:"children,omitempty"`
	Delegation             *Delegation         `json:"delegation,omitempty"`
	DependenciesAttached   bool                `json:"dependencies_attached,omitempty"`
	Plan                   []PlanItem          `json:"plan,omitempty"`
	Workflow               *WorkflowEvidence   `json:"workflow,omitempty"`
	Context                *ContextState       `json:"context,omitempty"`
	ContextArchive         []ArchivedContext   `json:"context_archive,omitempty"`
	Instructions           []Instruction       `json:"instructions,omitempty"`
	PendingInstructions    []Instruction       `json:"pending_instructions,omitempty"`
	LastAccess             *InputRequest       `json:"last_access,omitempty"`
	ID                     string              `json:"id"`
	Prompt                 string              `json:"prompt"`
	Status                 TaskStatus          `json:"status"`
	Result                 string              `json:"result,omitempty"`
	Error                  string              `json:"error,omitempty"`
	ErrorCode              string              `json:"error_code,omitempty"`
	Steps                  []Step              `json:"steps,omitempty"`
	Config                 AgentConfig         `json:"config"`
	CreatedAt              time.Time           `json:"created_at"`
	StartedAt              *time.Time          `json:"started_at,omitempty"`
	CompletedAt            *time.Time          `json:"completed_at,omitempty"`
	DeletedAt              *time.Time          `json:"deleted_at,omitempty"`
	ArchivedAt             *time.Time          `json:"archived_at,omitempty"`
	Model                  string              `json:"model,omitempty"`
	Actor                  string              `json:"actor,omitempty"`
	ComputerSessionExpired bool                `json:"computer_session_expired,omitempty"`
	PendingApproval        *Approval           `json:"pending_approval,omitempty"`
	PendingInput           *InputRequest       `json:"pending_input,omitempty"`
	ResolvedInputID        string              `json:"resolved_input_id,omitempty"`
	RequestID              string              `json:"request_id,omitempty"`
	RequestDigest          string              `json:"request_digest,omitempty"`
	Checkpoint             *Checkpoint         `json:"checkpoint,omitempty"`
	Progress               *RunProgress        `json:"progress,omitempty"`
	cancel                 context.CancelFunc
}
