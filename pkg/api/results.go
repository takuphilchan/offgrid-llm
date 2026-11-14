package api

// ResultPage carries bounded UTF-8 output with immutable settlement provenance.
// Status is the recorded run outcome; independently_verified is separate.
type ResultPage struct {
	ID                    string     `json:"id"`
	RunID                 string     `json:"run_id"`
	WorkID                string     `json:"work_id,omitempty"`
	WorkerID              string     `json:"worker_id,omitempty"`
	ConversationID        string     `json:"conversation_id,omitempty"`
	InstructionVersion    uint64     `json:"instruction_version"`
	ControlEpoch          uint64     `json:"control_epoch"`
	Status                TaskStatus `json:"status"`
	IndependentlyVerified bool       `json:"independently_verified"`
	EvidenceSteps         []int      `json:"evidence_steps"`
	Content               string     `json:"content"`
	Offset                int        `json:"offset"`
	NextOffset            int        `json:"next_offset"`
	TotalBytes            int        `json:"total_bytes"`
	Complete              bool       `json:"complete"`
}
