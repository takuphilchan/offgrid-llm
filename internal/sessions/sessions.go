package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Message represents a single chat message
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Session represents a conversation session
type Session struct {
	Name      string    `json:"name"`
	ModelID   string    `json:"model_id"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SessionManager handles session persistence
type SessionManager struct {
	sessionsDir string
}

// NewSessionManager creates a new session manager
func NewSessionManager(sessionsDir string) *SessionManager {
	return &SessionManager{
		sessionsDir: sessionsDir,
	}
}

// Save saves a session to disk
func (sm *SessionManager) Save(session *Session) error {
	if err := os.MkdirAll(sm.sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	session.UpdatedAt = time.Now()

	filePath := filepath.Join(sm.sessionsDir, session.Name+".json")
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// Load loads a session from disk
func (sm *SessionManager) Load(name string) (*Session, error) {
	filePath := filepath.Join(sm.sessionsDir, name+".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session '%s' not found", name)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &session, nil
}

// List lists all available sessions
func (sm *SessionManager) List() ([]Session, error) {
	if _, err := os.Stat(sm.sessionsDir); os.IsNotExist(err) {
		return []Session{}, nil
	}

	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		name := entry.Name()[:len(entry.Name())-5] // Remove .json
		session, err := sm.Load(name)
		if err != nil {
			continue // Skip corrupted sessions
		}
		sessions = append(sessions, *session)
	}

	// Sort by updated time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return sessions, nil
}

// Delete deletes a session
func (sm *SessionManager) Delete(name string) error {
	filePath := filepath.Join(sm.sessionsDir, name+".json")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session '%s' not found", name)
		}
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// Exists checks if a session exists
func (sm *SessionManager) Exists(name string) bool {
	filePath := filepath.Join(sm.sessionsDir, name+".json")
	_, err := os.Stat(filePath)
	return err == nil
}

// NewSession creates a new session
func NewSession(name, modelID string) *Session {
	now := time.Now()
	return &Session{
		Name:      name,
		ModelID:   modelID,
		Messages:  []Message{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddMessage adds a message to the session
func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// GetContext returns the full conversation context for the LLM
func (s *Session) GetContext() string {
	var context string
	for _, msg := range s.Messages {
		if msg.Role == "user" {
			context += "User: " + msg.Content + "\n\n"
		} else {
			context += "Assistant: " + msg.Content + "\n\n"
		}
	}
	return context
}

// MessageCount returns the number of messages in the session
func (s *Session) MessageCount() int {
	return len(s.Messages)
}

// Export exports the session to markdown format
func (s *Session) ExportMarkdown() string {
	md := fmt.Sprintf("# Session: %s\n\n", s.Name)
	md += fmt.Sprintf("**Model:** %s  \n", s.ModelID)
	md += fmt.Sprintf("**Created:** %s  \n", s.CreatedAt.Format("2006-01-02 15:04:05"))
	md += fmt.Sprintf("**Updated:** %s  \n", s.UpdatedAt.Format("2006-01-02 15:04:05"))
	md += fmt.Sprintf("**Messages:** %d\n\n", len(s.Messages))
	md += "---\n\n"

	for i, msg := range s.Messages {
		if msg.Role == "user" {
			md += fmt.Sprintf("## Message %d - User\n\n", i+1)
		} else {
			md += fmt.Sprintf("## Message %d - Assistant\n\n", i+1)
		}
		md += msg.Content + "\n\n"
		md += fmt.Sprintf("*%s*\n\n", msg.Timestamp.Format("2006-01-02 15:04:05"))
	}

	return md
}
