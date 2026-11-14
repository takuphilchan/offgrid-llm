package api

import (
	"encoding/json"
	"time"
)

// Shared Go wire types for the conversation resources in openapi.yaml.
// Keeping these independent of execution avoids a client/runtime import cycle.
type TaskStatus string

type ExecutionPermissions struct {
	Read      bool `json:"read"`
	Write     bool `json:"write"`
	Processes bool `json:"processes"`
}
type ProfileToolReference struct {
	Provider       string `json:"provider"`
	Tool           string `json:"tool"`
	SchemaRevision string `json:"schema_revision"`
	Location       string `json:"location"`
}
type ResourceRestrictions struct {
	Tools     *[]ProfileToolReference `json:"tools,omitempty"`
	Knowledge *[]string               `json:"knowledge,omitempty"`
	Computer  *bool                   `json:"computer,omitempty"`
	Execution *ExecutionPermissions   `json:"execution,omitempty"`
}

// Preferences contain no credentials, host enrollment, live grants or hooks.
// Resource restrictions are ceilings, not proof that a resource is accessible.
type ConversationPreferences struct {
	Model           string               `json:"model,omitempty"`
	ProfileID       string               `json:"profile_id,omitempty"`
	ProfileRevision int                  `json:"profile_revision,omitempty"`
	Restrictions    ResourceRestrictions `json:"restrictions"`
	Delegation      *bool                `json:"delegation,omitempty"`
}

type Project struct {
	FolderIdentity      string                     `json:"folder_identity,omitempty"`
	ConfigurationSource string                     `json:"configuration_source,omitempty"`
	Resources           []ProjectResourceReference `json:"resources,omitempty"`
	ConfigurationDigest string                     `json:"configuration_digest,omitempty"`

	ID          string                  `json:"id"`
	Revision    int64                   `json:"revision"`
	Name        string                  `json:"name"`
	Default     bool                    `json:"default"`
	Preferences ConversationPreferences `json:"preferences"`
}

type Conversation struct {
	PendingQuestion    *ConversationQuestion   `json:"pending_question,omitempty"`
	ContextRevision    uint64                  `json:"context_revision,omitempty"`
	ConfigurationKnown bool                    `json:"configuration_known"`
	ID                 string                  `json:"id"`
	Revision           int64                   `json:"revision"`
	ProjectID          string                  `json:"project_id"`
	Title              string                  `json:"title"`
	Preferences        ConversationPreferences `json:"preferences"`
	FocusedWorkID      string                  `json:"focused_work_id,omitempty"`
	CreatedAt          time.Time               `json:"created_at"`
}

type WorkItem struct {
	WorkerPolicy    *WorkerPolicy `json:"worker_policy,omitempty"`
	ProjectRevision int64         `json:"project_revision"`

	ID                    string     `json:"id"`
	Revision              int64      `json:"revision"`
	ProjectID             string     `json:"project_id"`
	Objective             string     `json:"objective"`
	Acceptance            []string   `json:"acceptance"`
	RunID                 string     `json:"run_id"`
	RunIDs                []string   `json:"run_ids,omitempty"`
	Status                TaskStatus `json:"status"`
	InstructionVersion    uint64     `json:"instruction_version"`
	ControlEpoch          uint64     `json:"control_epoch"`
	ResultReference       string     `json:"result_reference,omitempty"`
	IndependentlyVerified bool       `json:"independently_verified"`
}

type ConversationInput struct {
	Content    string   `json:"content"`
	Model      string   `json:"model,omitempty"`
	Acceptance []string `json:"acceptance,omitempty"`
}

// Commands use typed payloads so unknown fields cannot silently become
// configuration or authority. IDs are actor/workspace scoped across targets.
type ConversationCommand struct {
	RequestID        string                     `json:"request_id"`
	TargetID         string                     `json:"target_id"`
	ExpectedRevision int64                      `json:"expected_revision"`
	Kind             string                     `json:"kind"`
	Payload          ConversationCommandPayload `json:"payload"`
}

type ConversationCommandPayload struct {
	Folder      *ExecutionReference      `json:"folder,omitempty"`
	WorkID      string                   `json:"work_id,omitempty"`
	LegacyName  string                   `json:"legacy_name,omitempty"`
	Title       string                   `json:"title,omitempty"`
	ProjectID   string                   `json:"project_id,omitempty"`
	Preferences *ConversationPreferences `json:"preferences,omitempty"`
	Input       *ConversationInput       `json:"input,omitempty"`
}

type ConversationCommandReceipt struct {
	Input           *ConversationInboxEntry     `json:"input,omitempty"`
	UserPreferences *UserPreferences            `json:"user_preferences,omitempty"`
	Import          *ConversationImportManifest `json:"import,omitempty"`
	Worker          *WorkerSession              `json:"worker,omitempty"`
	Message         *WorkerMessage              `json:"message,omitempty"`
	Project         *Project                    `json:"project,omitempty"`

	RequestID    string        `json:"request_id"`
	TargetID     string        `json:"target_id"`
	Revision     int64         `json:"revision"`
	EventCursor  int64         `json:"event_cursor"`
	Conversation *Conversation `json:"conversation,omitempty"`
	Work         *WorkItem     `json:"work,omitempty"`
}

type ConversationImportManifest struct {
	Version            int       `json:"version"`
	LegacyName         string    `json:"legacy_name"`
	ConversationID     string    `json:"conversation_id"`
	SourceSHA256       string    `json:"source_sha256"`
	Authority          string    `json:"authority"`
	ConfigurationKnown bool      `json:"configuration_known"`
	LegacyRunIDs       []string  `json:"legacy_run_ids"`
	ImportedAt         time.Time `json:"imported_at"`
}

type ControlEvent struct {
	Cursor     int64           `json:"cursor"`
	TargetID   string          `json:"target_id"`
	Revision   int64           `json:"revision"`
	Kind       string          `json:"kind"`
	RecordedAt time.Time       `json:"recorded_at"`
	Payload    json.RawMessage `json:"payload"`
}

type ControlEventPage struct {
	Events     []ControlEvent  `json:"events"`
	NextCursor int64           `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
	HistoryGap bool            `json:"history_gap"`
	Snapshot   json.RawMessage `json:"snapshot,omitempty"`
}

// EffectiveConversationPreferences exposes the source of each resolved
// preference. It intentionally contains no effective/live authority claims.
type EffectiveConversationPreferences struct {
	Folder                  *ConversationFolderStatus `json:"folder,omitempty"`
	UserPreferencesRevision int64                     `json:"user_preferences_revision"`
	Resources               []ResourceResolution      `json:"resources"`
	ProjectID               string                    `json:"project_id"`
	ProjectRevision         int64                     `json:"project_revision"`

	ConfigurationKnown bool                 `json:"configuration_known"`
	Model              string               `json:"model"`
	Delegation         bool                 `json:"delegation"`
	ProfileID          string               `json:"profile_id,omitempty"`
	ProfileRevision    int                  `json:"profile_revision,omitempty"`
	Restrictions       ResourceRestrictions `json:"restrictions"`
	Provenance         map[string]string    `json:"provenance"`
}

// Availability is observation metadata, never a grant or execution admission.
type ConversationFolderStatus struct {
	Session   string `json:"session"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}
