package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
	"github.com/takuphilchan/offgrid-llm/internal/users"
)

// handleCurrentUser returns the currently authenticated user
func (s *Server) handleCurrentUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := users.GetUser(r)
	if user == nil || user.ID == "guest" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auth_required": s.config.RequireAuth,
			"user":          nil,
			"authenticated": false,
			"guest":         user != nil && user.ID == "guest",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"user":          user.ToPublic(),
		"authenticated": true,
		"auth_required": s.config.RequireAuth,
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse path: /v1/users, /v1/users/{id}, /v1/users/{id}/regenerate-key, /v1/users/{id}/role
	path := strings.TrimPrefix(r.URL.Path, "/v1/users")
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	// Handle special sub-paths
	if len(parts) >= 2 && parts[0] != "" {
		userID := parts[0]
		action := parts[1]

		// Verify user exists
		_, ok := s.userStore.GetUser(userID)
		if !ok {
			http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
			return
		}

		switch action {
		case "regenerate-key":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			newKey, err := s.userStore.RegenerateAPIKey(userID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"api_key": newKey})
			return

		case "role":
			if r.Method != http.MethodPut {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Role users.Role `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
				return
			}
			if err := s.userStore.UpdateUser(userID, map[string]any{"role": req.Role}); err != nil {
				http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
				return
			}
			// Update quota limits for new role
			s.quotaManager.InitUserQuota(userID, req.Role)
			json.NewEncoder(w).Encode(map[string]string{"status": "role updated"})
			return

		default:
			http.Error(w, `{"error": "unknown action"}`, http.StatusNotFound)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		// List users or get specific user
		if len(parts) > 0 && parts[0] != "" {
			// Get specific user
			user, ok := s.userStore.GetUser(parts[0])
			if !ok {
				http.Error(w, `{"error": "user not found"}`, http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(user.ToPublic())
			return
		}
		// List all users
		userList := s.userStore.ListUsers()
		publicUsers := make([]users.UserPublic, 0, len(userList))
		for _, u := range userList {
			publicUsers = append(publicUsers, u.ToPublic())
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"users": publicUsers,
		})

	case http.MethodPost:
		// Create user
		var req struct {
			Username string     `json:"username"`
			Password string     `json:"password"`
			Role     users.Role `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}
		if req.Role == "" {
			req.Role = users.RoleUser
		}
		user, apiKey, err := s.userStore.CreateUser(req.Username, req.Password, req.Role)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		// Initialize quota for user
		s.quotaManager.InitUserQuota(user.ID, req.Role)
		json.NewEncoder(w).Encode(map[string]any{
			"user":    user.ToPublic(),
			"api_key": apiKey,
		})

	case http.MethodDelete:
		// Delete user
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, `{"error": "user id required"}`, http.StatusBadRequest)
			return
		}
		if err := s.userStore.DeleteUser(parts[0]); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		s.quotaManager.DeleteUserQuota(parts[0])
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	var user *users.User
	var authMethod string

	// Try LDAP authentication first if enabled
	if s.ldapAuth != nil {
		ldapUser, err := s.ldapAuth.Authenticate(req.Username, req.Password)
		if err == nil {
			user = ldapUser
			authMethod = "ldap"
		}
		// If LDAP fails, fall through to local auth
	}

	// Fall back to local authentication
	if user == nil {
		localUser, ok := s.userStore.ValidatePassword(req.Username, req.Password)
		if !ok {
			s.auditLog(audit.EventTypeAuth, "login_failed", req.Username, r.RemoteAddr, map[string]string{
				"reason": "invalid_credentials",
			}, false)
			http.Error(w, `{"error": "invalid credentials"}`, http.StatusUnauthorized)
			return
		}
		user = localUser
		authMethod = "local"
	}

	// Create session
	session, token, err := s.userStore.CreateSession(user.ID, r.RemoteAddr, r.UserAgent(), 24*time.Hour)
	if err != nil {
		http.Error(w, `{"error": "failed to create session"}`, http.StatusInternalServerError)
		return
	}

	// Audit successful login
	s.auditLog(audit.EventTypeAuth, "login_success", user.Username, r.RemoteAddr, map[string]string{
		"user_id":     user.ID,
		"role":        string(user.Role),
		"auth_method": authMethod,
	}, true)

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user":        user.ToPublic(),
		"token":       token,
		"expires_at":  session.ExpiresAt,
		"auth_method": authMethod,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get token from cookie or header
	token := ""
	if cookie, err := r.Cookie("session"); err == nil {
		token = cookie.Value
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		token = strings.TrimPrefix(auth, "Bearer ")
	}

	if token != "" {
		s.userStore.DeleteSession(token)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

// handleLDAPTest tests the LDAP connection
func (s *Server) handleLDAPTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if s.ldapAuth == nil {
		json.NewEncoder(w).Encode(map[string]any{
			"enabled": false,
			"status":  "LDAP authentication is not configured",
		})
		return
	}

	// For POST, test with provided credentials
	if r.Method == http.MethodPost {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
			return
		}

		user, err := s.ldapAuth.Authenticate(req.Username, req.Password)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"enabled": true,
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"enabled":  true,
			"success":  true,
			"username": user.Username,
			"role":     user.Role,
		})
		return
	}

	// GET just returns LDAP status
	json.NewEncoder(w).Encode(map[string]any{
		"enabled": true,
		"server":  s.config.LDAPServer,
		"port":    s.config.LDAPPort,
		"use_tls": s.config.LDAPUseTLS,
		"base_dn": s.config.LDAPBaseDN,
	})
}

// handleLDAPSync syncs users from LDAP (admin only)
func (s *Server) handleLDAPSync(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if s.ldapAuth == nil {
		http.Error(w, `{"error": "LDAP not configured"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Pattern string `json:"pattern"` // Search pattern (e.g., "*" for all)
		Limit   int    `json:"limit"`   // Max users to sync
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Pattern == "" {
		req.Pattern = "*"
	}
	if req.Limit == 0 {
		req.Limit = 100
	}

	synced, err := s.ldapAuth.SyncAllUsers(req.Pattern, req.Limit)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"users_synced": synced,
	})
}

func (s *Server) handleQuota(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	currentUser := users.GetUser(r)
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		if currentUser != nil {
			userID = currentUser.ID
		}
	}

	if s.config != nil && s.config.RequireAuth {
		if currentUser == nil {
			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if userID != currentUser.ID && !currentUser.HasPermission(users.PermissionAdmin) {
			http.Error(w, `{"error": "forbidden"}`, http.StatusForbidden)
			return
		}
	}

	if userID == "" {
		http.Error(w, `{"error": "user_id required"}`, http.StatusBadRequest)
		return
	}

	summary := s.quotaManager.GetUsageSummary(userID)
	json.NewEncoder(w).Encode(summary)
}

// handleQuotaManage handles setting, resetting, and removing quota limits
func (s *Server) handleQuotaManage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract user ID from path: /v1/quota/{user_id}
	path := strings.TrimPrefix(r.URL.Path, "/v1/quota/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, `{"error": "user_id required in path"}`, http.StatusBadRequest)
		return
	}
	userID := parts[0]

	switch r.Method {
	case http.MethodGet:
		// Get quota for specific user
		summary := s.quotaManager.GetUsageSummary(userID)
		json.NewEncoder(w).Encode(summary)

	case http.MethodPost:
		// Set or update a quota limit
		var req struct {
			Type   string `json:"type"`   // "requests", "tokens_input", "tokens_output", "tokens_total", etc
			Period string `json:"period"` // "minute", "hour", "day", "month"
			Limit  int64  `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Type == "" || req.Period == "" || req.Limit <= 0 {
			http.Error(w, `{"error": "type, period, and positive limit required"}`, http.StatusBadRequest)
			return
		}

		err := s.quotaManager.SetUserQuotaLimit(userID, users.QuotaType(req.Type), users.QuotaPeriod(req.Period), req.Limit)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "updated",
			"user_id": userID,
			"type":    req.Type,
			"period":  req.Period,
			"limit":   req.Limit,
		})

	case http.MethodDelete:
		// Check for action in query: ?action=reset to reset usage, otherwise remove limit
		action := r.URL.Query().Get("action")
		if action == "reset" {
			s.quotaManager.ResetUserQuota(userID)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "reset",
				"user_id": userID,
			})
			return
		}

		// Remove a specific quota limit
		qtype := r.URL.Query().Get("type")
		period := r.URL.Query().Get("period")
		if qtype == "" || period == "" {
			// Delete all quotas for user
			s.quotaManager.DeleteUserQuota(userID)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "deleted",
				"user_id": userID,
			})
			return
		}

		s.quotaManager.RemoveUserQuotaLimit(userID, users.QuotaType(qtype), users.QuotaPeriod(period))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "removed",
			"user_id": userID,
			"type":    qtype,
			"period":  period,
		})

	default:
		http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
	}
}
