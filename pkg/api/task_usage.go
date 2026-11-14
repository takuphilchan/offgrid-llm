package api

// TaskResourceUsage contains accounting only, never executable checkpoints or
// host grants. Root attempts inherit earlier root charges; do not sum attempts.
type TaskResourceUsage struct {
	ModelCallLimit               int               `json:"model_call_limit"`
	ModelCalls                   int               `json:"model_calls"`
	ChildCallAllocations         int               `json:"child_call_allocations"`
	Input                        *TaskInputUsage   `json:"input,omitempty"`
	Tools                        *TaskToolUsage    `json:"tools,omitempty"`
	Processes                    *TaskProcessUsage `json:"processes,omitempty"`
	Storage                      *TaskStorageUsage `json:"storage,omitempty"`
	ChargedOutputTokens          int64             `json:"charged_output_tokens"`
	MeasuredOutputTokens         int64             `json:"measured_output_tokens"`
	ChargedInferenceNS           int64             `json:"charged_inference_ns"`
	MeasuredInferenceNS          int64             `json:"measured_inference_ns"`
	ContextPreparations          int               `json:"context_preparations"`
	MeasuredContextPreparationNS int64             `json:"measured_context_preparation_ns"`
}

type TaskInputUsage struct {
	Window          int   `json:"window"`
	ChargedTokens   int64 `json:"charged_tokens"`
	MeasuredTokens  int64 `json:"measured_tokens"`
	UnmeasuredCalls int   `json:"unmeasured_calls"`
}

type TaskToolUsage struct {
	Version             int   `json:"version"`
	CallsPerDecision    int   `json:"calls_per_decision"`
	PerCallNS           int64 `json:"per_call_ns"`
	ReservedCalls       int   `json:"reserved_calls"`
	LegacyReservedCalls int   `json:"legacy_reserved_calls"`
	ChargedNS           int64 `json:"charged_ns"`
	MeasuredNS          int64 `json:"measured_ns"`
	UnsettledCalls      int   `json:"unsettled_calls"`
}

type TaskProcessUsage struct {
	Version              int   `json:"version"`
	PerDecisionNS        int64 `json:"per_decision_ns"`
	StartsPerDecision    int   `json:"starts_per_decision"`
	ReservedStarts       int   `json:"reserved_starts"`
	LegacyReservedStarts int   `json:"legacy_reserved_starts"`
	ChargedNS            int64 `json:"charged_ns"`
	MeasuredNS           int64 `json:"measured_ns"`
	UnsettledStarts      int   `json:"unsettled_starts"`
}

type TaskStorageUsage struct {
	Version          int   `json:"version"`
	LimitBytes       int64 `json:"limit_bytes"`
	ChargedBytes     int64 `json:"charged_bytes"`
	RetainedCopies   int   `json:"retained_copies"`
	MeasuredBytes    int64 `json:"measured_bytes"`
	UnmeasuredCopies int   `json:"unmeasured_copies"`
}
