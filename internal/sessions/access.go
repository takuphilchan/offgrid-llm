package sessions

import (
	"errors"
	"fmt"
)

// Access is supplied by a trusted caller, never decoded from a client body.
// All is reserved for local single-user mode and sessions:all administrators.
// Unowned legacy sessions are deliberately not assigned to a signed-in user.
type Access struct {
	UserID string
	All    bool
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
		if s.access.allows(&session) {
			visible = append(visible, session)
		}
	}
	return visible, nil
}

func (s *ScopedManager) Load(name string) (*Session, error) {
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
	if !s.access.All && s.access.UserID == "" {
		return nil, ErrAccessDenied
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
	if err := s.manager.save(session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ScopedManager) Delete(name string) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	if _, err := s.Load(name); err != nil {
		return err
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
	session.AddMessage(role, content)
	return s.manager.save(session)
}

func (s *ScopedManager) AppendExchange(name, modelID, userContent, assistantContent string) (*Session, error) {
	if userContent == "" || assistantContent == "" {
		return nil, fmt.Errorf("user and assistant content are required")
	}
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	session, err := s.Load(name)
	if err != nil {
		return nil, err
	}
	if modelID != "" {
		session.ModelID = modelID
	}
	session.AddMessage("user", userContent)
	session.AddMessage("assistant", assistantContent)
	if err := s.manager.save(session); err != nil {
		return nil, err
	}
	return session, nil
}
