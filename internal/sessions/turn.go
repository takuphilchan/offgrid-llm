package sessions

import (
	"encoding/json"
	"errors"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

var ErrTurnConflict = errors.New("conversation turn changed or is still active")
var ErrImageCapacity = errors.New("conversation image capacity reached; start a new conversation")

// Turn is a recoverable generation snapshot, separate from completed context.
// A final answer and its completed status are committed in the SAME session file.
type Turn struct {
	Attachments        []AttachmentReference `json:"attachments,omitempty"`
	Images             []api.SessionImage    `json:"images,omitempty"`
	Agent              *AgentSelection       `json:"agent,omitempty"`
	JobID              string                `json:"job_id,omitempty"`
	JobAdmission       string                `json:"job_admission,omitempty"`
	ID                 string                `json:"id"`
	RequestHash        string                `json:"request_hash"`
	Prompt             string                `json:"prompt"`
	Model              string                `json:"model"`
	Profile            string                `json:"profile"`
	Knowledge          bool                  `json:"knowledge"`
	KnowledgeDocuments []string              `json:"knowledge_documents,omitempty"`
	KnowledgeSources   []KnowledgePassage    `json:"knowledge_sources,omitempty"`
	MaxTokens          int                   `json:"max_tokens"`
	Status             string                `json:"status"`
	Phase              string                `json:"phase"`
	Output             string                `json:"output"`
	Error              string                `json:"error,omitempty"`
	ErrorCode          string                `json:"error_code,omitempty"`
	FinishReason       string                `json:"finish_reason,omitempty"`
	Metrics            json.RawMessage       `json:"metrics,omitempty"`
}

func (t *Turn) Active() bool {
	if t == nil {
		return false
	}
	// An unresolved linked job cannot be replaced by another turn merely
	// because it needs access, was interrupted, or has uncertain effects.
	if t.JobAdmission != "" {
		return t.Status != "completed" && t.Status != "failed" && t.Status != "cancelled"
	}
	return t.Status == "running" || t.Status == "pending"
}

func (s *ScopedManager) BeginTurn(name string, turn *Turn) (*Turn, bool, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return nil, false, err
	}
	if current := session.Turn; current != nil {
		if current.ID == turn.ID {
			if current.RequestHash != turn.RequestHash {
				return nil, false, ErrTurnConflict
			}
			return current, false, nil
		}
		if current.Active() {
			return nil, false, ErrTurnConflict
		}
	}
	if session.TurnRequests[turn.ID] {
		return nil, false, ErrTurnConflict
	}
	if err := s.admitAttachmentContext(name, session, turn); err != nil {
		return nil, false, err
	}
	// Admission and the history byte budget share the mutation lock. Concurrent
	// completed turns cannot race a handler's earlier projection of this limit.
	imageBytes := 0
	for _, item := range turn.Images {
		imageBytes += len(item.DataURL)
	}
	for _, message := range session.Messages {
		for _, item := range message.Images {
			imageBytes += len(item.DataURL)
		}
	}
	if imageBytes > api.MaxSessionImageRequestBytes {
		return nil, false, ErrImageCapacity
	}
	if session.TurnRequests == nil {
		session.TurnRequests = map[string]bool{}
	}
	session.TurnRequests[turn.ID] = true
	session.Turn = turn
	if err := s.validateKnowledge(session); err != nil {
		return nil, false, err
	}
	if err := s.manager.save(session); err != nil {
		return nil, false, err
	}
	return turn, true, nil
}

func (s *ScopedManager) SaveTurn(name string, turn *Turn) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return err
	}
	if session.Turn == nil || session.Turn.ID != turn.ID || !session.Turn.Active() || session.Turn.JobAdmission != "" || turn.JobAdmission != "" {
		return ErrTurnConflict
	}
	session.Turn = turn
	if err := s.validateKnowledge(session); err != nil {
		return err
	}
	if turn.Status == "completed" {
		if turn.Output == "" {
			return errors.New("empty answer cannot complete a turn")
		}
		session.ModelID = turn.Model
		session.AddMessage("user", turn.Prompt)
		session.Messages[len(session.Messages)-1].Images = turn.Images
		session.Messages[len(session.Messages)-1].Attachments = turn.Attachments
		turn.Attachments = nil
		turn.Images = nil // Completed bytes live once, in the saved user message.
		session.AddMessage("assistant", turn.Output)
		session.Messages[len(session.Messages)-1].KnowledgeSources = turn.KnowledgeSources
		session.Messages[len(session.Messages)-1].KnowledgeDocuments = append([]string(nil), turn.KnowledgeDocuments...)
	}
	return s.manager.save(session)
}

// Restart never silently replays work. Preserve its draft/partial output instead.
func (sm *SessionManager) RecoverTurns() error {
	all, err := sm.List()
	if err != nil {
		return err
	}
	for _, session := range all {
		if session.Turn.Active() {
			session.Turn.Status = "interrupted"
			session.Turn.Error = "Service restarted before completion. Partial output is not saved conversational context. Review before retrying."
			if err := sm.Save(&session); err != nil {
				return err
			}
		}
	}
	return nil
}
