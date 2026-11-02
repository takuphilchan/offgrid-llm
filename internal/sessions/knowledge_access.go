package sessions

import "errors"

var ErrKnowledgeUnavailable = errors.New("saved Knowledge is no longer available")

// CheckKnowledge is repeated at use/delivery. A previous successful check does
// not authorize later reads, streamed output or publication.
func (s *ScopedManager) CheckKnowledge(ids []string) error {
	if len(ids) > 0 && s.access.Knowledge != nil && !s.access.Knowledge(ids) {
		return ErrKnowledgeUnavailable
	}
	return nil
}

func (s *ScopedManager) CheckHistory(session *Session) error { return s.validateKnowledge(session) }

func (s *ScopedManager) validateKnowledge(session *Session) error {
	ids := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, message := range session.Messages {
		for _, id := range message.KnowledgeDocuments {
			add(id)
		}
		for _, source := range message.KnowledgeSources {
			add(source.DocumentID)
		}
	}
	if turn := session.Turn; turn != nil {
		for _, id := range turn.KnowledgeDocuments {
			add(id)
		}
		for _, source := range turn.KnowledgeSources {
			add(source.DocumentID)
		}
	}
	return s.CheckKnowledge(ids)
}

// ControlState authorizes ownership but exposes no conversation/source text.
// Revocation must not prevent the owner from stopping or deleting saved work.
func (s *ScopedManager) ControlState(name string) (*Session, error) {
	current, err := s.loadOwned(name)
	if err != nil {
		return nil, err
	}
	out := &Session{Name: current.Name, OwnerID: current.OwnerID, Jobs: append([]ConversationJobLink(nil), current.Jobs...)}
	if t := current.Turn; t != nil {
		out.Turn = &Turn{ID: t.ID, RequestHash: t.RequestHash, Status: t.Status, JobID: t.JobID, JobAdmission: t.JobAdmission}
	}
	return out, nil
}

// BlockKnowledgeTurn records a safe terminal outcome after a publication check
// fails. It never returns retained output or overwrites another/newer turn.
func (s *ScopedManager) BlockKnowledgeTurn(name, id string) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	current, err := s.loadOwned(name)
	if err != nil {
		return err
	}
	if current.Turn == nil || current.Turn.ID != id || !current.Turn.Active() || current.Turn.JobAdmission != "" {
		return ErrTurnConflict
	}
	current.Turn.Status, current.Turn.Phase = "failed", "failed"
	current.Turn.ErrorCode = "knowledge_unavailable"
	current.Turn.Error = "Source access changed. No completed answer was saved."
	return s.manager.save(current)
}
