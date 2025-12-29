// Package users provides user management and authentication
package users

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig holds LDAP/Active Directory configuration
type LDAPConfig struct {
	// Server settings
	Server   string `json:"server"`    // LDAP server hostname or IP
	Port     int    `json:"port"`      // LDAP port (389 for LDAP, 636 for LDAPS)
	UseTLS   bool   `json:"use_tls"`   // Use TLS connection
	StartTLS bool   `json:"start_tls"` // Use StartTLS after connecting

	// Bind credentials for searching
	BindDN       string `json:"bind_dn"`       // DN for binding (e.g., cn=admin,dc=example,dc=com)
	BindPassword string `json:"bind_password"` // Password for bind DN

	// Search settings
	BaseDN        string `json:"base_dn"`        // Base DN for user search (e.g., dc=example,dc=com)
	UserFilter    string `json:"user_filter"`    // User search filter (e.g., (sAMAccountName=%s) for AD)
	UserAttribute string `json:"user_attribute"` // Attribute containing username (e.g., sAMAccountName for AD)

	// Group settings (optional)
	GroupBaseDN string          `json:"group_base_dn,omitempty"` // Base DN for group search
	GroupFilter string          `json:"group_filter,omitempty"`  // Group search filter
	AdminGroups []string        `json:"admin_groups,omitempty"`  // Groups that get admin role
	UserGroups  []string        `json:"user_groups,omitempty"`   // Groups that get user role
	DefaultRole Role            `json:"default_role,omitempty"`  // Default role if no group match
	RoleMapping map[string]Role `json:"role_mapping,omitempty"`  // Custom group to role mapping

	// Attribute mapping
	EmailAttribute       string `json:"email_attribute,omitempty"`        // Attribute for email (e.g., mail)
	DisplayNameAttribute string `json:"display_name_attribute,omitempty"` // Attribute for display name

	// Connection settings
	ConnectionTimeout  time.Duration `json:"connection_timeout,omitempty"`   // Connection timeout
	InsecureSkipVerify bool          `json:"insecure_skip_verify,omitempty"` // Skip TLS verification (not recommended)
}

// LDAPUser represents a user from LDAP
type LDAPUser struct {
	DN          string
	Username    string
	Email       string
	DisplayName string
	Groups      []string
}

// LDAPAuthProvider implements LDAP/Active Directory authentication
type LDAPAuthProvider struct {
	config LDAPConfig
	mu     sync.RWMutex
	cache  map[string]*ldapCacheEntry
}

type ldapCacheEntry struct {
	user      *LDAPUser
	expiresAt time.Time
}

// NewLDAPAuthProvider creates a new LDAP auth provider
func NewLDAPAuthProvider(config LDAPConfig) *LDAPAuthProvider {
	// Set defaults
	if config.Port == 0 {
		if config.UseTLS {
			config.Port = 636
		} else {
			config.Port = 389
		}
	}
	if config.UserFilter == "" {
		config.UserFilter = "(uid=%s)"
	}
	if config.UserAttribute == "" {
		config.UserAttribute = "uid"
	}
	if config.DefaultRole == "" {
		config.DefaultRole = RoleUser
	}
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 10 * time.Second
	}

	return &LDAPAuthProvider{
		config: config,
		cache:  make(map[string]*ldapCacheEntry),
	}
}

// NewLDAPAuthProviderForActiveDirectory creates an LDAP provider configured for Active Directory
func NewLDAPAuthProviderForActiveDirectory(server string, port int, baseDN, bindDN, bindPassword string) *LDAPAuthProvider {
	config := LDAPConfig{
		Server:               server,
		Port:                 port,
		UseTLS:               port == 636,
		BaseDN:               baseDN,
		BindDN:               bindDN,
		BindPassword:         bindPassword,
		UserFilter:           "(sAMAccountName=%s)",
		UserAttribute:        "sAMAccountName",
		EmailAttribute:       "mail",
		DisplayNameAttribute: "displayName",
		GroupFilter:          "(member=%s)",
		DefaultRole:          RoleUser,
		ConnectionTimeout:    10 * time.Second,
	}
	return NewLDAPAuthProvider(config)
}

// Authenticate authenticates a user against LDAP
func (p *LDAPAuthProvider) Authenticate(username, password string) (*LDAPUser, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password required")
	}

	// Connect to LDAP server
	conn, err := p.connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP server: %w", err)
	}
	defer conn.Close()

	// Bind with service account to search
	if p.config.BindDN != "" {
		err = conn.Bind(p.config.BindDN, p.config.BindPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to bind with service account: %w", err)
		}
	}

	// Search for the user
	searchRequest := ldap.NewSearchRequest(
		p.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(p.config.UserFilter, ldap.EscapeFilter(username)),
		[]string{"dn", p.config.UserAttribute, p.config.EmailAttribute, p.config.DisplayNameAttribute, "memberOf"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search for user: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	if len(result.Entries) > 1 {
		return nil, fmt.Errorf("multiple users found")
	}

	entry := result.Entries[0]
	userDN := entry.DN

	// Bind as the user to verify password
	err = conn.Bind(userDN, password)
	if err != nil {
		return nil, fmt.Errorf("authentication failed")
	}

	// Extract user info
	ldapUser := &LDAPUser{
		DN:          userDN,
		Username:    entry.GetAttributeValue(p.config.UserAttribute),
		Email:       entry.GetAttributeValue(p.config.EmailAttribute),
		DisplayName: entry.GetAttributeValue(p.config.DisplayNameAttribute),
		Groups:      entry.GetAttributeValues("memberOf"),
	}

	// Cache the user
	p.cacheUser(username, ldapUser)

	return ldapUser, nil
}

// GetRole determines the role for an LDAP user based on group membership
func (p *LDAPAuthProvider) GetRole(ldapUser *LDAPUser) Role {
	// Check custom role mapping first
	if p.config.RoleMapping != nil {
		for _, group := range ldapUser.Groups {
			groupCN := extractCN(group)
			if role, ok := p.config.RoleMapping[groupCN]; ok {
				return role
			}
			// Also check full DN
			if role, ok := p.config.RoleMapping[group]; ok {
				return role
			}
		}
	}

	// Check admin groups
	for _, adminGroup := range p.config.AdminGroups {
		for _, userGroup := range ldapUser.Groups {
			if matchesGroup(userGroup, adminGroup) {
				return RoleAdmin
			}
		}
	}

	// Check user groups
	for _, userGroupConfig := range p.config.UserGroups {
		for _, userGroup := range ldapUser.Groups {
			if matchesGroup(userGroup, userGroupConfig) {
				return RoleUser
			}
		}
	}

	return p.config.DefaultRole
}

// TestConnection tests the LDAP connection
func (p *LDAPAuthProvider) TestConnection() error {
	conn, err := p.connect()
	if err != nil {
		return err
	}
	defer conn.Close()

	// Try to bind with service account
	if p.config.BindDN != "" {
		err = conn.Bind(p.config.BindDN, p.config.BindPassword)
		if err != nil {
			return fmt.Errorf("bind failed: %w", err)
		}
	}

	return nil
}

// SearchUsers searches for users matching a pattern
func (p *LDAPAuthProvider) SearchUsers(pattern string, limit int) ([]*LDAPUser, error) {
	conn, err := p.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Bind with service account
	if p.config.BindDN != "" {
		err = conn.Bind(p.config.BindDN, p.config.BindPassword)
		if err != nil {
			return nil, fmt.Errorf("bind failed: %w", err)
		}
	}

	// Build search filter
	filter := fmt.Sprintf("(&%s(%s=*%s*))",
		strings.TrimPrefix(strings.TrimSuffix(p.config.UserFilter, ")"), "("),
		p.config.UserAttribute,
		ldap.EscapeFilter(pattern))

	// Replace %s placeholder
	filter = strings.ReplaceAll(filter, "%s", "*")

	searchRequest := ldap.NewSearchRequest(
		p.config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, limit, 0, false,
		filter,
		[]string{"dn", p.config.UserAttribute, p.config.EmailAttribute, p.config.DisplayNameAttribute, "memberOf"},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	users := make([]*LDAPUser, 0, len(result.Entries))
	for _, entry := range result.Entries {
		users = append(users, &LDAPUser{
			DN:          entry.DN,
			Username:    entry.GetAttributeValue(p.config.UserAttribute),
			Email:       entry.GetAttributeValue(p.config.EmailAttribute),
			DisplayName: entry.GetAttributeValue(p.config.DisplayNameAttribute),
			Groups:      entry.GetAttributeValues("memberOf"),
		})
	}

	return users, nil
}

// GetCachedUser gets a cached LDAP user
func (p *LDAPAuthProvider) GetCachedUser(username string) (*LDAPUser, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entry, ok := p.cache[strings.ToLower(username)]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.user, true
}

// connect establishes a connection to the LDAP server
func (p *LDAPAuthProvider) connect() (*ldap.Conn, error) {
	address := fmt.Sprintf("%s:%d", p.config.Server, p.config.Port)

	var conn *ldap.Conn
	var err error

	if p.config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: p.config.InsecureSkipVerify,
			ServerName:         p.config.Server,
		}
		conn, err = ldap.DialTLS("tcp", address, tlsConfig)
	} else {
		conn, err = ldap.Dial("tcp", address)
	}

	if err != nil {
		return nil, err
	}

	// StartTLS if configured
	if p.config.StartTLS && !p.config.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: p.config.InsecureSkipVerify,
			ServerName:         p.config.Server,
		}
		err = conn.StartTLS(tlsConfig)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}

// cacheUser caches an LDAP user for 5 minutes
func (p *LDAPAuthProvider) cacheUser(username string, user *LDAPUser) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cache[strings.ToLower(username)] = &ldapCacheEntry{
		user:      user,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
}

// CleanupCache removes expired cache entries
func (p *LDAPAuthProvider) CleanupCache() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, entry := range p.cache {
		if now.After(entry.expiresAt) {
			delete(p.cache, key)
		}
	}
}

// extractCN extracts the CN from a distinguished name
func extractCN(dn string) string {
	parts := strings.Split(dn, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "cn=") {
			return strings.TrimPrefix(part, "cn=")
		}
	}
	return dn
}

// matchesGroup checks if a user group matches a configured group
func matchesGroup(userGroup, configGroup string) bool {
	// Exact match
	if strings.EqualFold(userGroup, configGroup) {
		return true
	}
	// CN match
	userCN := extractCN(userGroup)
	configCN := extractCN(configGroup)
	return strings.EqualFold(userCN, configCN)
}

// LDAPAuthenticator provides integration between LDAP and UserStore
type LDAPAuthenticator struct {
	ldap       *LDAPAuthProvider
	userStore  *UserStore
	autoCreate bool
}

// NewLDAPAuthenticator creates a new LDAP authenticator that integrates with UserStore
func NewLDAPAuthenticator(ldap *LDAPAuthProvider, userStore *UserStore, autoCreate bool) *LDAPAuthenticator {
	return &LDAPAuthenticator{
		ldap:       ldap,
		userStore:  userStore,
		autoCreate: autoCreate,
	}
}

// Authenticate authenticates a user via LDAP and ensures they exist in the local store
func (a *LDAPAuthenticator) Authenticate(username, password string) (*User, error) {
	// Authenticate against LDAP
	ldapUser, err := a.ldap.Authenticate(username, password)
	if err != nil {
		return nil, err
	}

	// Check if user exists in local store
	user, exists := a.userStore.GetUserByUsername(ldapUser.Username)
	if exists {
		// Update role based on current LDAP groups
		newRole := a.ldap.GetRole(ldapUser)
		if user.Role != newRole {
			a.userStore.UpdateUser(user.ID, map[string]any{
				"role": newRole,
			})
			user.Role = newRole
		}
		// Update email if changed
		if ldapUser.Email != "" && user.Email != ldapUser.Email {
			a.userStore.UpdateUser(user.ID, map[string]any{
				"email": ldapUser.Email,
			})
		}
		return user, nil
	}

	// Auto-create user if enabled
	if a.autoCreate {
		role := a.ldap.GetRole(ldapUser)
		// Create user with a random password (they'll always auth via LDAP)
		newUser, _, err := a.userStore.CreateUser(ldapUser.Username, generateToken(), role)
		if err != nil {
			return nil, fmt.Errorf("failed to create local user: %w", err)
		}
		// Update email
		if ldapUser.Email != "" {
			a.userStore.UpdateUser(newUser.ID, map[string]any{
				"email": ldapUser.Email,
				"metadata": map[string]any{
					"ldap_dn":      ldapUser.DN,
					"ldap_synced":  true,
					"display_name": ldapUser.DisplayName,
				},
			})
		}
		return newUser, nil
	}

	return nil, fmt.Errorf("user not found in local store and auto-create is disabled")
}

// SyncUser syncs an LDAP user to the local store
func (a *LDAPAuthenticator) SyncUser(ldapUser *LDAPUser) (*User, error) {
	role := a.ldap.GetRole(ldapUser)

	// Check if user exists
	user, exists := a.userStore.GetUserByUsername(ldapUser.Username)
	if exists {
		// Update role and email
		a.userStore.UpdateUser(user.ID, map[string]any{
			"role":  role,
			"email": ldapUser.Email,
			"metadata": map[string]any{
				"ldap_dn":      ldapUser.DN,
				"ldap_synced":  true,
				"display_name": ldapUser.DisplayName,
			},
		})
		return user, nil
	}

	// Create new user
	newUser, _, err := a.userStore.CreateUser(ldapUser.Username, generateToken(), role)
	if err != nil {
		return nil, err
	}

	a.userStore.UpdateUser(newUser.ID, map[string]any{
		"email": ldapUser.Email,
		"metadata": map[string]any{
			"ldap_dn":      ldapUser.DN,
			"ldap_synced":  true,
			"display_name": ldapUser.DisplayName,
		},
	})

	return newUser, nil
}

// SyncAllUsers syncs all users from LDAP matching a pattern
func (a *LDAPAuthenticator) SyncAllUsers(pattern string, limit int) (int, error) {
	users, err := a.ldap.SearchUsers(pattern, limit)
	if err != nil {
		return 0, err
	}

	synced := 0
	for _, ldapUser := range users {
		_, err := a.SyncUser(ldapUser)
		if err == nil {
			synced++
		}
	}

	return synced, nil
}
