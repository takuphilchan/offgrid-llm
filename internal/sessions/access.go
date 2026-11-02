package sessions

import (
	"errors"
	"fmt"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// Access is supplied by a trusted caller, never decoded from a client body.
// All is reserved for local single-user mode and sessions:all administrators.
// Unowned legacy sessions are deliberately not assigned to a signed-in user.
type Access struct {
	UserID string
	All    bool
	// Knowledge is a trusted, live policy check, never a persisted grant.
	Knowledge func([]string) bool
}

func (a Access) allows(session *Session) bool {
	return a.All || (a.UserID != "" && a.UserID == session.OwnerID)
}

// ScopedManager is the application boundary for remotely accessible sessions.
// Authorization and mutations share a lock so callers cannot check ownership
// against one version of an object and then overwrite another version.
// The unscoped manager remains available for trusted local CLI/file operations.
type ScopedManager struct {
	manager *SessionManager
	access  Access
}

func (sm *SessionManager) WithAccess(access Access) *ScopedManager {
	return &ScopedManager{manager: sm, access: access}
}

func (s *ScopedManager) List() ([]Session, error) {
	if !s.access.All && s.access.UserID == "" {
		return nil, ErrAccessDenied
	}
	all, err := s.manager.List()
	if err != nil {
		return nil, err
	}
	visible := make([]Session, 0, len(all))
	for _, session := range all {
		if s.access.allows(&session) && s.validateKnowledge(&session) == nil {
			visible = append(visible, session)
		}
	}
	return visible, nil
}

func (s *ScopedManager) Load(name string) (*Session, error) {
	session, err := s.loadOwned(name)
	if err != nil {
		return nil, err
	}
	if err := s.validateKnowledge(session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ScopedManager) loadOwned(name string) (*Session, error) {
	session, err := s.manager.Load(name)
	if err != nil {
		return nil, err
	}
	if !s.access.allows(session) {
		// Do not disclose the existence or owner of somebody else's session.
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (s *ScopedManager) Create(name, modelID string) (*Session, error) {
	return s.CreateWithHistory(name, modelID, nil)
}

// CreateWithHistory atomically preserves a local conversation when its user
// first attaches a service-owned file. It neither appends to an existing session
// nor admits inference. The caller's original session remains untouched.
func (s *ScopedManager) CreateWithHistory(name, modelID string, history []Message) (*Session, error) {
	if !s.access.All && s.access.UserID == "" {
		return nil, ErrAccessDenied
	}
	if err := ValidateSeedHistory(history); err != nil {
		return nil, err
	}
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	if _, err := s.manager.Load(name); err == nil {
		return nil, ErrSessionExists
	} else if !errors.Is(err, ErrSessionNotFound) {
		return nil, err
	}
	session := NewSession(name, modelID)
	session.OwnerID = s.access.UserID
	for _, m := range history {
		m.Images = append([]api.SessionImage(nil), m.Images...)
		session.Messages = append(session.Messages, m)
	}
	if err := s.manager.save(session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ScopedManager) Delete(name string) error {
	return s.deleteWithJobs(name, false)
}

// The service must first persist the explicit retain/confirmed-stop choice and
// detach its durable admission ledger. Ordinary legacy deletion stays blocked.
func (s *ScopedManager) DeleteRetainingJobs(name string) error {
	return s.deleteWithJobs(name, true)
}

func (s *ScopedManager) deleteWithJobs(name string, retain bool) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.loadOwned(name)
	if err != nil {
		return err
	}
	if !retain && (len(session.Jobs) > 0 || session.Turn != nil && session.Turn.JobAdmission != "") {
		return ErrTurnConflict
	}
	return s.manager.delete(name)
}

func (s *ScopedManager) AddMessage(name, role, content string) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return err
	}
	if session.Turn != nil && session.Turn.JobAdmission != "" && session.Turn.Active() {
		return ErrTurnConflict
	}
	session.AddMessage(role, content)
	return s.manager.save(session)
}

func (s *ScopedManager) AppendExchange(name, modelID, userContent, assistantContent string, sources ...[]KnowledgePassage) (*Session, error) {
	if userContent == "" || assistantContent == "" {
		return nil, fmt.Errorf("user and assistant content are required")
	}
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return nil, err
	}
	if session.Turn != nil && session.Turn.JobAdmission != "" && session.Turn.Active() {
		return nil, ErrTurnConflict
	}
	if modelID != "" {
		session.ModelID = modelID
	}
	session.AddMessage("user", userContent)
	session.AddMessage("assistant", assistantContent)
	if len(sources) > 0 {
		session.Messages[len(session.Messages)-1].KnowledgeSources = sources[0]
	}
	if err := s.validateKnowledge(session); err != nil {
		return nil, err
	}
	if err := s.manager.save(session); err != nil {
		return nil, err
	}
	return session, nil
}
