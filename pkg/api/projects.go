package api

// An address of an already approved supervisor session; knowing it grants no access.
type ExecutionReference struct {
	Host     string `json:"host"`
	Session  string `json:"session"`
	Grant    string `json:"grant"`
	Revision int    `json:"revision"`
}

// Project resources name existing scoped catalog entries. Their presence never
// creates authority, enrolls a host or makes network credentials available.
type ProjectResourceReference struct {
	Kind     string `json:"kind" toml:"kind"`
	ID       string `json:"id" toml:"id"`
	Revision string `json:"revision,omitempty" toml:"revision"`
	Required bool   `json:"required,omitempty" toml:"required"`
}

// ResourceResolution explains admission without claiming a reference is a grant.
type ResourceResolution struct {
	ProjectResourceReference
	Selected bool   `json:"selected"`
	Reason   string `json:"reason"`
}

type ProjectCommand struct {
	RequestID        string                `json:"request_id"`
	TargetID         string                `json:"target_id"`
	ExpectedRevision int64                 `json:"expected_revision"`
	Kind             string                `json:"kind"`
	Payload          ProjectCommandPayload `json:"payload"`
}

type ProjectCommandPayload struct {
	ConfigurationTOML string                     `json:"configuration_toml,omitempty"`
	Name              string                     `json:"name"`
	Preferences       ConversationPreferences    `json:"preferences"`
	Resources         []ProjectResourceReference `json:"resources,omitempty"`
}
