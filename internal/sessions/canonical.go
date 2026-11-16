package sessions

import (
	"encoding/json"
	"errors"
	"os"
)

var ErrCanonicalConversation = errors.New("conversation is owned by the canonical control store")

// Configure before accepting traffic. Every legacy write checks this boundary
// while holding mutationMu, including callers that bypass HTTP handlers.
func (sm *SessionManager) SetCanonicalGuard(guard func(*Session) error) {
	sm.mutationMu.Lock()
	defer sm.mutationMu.Unlock()
	sm.canonicalGuard = guard
}

func (sm *SessionManager) checkCanonical(session *Session) error {
	if sm.canonicalGuard == nil {
		return nil
	}
	return sm.canonicalGuard(session)
}

func (s *ScopedManager) checkCanonicalName(name string) error {
	current, err := s.loadOwned(name)
	if err != nil {
		return err
	}
	return s.manager.checkCanonical(current)
}

// Callers hold their conversation execution lock through model dispatch.
// Admission methods also check under mutationMu before returning old intents.
func (s *ScopedManager) LoadForExecution(name string) (*Session, error) {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	current, err := s.Load(name)
	if err != nil {
		return nil, err
	}
	if err = s.manager.checkCanonical(current); err != nil {
		return nil, err
	}
	return current, nil
}

// MigrateSnapshot freezes legacy mutation until the canonical authority marker
// commits. The original file is retained byte for byte; projection is separate.
func (s *ScopedManager) MigrateSnapshot(name string, commit func(*Session, []byte) error) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	path, err := s.manager.sessionFilePath(name)
	if err != nil {
		return err
	}
	s.manager.fileMu.RLock()
	data, err := os.ReadFile(path)
	s.manager.fileMu.RUnlock()
	if os.IsNotExist(err) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	var current Session
	if err = json.Unmarshal(data, &current); err != nil {
		return err
	}
	if current.Name != name || !s.access.allows(&current) {
		return ErrSessionNotFound
	}
	if err = s.validateKnowledge(&current); err != nil {
		return err
	}
	if current.Turn.Active() {
		return ErrTurnConflict
	}
	return commit(&current, data)
}

// DeleteCanonical commits the service-owned deletion fence before removing its
// retained legacy projection. A failed cleanup leaves the file for explicit retry.
// The callback must authenticate the canonical identity and revision atomically.
func (s *ScopedManager) DeleteCanonical(name string, commit func() error) error {
	s.manager.mutationMu.Lock()
	defer s.manager.mutationMu.Unlock()
	if _, err := s.loadOwned(name); err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	if commit == nil {
		return ErrAccessDenied
	}
	if err := commit(); err != nil {
		return err
	}
	err := s.manager.deleteFiles(name)
	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}
	return err
}
