package api

// User defaults are preferences only. Authority and memory consent are not
// configurable through this resource. Empty values inherit service defaults.
type UserDefaultValues struct {
	Model      string `json:"model,omitempty"`
	Delegation *bool  `json:"delegation,omitempty"`
}

type UserPreferences struct {
	ID          string            `json:"id"`
	Revision    int64             `json:"revision"`
	Preferences UserDefaultValues `json:"preferences"`
}

type UserPreferencesCommand struct {
	RequestID        string            `json:"request_id"`
	TargetID         string            `json:"target_id"`
	ExpectedRevision int64             `json:"expected_revision"`
	Kind             string            `json:"kind"`
	Payload          UserDefaultValues `json:"payload"`
}
