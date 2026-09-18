package sessions

import (
	"encoding/json"
	"errors"
)

var ErrTurnConflict = errors.New("conversation turn changed or is still active")

// Turn is a recoverable generation snapshot, separate from completed context.
// A final answer and its completed status are committed in the SAME session file.
type Turn struct {
	ID           string          `json:"id"`
	RequestHash  string          `json:"request_hash"`
	Prompt       string          `json:"prompt"`
	Model        string          `json:"model"`
	Profile      string          `json:"profile"`
	Knowledge    bool            `json:"knowledge"`
	MaxTokens    int             `json:"max_tokens"`
	Status       string          `json:"status"`
	Phase        string          `json:"phase"`
	Output       string          `json:"output"`
	Error        string          `json:"error,omitempty"`
	FinishReason string          `json:"finish_reason,omitempty"`
	Metrics      json.RawMessage `json:"metrics,omitempty"`
}

func (t *Turn) Active() bool { return t != nil && (t.Status == "running" || t.Status == "pending") }

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
	if session.TurnRequests == nil {
		session.TurnRequests = map[string]bool{}
	}
	session.TurnRequests[turn.ID] = true
	session.Turn = turn
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
	if session.Turn == nil || session.Turn.ID != turn.ID || !session.Turn.Active() {
		return ErrTurnConflict
	}
	session.Turn = turn
	if turn.Status == "completed" {
		if turn.Output == "" {
			return errors.New("empty answer cannot complete a turn")
		}
		session.ModelID = turn.Model
		session.AddMessage("user", turn.Prompt)
		session.AddMessage("assistant", turn.Output)
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
