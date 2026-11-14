package api

// Work controls share the actor-scoped deduplication namespace with other v2
// commands. Corrections do not allocate new execution allowance.
type WorkCommand struct {
	RequestID        string             `json:"request_id"`
	TargetID         string             `json:"target_id"`
	ExpectedRevision int64              `json:"expected_revision"`
	Kind             string             `json:"kind"`
	Payload          WorkCommandPayload `json:"payload"`
}

type WorkCommandPayload struct {
	Instruction string `json:"instruction,omitempty"`
}
