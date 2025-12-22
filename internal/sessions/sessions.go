package sessions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
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

// SessionMeta contains lightweight session metadata for fast listing
type SessionMeta struct {
	Name         string    `json:"name"`
	ModelID      string    `json:"model_id"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ModTime      time.Time `json:"-"` // File modification time for cache invalidation
}

// SessionManager handles session persistence with metadata caching
type SessionManager struct {
	sessionsDir string
	mu          sync.RWMutex
	metaCache   map[string]*SessionMeta // Cache of session metadata keyed by name
	cacheValid  bool                    // Whether the cache is valid
}

// NewSessionManager creates a new session manager
func NewSessionManager(sessionsDir string) *SessionManager {
	return &SessionManager{
		sessionsDir: sessionsDir,
		metaCache:   make(map[string]*SessionMeta),
		cacheValid:  false,
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

	// Update metadata cache
	sm.mu.Lock()
	sm.metaCache[session.Name] = &SessionMeta{
		Name:         session.Name,
		ModelID:      session.ModelID,
		MessageCount: len(session.Messages),
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
		ModTime:      time.Now(),
	}
	sm.mu.Unlock()

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

// List lists all available sessions (returns full sessions for backward compatibility)
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

// ListMeta returns lightweight metadata for all sessions (faster than List)
// Uses caching with file modification time validation
func (sm *SessionManager) ListMeta() ([]SessionMeta, error) {
	if _, err := os.Stat(sm.sessionsDir); os.IsNotExist(err) {
		return []SessionMeta{}, nil
	}

	entries, err := os.ReadDir(sm.sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	var result []SessionMeta
	currentFiles := make(map[string]bool)

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		name := entry.Name()[:len(entry.Name())-5] // Remove .json
		currentFiles[name] = true

		// Get file info for modification time
		info, err := entry.Info()
		if err != nil {
			continue
		}
		modTime := info.ModTime()

		// Check if we have valid cached metadata
		if cached, ok := sm.metaCache[name]; ok && cached.ModTime.Equal(modTime) {
			result = append(result, *cached)
			continue
		}

		// Need to load and parse the session
		filePath := filepath.Join(sm.sessionsDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		// Update cache
		meta := &SessionMeta{
			Name:         session.Name,
			ModelID:      session.ModelID,
			MessageCount: len(session.Messages),
			CreatedAt:    session.CreatedAt,
			UpdatedAt:    session.UpdatedAt,
			ModTime:      modTime,
		}
		sm.metaCache[name] = meta
		result = append(result, *meta)
	}

	// Clean up cache for deleted files
	for name := range sm.metaCache {
		if !currentFiles[name] {
			delete(sm.metaCache, name)
		}
	}

	// Sort by updated time (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result, nil
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

	// Remove from metadata cache
	sm.mu.Lock()
	delete(sm.metaCache, name)
	sm.mu.Unlock()

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
