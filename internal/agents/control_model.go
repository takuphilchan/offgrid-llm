package agents

import (
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var ErrControlNotFound = errors.New("conversation resource not found")
var ErrControlCommand = errors.New("invalid conversation command")

// ControlScope is supplied by the authenticated service, never by a payload.
type ControlScope struct {
	Workspace string `json:"workspace"`
	Actor     string `json:"actor"`
}

type ConversationPreferences = api.ConversationPreferences
type Project = api.Project
type Conversation = api.Conversation
type WorkItem = api.WorkItem
type ConversationInput = api.ConversationInput
type ConversationCommand = api.ConversationCommand
type ConversationCommandPayload = api.ConversationCommandPayload
type ConversationCommandReceipt = api.ConversationCommandReceipt
type ControlEvent = api.ControlEvent
type ControlEventPage = api.ControlEventPage
type EffectiveConversationPreferences = api.EffectiveConversationPreferences

type controlRecord struct {
	ID       string
	Kind     string
	Revision int64
	Floor    int64
	Snapshot json.RawMessage
}
