package sessions

import "errors"

type ConversationJobLink struct {
	TurnID string `json:"turn_id"`
	JobID  string `json:"job_id"`
}

// AgentSelection is separate from Turn.Profile, the legacy inference-budget
// preset. Selecting an agent must never reinterpret an old Chat payload.
type AgentSelection struct {
	ID       string `json:"id"`
	Revision int    `json:"revision"`
}

// BeginJobTurn saves accepted intent before the coordinator creates its held
// job. The hash is the normalized complete request, including conversation.
func (s *ScopedManager) BeginJobTurn(name string, turn *Turn) (*Turn, bool, error) {
	if turn == nil || turn.ID == "" || turn.RequestHash == "" || turn.Prompt == "" || turn.Agent == nil || turn.Agent.ID == "" || turn.Agent.Revision < 1 || turn.JobID != "" || turn.JobAdmission != "" {
		return nil, false, ErrTurnConflict
	}
	copy := *turn
	agent := *turn.Agent
	copy.Agent = &agent
	copy.Status, copy.Phase, copy.JobAdmission = "pending", "linking", "intent"
	return s.BeginTurn(name, &copy)
}

// LinkJobTurn binds the only canonical job. Repetition can complete interrupted
// bookkeeping, but a new job cannot replace an existing association.
func (s *ScopedManager) LinkJobTurn(name, turnID, hash, jobID string) (*Turn, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return nil, err
	}
	t := session.Turn
	if t == nil || t.ID != turnID || t.RequestHash != hash || t.JobAdmission == "" || jobID == "" || (t.JobID != "" && t.JobID != jobID) {
		return nil, ErrTurnConflict
	}
	if t.JobID == jobID {
		return t, nil
	}
	t.JobID, t.JobAdmission = jobID, "linked"
	session.Jobs = append(session.Jobs, ConversationJobLink{TurnID: turnID, JobID: jobID})
	if err := s.manager.save(session); err != nil {
		return nil, err
	}
	return t, nil
}

// CommitJobTurn only attaches an already completed canonical job's text. It
// never resumes execution. The exchange and final turn are one atomic file
// replacement, so a lost acknowledgment cannot append the response twice.
func (s *ScopedManager) CommitJobTurn(name, turnID, hash, jobID, model, output string) (*Turn, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return nil, err
	}
	t := session.Turn
	if t == nil || t.ID != turnID || t.RequestHash != hash || t.JobID != jobID || jobID == "" || t.JobAdmission != "linked" {
		return nil, ErrTurnConflict
	}
	if output == "" || model == "" {
		return nil, errors.New("completed job must have a model and response")
	}
	if t.Status == "completed" {
		if t.Output != output || t.Model != model {
			return nil, ErrTurnConflict
		}
		return t, nil
	}
	if t.Status == "failed" || t.Status == "cancelled" {
		return nil, ErrTurnConflict
	}
	t.Status, t.Phase, t.Output, t.Model, t.Error = "completed", "completed", output, model, ""
	session.ModelID = model
	session.AddMessage("user", t.Prompt)
	session.AddMessage("assistant", output)
	for index := len(session.Messages) - 2; index < len(session.Messages); index++ {
		session.Messages[index].JobID, session.Messages[index].TurnID = jobID, turnID
	}
	if err := s.manager.save(session); err != nil {
		return nil, err
	}
	return t, nil
}

// ProjectJobTurn only stores provisional state; completed output must pass the
// identity-bound CommitJobTurn transaction. A later observer cannot reopen it.
func (s *ScopedManager) ProjectJobTurn(name, turnID, hash, jobID, status, phase, output, failure string) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return err
	}
	t := session.Turn
	if t == nil || t.ID != turnID || t.RequestHash != hash || t.JobID != jobID || jobID == "" || t.JobAdmission != "linked" {
		return ErrTurnConflict
	}
	if t.Status == "completed" {
		return nil
	}
	if status == "completed" {
		return ErrTurnConflict
	}
	if t.Status == status && t.Phase == phase && t.Output == output && t.Error == failure {
		return nil
	}
	t.Status, t.Phase, t.Output, t.Error = status, phase, output, failure
	return s.manager.save(session)
}
