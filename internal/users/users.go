package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Role represents a user role
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleUser   Role = "user"
	RoleGuest  Role = "guest"
	RoleViewer Role = "viewer"
)

// Permission represents a permission
type Permission string

const (
	PermissionChat         Permission = "chat"
	PermissionModels       Permission = "models"
	PermissionModelsManage Permission = "models:manage"
	PermissionRAG          Permission = "rag"
	PermissionRAGManage    Permission = "rag:manage"
	PermissionSessions     Permission = "sessions"
	PermissionSessionsAll  Permission = "sessions:all"
	PermissionAdmin        Permission = "admin"
	PermissionStats        Permission = "stats"
)

// RolePermissions maps roles to their permissions
var RolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermissionChat, PermissionModels, PermissionModelsManage,
		PermissionRAG, PermissionRAGManage, PermissionSessions,
		PermissionSessionsAll, PermissionAdmin, PermissionStats,
	},
	RoleUser: {
		PermissionChat, PermissionModels, PermissionRAG,
		PermissionSessions, PermissionStats,
	},
	RoleViewer: {
		PermissionChat, PermissionModels, PermissionStats,
	},
	RoleGuest: {
		PermissionChat,
	},
}

// User represents a user
type User struct {
	ID           string         `json:"id"`
	Username     string         `json:"username"`
	Email        string         `json:"email,omitempty"`
	PasswordHash string         `json:"-"` // Never expose
	Role         Role           `json:"role"`
	APIKey       string         `json:"-"` // Never expose
	APIKeyHash   string         `json:"-"` // Stored hash
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
	Disabled     bool           `json:"disabled"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// UserPublic is the public representation of a user
type UserPublic struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email,omitempty"`
	Role        Role       `json:"role"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// ToPublic converts a User to UserPublic
func (u *User) ToPublic() UserPublic {
	return UserPublic{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt,
		LastLoginAt: u.LastLoginAt,
	}
}

// HasPermission checks if the user has a permission
func (u *User) HasPermission(perm Permission) bool {
	if u.Disabled {
		return false
	}

	perms, ok := RolePermissions[u.Role]
	if !ok {
		return false
	}

	for _, p := range perms {
		if p == perm || p == PermissionAdmin {
			return true
		}
	}
	return false
}

// Session represents an auth session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"-"`
	TokenHash string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IP        string    `json:"ip,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
}

// IsExpired checks if the session is expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// UserStore manages users
type UserStore struct {
	mu         sync.RWMutex
	users      map[string]*User    // ID -> User
	byUsername map[string]string   // Username -> ID
	byAPIKey   map[string]string   // APIKeyHash -> ID
	sessions   map[string]*Session // TokenHash -> Session
	dataDir    string
}

// NewUserStore creates a new user store
func NewUserStore(dataDir string) *UserStore {
	store := &UserStore{
		users:      make(map[string]*User),
		byUsername: make(map[string]string),
		byAPIKey:   make(map[string]string),
		sessions:   make(map[string]*Session),
		dataDir:    dataDir,
	}

	// Load from disk if exists
	store.load()

	return store
}

// CreateUser creates a new user
func (s *UserStore) CreateUser(username, password string, role Role) (*User, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if username exists
	if _, exists := s.byUsername[strings.ToLower(username)]; exists {
		return nil, "", fmt.Errorf("username already exists")
	}

	// Generate ID
	id := generateID()

	// Hash password
	passwordHash := hashPassword(password)

	// Generate API key
	apiKey := generateAPIKey()
	apiKeyHash := hashString(apiKey)

	user := &User{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		APIKeyHash:   apiKeyHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Metadata:     make(map[string]any),
	}

	s.users[id] = user
	s.byUsername[strings.ToLower(username)] = id
	s.byAPIKey[apiKeyHash] = id

	s.save()

	return user, apiKey, nil
}

// GetUser gets a user by ID
func (s *UserStore) GetUser(id string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	return user, ok
}

// GetUserByUsername gets a user by username
func (s *UserStore) GetUserByUsername(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byUsername[strings.ToLower(username)]
	if !ok {
		return nil, false
	}
	return s.users[id], true
}

// GetUserByAPIKey gets a user by API key
func (s *UserStore) GetUserByAPIKey(apiKey string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hash := hashString(apiKey)
	id, ok := s.byAPIKey[hash]
	if !ok {
		return nil, false
	}
	return s.users[id], true
}

// UpdateUser updates a user
func (s *UserStore) UpdateUser(id string, updates map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return fmt.Errorf("user not found")
	}

	if email, ok := updates["email"].(string); ok {
		user.Email = email
	}
	if role, ok := updates["role"].(Role); ok {
		user.Role = role
	}
	if disabled, ok := updates["disabled"].(bool); ok {
		user.Disabled = disabled
	}
	if metadata, ok := updates["metadata"].(map[string]any); ok {
		for k, v := range metadata {
			user.Metadata[k] = v
		}
	}

	user.UpdatedAt = time.Now()
	s.save()

	return nil
}

// UpdatePassword updates a user's password
func (s *UserStore) UpdatePassword(id, newPassword string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return fmt.Errorf("user not found")
	}

	user.PasswordHash = hashPassword(newPassword)
	user.UpdatedAt = time.Now()
	s.save()

	return nil
}

// RegenerateAPIKey regenerates a user's API key
func (s *UserStore) RegenerateAPIKey(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return "", fmt.Errorf("user not found")
	}

	// Remove old API key mapping
	delete(s.byAPIKey, user.APIKeyHash)

	// Generate new API key
	apiKey := generateAPIKey()
	user.APIKeyHash = hashString(apiKey)
	user.UpdatedAt = time.Now()

	s.byAPIKey[user.APIKeyHash] = id
	s.save()

	return apiKey, nil
}

// DeleteUser deletes a user
func (s *UserStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return fmt.Errorf("user not found")
	}

	delete(s.byUsername, strings.ToLower(user.Username))
	delete(s.byAPIKey, user.APIKeyHash)
	delete(s.users, id)

	// Delete user sessions
	for hash, session := range s.sessions {
		if session.UserID == id {
			delete(s.sessions, hash)
		}
	}

	s.save()
	return nil
}

// ListUsers lists all users
func (s *UserStore) ListUsers() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	return users
}

// ValidatePassword validates a username and password
func (s *UserStore) ValidatePassword(username, password string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.byUsername[strings.ToLower(username)]
	if !ok {
		return nil, false
	}

	user := s.users[id]
	if user.Disabled {
		return nil, false
	}

	if !verifyPassword(password, user.PasswordHash) {
		return nil, false
	}

	return user, true
}

// CreateSession creates a new session for a user
func (s *UserStore) CreateSession(userID, ip, userAgent string, duration time.Duration) (*Session, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[userID]; !ok {
		return nil, "", fmt.Errorf("user not found")
	}

	token := generateToken()
	tokenHash := hashString(token)

	session := &Session{
		ID:        generateID(),
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
		IP:        ip,
		UserAgent: userAgent,
	}

	s.sessions[tokenHash] = session

	// Update last login
	if user, ok := s.users[userID]; ok {
		now := time.Now()
		user.LastLoginAt = &now
	}

	s.save()

	return session, token, nil
}

// ValidateSession validates a session token
func (s *UserStore) ValidateSession(token string) (*Session, *User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokenHash := hashString(token)
	session, ok := s.sessions[tokenHash]
	if !ok {
		return nil, nil, false
	}

	if session.IsExpired() {
		return nil, nil, false
	}

	user, ok := s.users[session.UserID]
	if !ok || user.Disabled {
		return nil, nil, false
	}

	return session, user, true
}

// DeleteSession deletes a session
func (s *UserStore) DeleteSession(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenHash := hashString(token)
	delete(s.sessions, tokenHash)
	s.save()
}

// CleanupExpiredSessions removes expired sessions
func (s *UserStore) CleanupExpiredSessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for hash, session := range s.sessions {
		if session.IsExpired() {
			delete(s.sessions, hash)
			count++
		}
	}

	if count > 0 {
		s.save()
	}
	return count
}

// save persists the store to disk
func (s *UserStore) save() {
	if s.dataDir == "" {
		return
	}

	data := struct {
		Users    map[string]*User    `json:"users"`
		Sessions map[string]*Session `json:"sessions"`
	}{
		Users:    s.users,
		Sessions: s.sessions,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(s.dataDir, "users.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, jsonData, 0600)
}

// load loads the store from disk
func (s *UserStore) load() {
	if s.dataDir == "" {
		return
	}

	path := filepath.Join(s.dataDir, "users.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var stored struct {
		Users    map[string]*User    `json:"users"`
		Sessions map[string]*Session `json:"sessions"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	s.users = stored.Users
	s.sessions = stored.Sessions

	// Rebuild indexes
	s.byUsername = make(map[string]string)
	s.byAPIKey = make(map[string]string)
	for id, user := range s.users {
		s.byUsername[strings.ToLower(user.Username)] = id
		if user.APIKeyHash != "" {
			s.byAPIKey[user.APIKeyHash] = id
		}
	}
}

// Middleware provides authentication middleware
type Middleware struct {
	store        *UserStore
	bypassPaths  map[string]bool
	requireAuth  bool
	guestEnabled bool
}

// NewMiddleware creates a new auth middleware
func NewMiddleware(store *UserStore) *Middleware {
	return &Middleware{
		store: store,
		bypassPaths: map[string]bool{
			"/health":      true,
			"/":            true,
			"/ui/":         true,
			"/v1/login":    true,
			"/v1/register": true,
		},
		requireAuth:  false, // Default: no auth required
		guestEnabled: true,
	}
}

// SetRequireAuth sets whether authentication is required
func (m *Middleware) SetRequireAuth(required bool) {
	m.requireAuth = required
}

// SetGuestEnabled sets whether guest access is enabled
func (m *Middleware) SetGuestEnabled(enabled bool) {
	m.guestEnabled = enabled
}

// AddBypassPath adds a path that bypasses authentication
func (m *Middleware) AddBypassPath(path string) {
	m.bypassPaths[path] = true
}

// Wrap wraps an HTTP handler with authentication
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check bypass paths
		for path := range m.bypassPaths {
			if strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Try to authenticate
		user := m.authenticate(r)

		if user == nil && m.requireAuth {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if user == nil && m.guestEnabled {
			// Create guest context
			user = &User{
				ID:       "guest",
				Username: "guest",
				Role:     RoleGuest,
			}
		}

		// Add user to context
		if user != nil {
			ctx := context.WithValue(r.Context(), userContextKey, user)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// authenticate tries to authenticate from request
func (m *Middleware) authenticate(r *http.Request) *User {
	// Try API key from header
	if apiKey := r.Header.Get("Authorization"); apiKey != "" {
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		if user, ok := m.store.GetUserByAPIKey(apiKey); ok && !user.Disabled {
			return user
		}
	}

	// Try API key from query
	if apiKey := r.URL.Query().Get("api_key"); apiKey != "" {
		if user, ok := m.store.GetUserByAPIKey(apiKey); ok && !user.Disabled {
			return user
		}
	}

	// Try session cookie
	if cookie, err := r.Cookie("session"); err == nil {
		if _, user, ok := m.store.ValidateSession(cookie.Value); ok {
			return user
		}
	}

	return nil
}

// Context key for user
type contextKey string

const userContextKey contextKey = "user"

// GetUser gets the user from request context
func GetUser(r *http.Request) *User {
	if user, ok := r.Context().Value(userContextKey).(*User); ok {
		return user
	}
	return nil
}

// GetUserID gets the user ID from request context
func GetUserID(r *http.Request) string {
	if user := GetUser(r); user != nil {
		return user.ID
	}
	return ""
}

// RequirePermission returns a middleware that requires a permission
func RequirePermission(perm Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			if user == nil || !user.HasPermission(perm) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Helper functions

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "og_" + base64.RawURLEncoding.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func verifyPassword(password, hash string) bool {
	computed := hashPassword(password)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// UserSessionIsolation manages per-user session isolation
type UserSessionIsolation struct {
	mu       sync.RWMutex
	sessions map[string]map[string]any // UserID -> SessionID -> Data
}

// NewUserSessionIsolation creates a new isolation manager
func NewUserSessionIsolation() *UserSessionIsolation {
	return &UserSessionIsolation{
		sessions: make(map[string]map[string]any),
	}
}

// GetUserSessions gets all session data for a user
func (i *UserSessionIsolation) GetUserSessions(userID string) map[string]any {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if sessions, ok := i.sessions[userID]; ok {
		// Return a copy
		result := make(map[string]any)
		for k, v := range sessions {
			result[k] = v
		}
		return result
	}
	return make(map[string]any)
}

// SetSessionData sets session data for a user
func (i *UserSessionIsolation) SetSessionData(userID, sessionID string, data any) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.sessions[userID] == nil {
		i.sessions[userID] = make(map[string]any)
	}
	i.sessions[userID][sessionID] = data
}

// GetSessionData gets session data for a user
func (i *UserSessionIsolation) GetSessionData(userID, sessionID string) (any, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if sessions, ok := i.sessions[userID]; ok {
		if data, ok := sessions[sessionID]; ok {
			return data, true
		}
	}
	return nil, false
}

// DeleteSessionData deletes session data
func (i *UserSessionIsolation) DeleteSessionData(userID, sessionID string) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if sessions, ok := i.sessions[userID]; ok {
		delete(sessions, sessionID)
	}
}

// DeleteUserData deletes all data for a user
func (i *UserSessionIsolation) DeleteUserData(userID string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.sessions, userID)
}
