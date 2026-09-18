package sessions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	ErrInvalidSessionName = errors.New("invalid session name")
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionExists      = errors.New("session name is already in use")
	ErrAccessDenied       = errors.New("session access denied")
)

// Message represents a single chat message
type Message struct {
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Session represents a conversation session
type Session struct {
	Name         string          `json:"name"`
	OwnerID      string          `json:"owner_id,omitempty"`
	ModelID      string          `json:"model_id"`
	Messages     []Message       `json:"messages"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Turn         *Turn           `json:"turn,omitempty"`
	TurnRequests map[string]bool `json:"turn_requests,omitempty"`
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
	mutationMu  sync.Mutex
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
	sm.mutationMu.Lock()
	defer sm.mutationMu.Unlock()
	return sm.save(session)
}

func (sm *SessionManager) save(session *Session) error {
	if session == nil {
		return fmt.Errorf("%w: session is nil", ErrInvalidSessionName)
	}
	filePath, err := sm.sessionFilePath(session.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sm.sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	session.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := writeSessionFileAtomic(filePath, data); err != nil {
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

// AppendExchange atomically appends a completed user/assistant exchange. It is
// used after inference succeeds so a stored conversation never contains half
// of a generated turn.
func (sm *SessionManager) AppendExchange(name, modelID, userContent, assistantContent string) (*Session, error) {
	if userContent == "" || assistantContent == "" {
		return nil, fmt.Errorf("user and assistant content are required")
	}
	sm.mutationMu.Lock()
	defer sm.mutationMu.Unlock()

	session, err := sm.Load(name)
	if err != nil {
		return nil, err
	}
	if modelID != "" {
		session.ModelID = modelID
	}
	session.AddMessage("user", userContent)
	session.AddMessage("assistant", assistantContent)
	if err := sm.save(session); err != nil {
		return nil, err
	}
	return session, nil
}

func writeSessionFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if n, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	} else if n != len(data) {
		tmp.Close()
		return fmt.Errorf("short session write: wrote %d of %d bytes", n, len(data))
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// Load loads a session from disk
func (sm *SessionManager) Load(name string) (*Session, error) {
	filePath, err := sm.sessionFilePath(name)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
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
	sm.mutationMu.Lock()
	defer sm.mutationMu.Unlock()
	return sm.delete(name)
}

func (sm *SessionManager) delete(name string) error {
	filePath, err := sm.sessionFilePath(name)
	if err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrSessionNotFound
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
	filePath, err := sm.sessionFilePath(name)
	if err != nil {
		return false
	}
	_, err = os.Stat(filePath)
	return err == nil
}

func (sm *SessionManager) sessionFilePath(name string) (string, error) {
	if name == "" || len(name) > 128 || name == "." || name == ".." ||
		filepath.Base(name) != name || strings.ContainsAny(name, `/\`) ||
		strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%w: %q", ErrInvalidSessionName, name)
	}

	path := filepath.Join(sm.sessionsDir, name+".json")
	rel, err := filepath.Rel(sm.sessionsDir, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", ErrInvalidSessionName, name)
	}
	return path, nil
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
