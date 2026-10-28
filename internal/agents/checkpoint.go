package agents

import (
	"github.com/takuphilchan/offgrid-llm/internal/capabilities"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Checkpoint is the durable execution boundary for a saved task. Budget
// reservations are persisted before local inference; missing usage and
// interrupted generations retain their conservative charge on restart.
type Checkpoint struct {
	SkillAnswerRepairs         int                                `json:"skill_answer_repairs,omitempty"`
	WorkspaceCompletionRepairs int                                `json:"workspace_completion_repairs,omitempty"`
	ExecutionIntent            *ExecutionIntent                   `json:"execution_intent,omitempty"`
	ExecutionReceipt           *ExecutionReceipt                  `json:"execution_receipt,omitempty"`
	DecisionRepairs            int                                `json:"decision_repairs,omitempty"`
	WorkspaceReadFailures      int                                `json:"workspace_read_failures,omitempty"`
	ExecutingCapability        *capabilities.Descriptor           `json:"executing_capability,omitempty"`
	ComputerReceipt            *ComputerReceipt                   `json:"computer_receipt,omitempty"`
	DiscoveredTools            map[string]capabilities.Descriptor `json:"discovered_tools,omitempty"`
	PreEffectRepairs           int                                `json:"pre_effect_repairs,omitempty"`
	ReservedChildIterations    int                                `json:"reserved_child_iterations,omitempty"`
	BudgetedModelCalls         int                                `json:"budgeted_model_calls,omitempty"`
	ChargedCompletionTokens    int64                              `json:"charged_completion_tokens,omitempty"`
	ChargedInferenceNS         int64                              `json:"charged_inference_ns,omitempty"`
	MeasuredInferenceNS        int64                              `json:"measured_inference_ns,omitempty"`
	SyntheticProbeCalls        int                                `json:"synthetic_probe_calls,omitempty"`
	SyntheticProbeTokens       int64                              `json:"synthetic_probe_tokens,omitempty"`
	SyntheticProbeNS           int64                              `json:"synthetic_probe_ns,omitempty"`
	MeasuredCompletionTokens   int64                              `json:"measured_completion_tokens,omitempty"`
	UnmeasuredModelCalls       int                                `json:"unmeasured_model_calls,omitempty"`
	NativeReadRepair           *NativeReadRepair                  `json:"native_read_repair,omitempty"`
	NativeProgress             *NativeProgressState               `json:"native_progress,omitempty"`
	CompletionRepairs          int                                `json:"completion_repairs,omitempty"`
	CompletionProgress         string                             `json:"completion_progress,omitempty"`
	ProposalRepairs            int                                `json:"proposal_repairs,omitempty"`
	ProposalUnresolved         bool                               `json:"proposal_unresolved,omitempty"`
	PreEffectRepair            bool                               `json:"pre_effect_repair,omitempty"`
	PreEffectRepairTool        string                             `json:"pre_effect_repair_tool,omitempty"`
	ProposalTool               string                             `json:"proposal_tool,omitempty"`
	SequenceRepairTool         string                             `json:"sequence_repair_tool,omitempty"`
	SequenceRepairCompleted    bool                               `json:"sequence_repair_completed,omitempty"`
	WorkflowEditPending        bool                               `json:"workflow_edit_pending,omitempty"`
	Messages                   []api.ChatMessage                  `json:"messages"`
	Iteration                  int                                `json:"iteration"`
	Calls                      []api.ToolCall                     `json:"calls,omitempty"`
	ExecutingCall              string                             `json:"executing_call,omitempty"`
	ReadOnlyCall               string                             `json:"read_only_call,omitempty"`
	ReadOnlyCapability         *capabilities.Descriptor           `json:"read_only_capability,omitempty"`
}
