package users

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// QuotaType represents a type of quota
type QuotaType string

const (
	QuotaTypeRequests     QuotaType = "requests"
	QuotaTypeTokensInput  QuotaType = "tokens_input"
	QuotaTypeTokensOutput QuotaType = "tokens_output"
	QuotaTypeTokensTotal  QuotaType = "tokens_total"
	QuotaTypeDocuments    QuotaType = "documents"
	QuotaTypeSessions     QuotaType = "sessions"
	QuotaTypeRAGQueries   QuotaType = "rag_queries"
	QuotaTypeEmbeddings   QuotaType = "embeddings"
)

// QuotaPeriod represents a quota time period
type QuotaPeriod string

const (
	QuotaPeriodMinute QuotaPeriod = "minute"
	QuotaPeriodHour   QuotaPeriod = "hour"
	QuotaPeriodDay    QuotaPeriod = "day"
	QuotaPeriodMonth  QuotaPeriod = "month"
)

// QuotaLimit defines a quota limit
type QuotaLimit struct {
	Type    QuotaType   `json:"type"`
	Period  QuotaPeriod `json:"period"`
	Limit   int64       `json:"limit"`
	Current int64       `json:"current"`
	Reset   time.Time   `json:"reset"`
}

// IsExceeded checks if the quota is exceeded
func (q *QuotaLimit) IsExceeded() bool {
	return q.Current >= q.Limit
}

// Remaining returns the remaining quota
func (q *QuotaLimit) Remaining() int64 {
	rem := q.Limit - q.Current
	if rem < 0 {
		return 0
	}
	return rem
}

// Increment increments the quota usage
func (q *QuotaLimit) Increment(amount int64) {
	q.Current += amount
}

// QuotaPlan defines a set of quota limits
type QuotaPlan struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Limits      []QuotaLimit `json:"limits"`
}

// DefaultQuotaPlans returns the default quota plans
func DefaultQuotaPlans() map[Role]*QuotaPlan {
	return map[Role]*QuotaPlan{
		RoleGuest: {
			Name:        "Guest",
			Description: "Limited guest access",
			Limits: []QuotaLimit{
				{Type: QuotaTypeRequests, Period: QuotaPeriodHour, Limit: 20},
				{Type: QuotaTypeTokensTotal, Period: QuotaPeriodDay, Limit: 10000},
			},
		},
		RoleViewer: {
			Name:        "Viewer",
			Description: "Read-only access with moderate limits",
			Limits: []QuotaLimit{
				{Type: QuotaTypeRequests, Period: QuotaPeriodHour, Limit: 100},
				{Type: QuotaTypeTokensTotal, Period: QuotaPeriodDay, Limit: 50000},
			},
		},
		RoleUser: {
			Name:        "User",
			Description: "Standard user access",
			Limits: []QuotaLimit{
				{Type: QuotaTypeRequests, Period: QuotaPeriodMinute, Limit: 30},
				{Type: QuotaTypeRequests, Period: QuotaPeriodHour, Limit: 500},
				{Type: QuotaTypeTokensInput, Period: QuotaPeriodDay, Limit: 500000},
				{Type: QuotaTypeTokensOutput, Period: QuotaPeriodDay, Limit: 100000},
				{Type: QuotaTypeDocuments, Period: QuotaPeriodDay, Limit: 50},
				{Type: QuotaTypeSessions, Period: QuotaPeriodDay, Limit: 20},
				{Type: QuotaTypeRAGQueries, Period: QuotaPeriodHour, Limit: 100},
			},
		},
		RoleAdmin: {
			Name:        "Admin",
			Description: "Unlimited access",
			Limits:      []QuotaLimit{}, // No limits
		},
	}
}

// UserQuota tracks a user's quota usage
type UserQuota struct {
	UserID    string       `json:"user_id"`
	Limits    []QuotaLimit `json:"limits"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// GetLimit gets a specific quota limit
func (uq *UserQuota) GetLimit(qtype QuotaType, period QuotaPeriod) *QuotaLimit {
	for i := range uq.Limits {
		if uq.Limits[i].Type == qtype && uq.Limits[i].Period == period {
			return &uq.Limits[i]
		}
	}
	return nil
}

// CheckAndIncrement checks if quota allows and increments
func (uq *UserQuota) CheckAndIncrement(qtype QuotaType, amount int64) (bool, *QuotaLimit) {
	for i := range uq.Limits {
		if uq.Limits[i].Type == qtype {
			// Reset if period expired
			uq.resetIfExpired(&uq.Limits[i])

			if uq.Limits[i].Current+amount > uq.Limits[i].Limit {
				return false, &uq.Limits[i]
			}
		}
	}

	// All checks passed, increment
	for i := range uq.Limits {
		if uq.Limits[i].Type == qtype {
			uq.Limits[i].Current += amount
		}
	}
	uq.UpdatedAt = time.Now()
	return true, nil
}

// resetIfExpired resets a quota if its period has expired
func (uq *UserQuota) resetIfExpired(limit *QuotaLimit) {
	now := time.Now()
	if now.After(limit.Reset) {
		limit.Current = 0
		limit.Reset = calculateNextReset(limit.Period)
	}
}

// QuotaManager manages user quotas
type QuotaManager struct {
	mu      sync.RWMutex
	quotas  map[string]*UserQuota // UserID -> UserQuota
	plans   map[Role]*QuotaPlan
	dataDir string
}

// NewQuotaManager creates a new quota manager
func NewQuotaManager(dataDir string) *QuotaManager {
	qm := &QuotaManager{
		quotas:  make(map[string]*UserQuota),
		plans:   DefaultQuotaPlans(),
		dataDir: dataDir,
	}
	qm.load()
	return qm
}

// SetPlan sets a quota plan for a role
func (qm *QuotaManager) SetPlan(role Role, plan *QuotaPlan) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.plans[role] = plan
}

// GetPlan gets a quota plan for a role
func (qm *QuotaManager) GetPlan(role Role) *QuotaPlan {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.plans[role]
}

// InitUserQuota initializes quotas for a user based on their role
func (qm *QuotaManager) InitUserQuota(userID string, role Role) *UserQuota {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	plan := qm.plans[role]
	if plan == nil {
		// Default to guest limits
		plan = qm.plans[RoleGuest]
	}

	// Copy limits from plan
	limits := make([]QuotaLimit, len(plan.Limits))
	for i, limit := range plan.Limits {
		limits[i] = QuotaLimit{
			Type:   limit.Type,
			Period: limit.Period,
			Limit:  limit.Limit,
			Reset:  calculateNextReset(limit.Period),
		}
	}

	quota := &UserQuota{
		UserID:    userID,
		Limits:    limits,
		UpdatedAt: time.Now(),
	}

	qm.quotas[userID] = quota
	qm.save()

	return quota
}

// GetUserQuota gets a user's quota
func (qm *QuotaManager) GetUserQuota(userID string) *UserQuota {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.quotas[userID]
}

// CheckQuota checks if a user has available quota
func (qm *QuotaManager) CheckQuota(userID string, qtype QuotaType, amount int64) (bool, *QuotaLimit) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, ok := qm.quotas[userID]
	if !ok {
		// No quota set, allow
		return true, nil
	}

	allowed, exceededLimit := quota.CheckAndIncrement(qtype, amount)
	if allowed {
		qm.save()
	}
	return allowed, exceededLimit
}

// RecordUsage records usage for a user
func (qm *QuotaManager) RecordUsage(userID string, qtype QuotaType, amount int64) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, ok := qm.quotas[userID]
	if !ok {
		return
	}

	for i := range quota.Limits {
		if quota.Limits[i].Type == qtype {
			quota.resetIfExpired(&quota.Limits[i])
			quota.Limits[i].Current += amount
		}
	}
	quota.UpdatedAt = time.Now()
	qm.save()
}

// GetUsageSummary gets a usage summary for a user
func (qm *QuotaManager) GetUsageSummary(userID string) map[string]any {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	quota, ok := qm.quotas[userID]
	if !ok {
		return map[string]any{
			"user_id": userID,
			"quotas":  []any{},
		}
	}

	quotas := make([]map[string]any, 0, len(quota.Limits))
	for _, limit := range quota.Limits {
		quotas = append(quotas, map[string]any{
			"type":      limit.Type,
			"period":    limit.Period,
			"limit":     limit.Limit,
			"current":   limit.Current,
			"remaining": limit.Remaining(),
			"exceeded":  limit.IsExceeded(),
			"reset":     limit.Reset,
		})
	}

	return map[string]any{
		"user_id": userID,
		"quotas":  quotas,
	}
}

// ResetUserQuota resets a user's quota
func (qm *QuotaManager) ResetUserQuota(userID string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if quota, ok := qm.quotas[userID]; ok {
		for i := range quota.Limits {
			quota.Limits[i].Current = 0
			quota.Limits[i].Reset = calculateNextReset(quota.Limits[i].Period)
		}
		quota.UpdatedAt = time.Now()
		qm.save()
	}
}

// SetUserQuotaLimit sets or updates a specific quota limit for a user
func (qm *QuotaManager) SetUserQuotaLimit(userID string, qtype QuotaType, period QuotaPeriod, limit int64) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, ok := qm.quotas[userID]
	if !ok {
		// Initialize quota for this user
		quota = &UserQuota{
			UserID:    userID,
			Limits:    []QuotaLimit{},
			UpdatedAt: time.Now(),
		}
		qm.quotas[userID] = quota
	}

	// Find existing limit or create new one
	found := false
	for i := range quota.Limits {
		if quota.Limits[i].Type == qtype && quota.Limits[i].Period == period {
			quota.Limits[i].Limit = limit
			found = true
			break
		}
	}

	if !found {
		quota.Limits = append(quota.Limits, QuotaLimit{
			Type:    qtype,
			Period:  period,
			Limit:   limit,
			Current: 0,
			Reset:   calculateNextReset(period),
		})
	}

	quota.UpdatedAt = time.Now()
	qm.save()
	return nil
}

// RemoveUserQuotaLimit removes a specific quota limit for a user
func (qm *QuotaManager) RemoveUserQuotaLimit(userID string, qtype QuotaType, period QuotaPeriod) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	quota, ok := qm.quotas[userID]
	if !ok {
		return
	}

	newLimits := make([]QuotaLimit, 0, len(quota.Limits))
	for _, l := range quota.Limits {
		if !(l.Type == qtype && l.Period == period) {
			newLimits = append(newLimits, l)
		}
	}
	quota.Limits = newLimits
	quota.UpdatedAt = time.Now()
	qm.save()
}

// DeleteUserQuota deletes a user's quota
func (qm *QuotaManager) DeleteUserQuota(userID string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	delete(qm.quotas, userID)
	qm.save()
}

// save persists quotas to disk
func (qm *QuotaManager) save() {
	if qm.dataDir == "" {
		return
	}

	data, err := json.MarshalIndent(qm.quotas, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(qm.dataDir, "quotas.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, data, 0600)
}

// load loads quotas from disk
func (qm *QuotaManager) load() {
	if qm.dataDir == "" {
		return
	}

	path := filepath.Join(qm.dataDir, "quotas.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	json.Unmarshal(data, &qm.quotas)
}

// calculateNextReset calculates the next reset time for a period
func calculateNextReset(period QuotaPeriod) time.Time {
	now := time.Now()
	switch period {
	case QuotaPeriodMinute:
		return now.Add(time.Minute)
	case QuotaPeriodHour:
		return now.Add(time.Hour)
	case QuotaPeriodDay:
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	case QuotaPeriodMonth:
		return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	default:
		return now.Add(time.Hour)
	}
}

// QuotaError represents a quota exceeded error
type QuotaError struct {
	UserID  string      `json:"user_id"`
	Type    QuotaType   `json:"type"`
	Period  QuotaPeriod `json:"period"`
	Limit   int64       `json:"limit"`
	Current int64       `json:"current"`
	Reset   time.Time   `json:"reset"`
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("quota exceeded: %s (%s) limit %d, current %d, resets at %s",
		e.Type, e.Period, e.Limit, e.Current, e.Reset.Format(time.RFC3339))
}

// NewQuotaError creates a quota error from a limit
func NewQuotaError(userID string, limit *QuotaLimit) *QuotaError {
	return &QuotaError{
		UserID:  userID,
		Type:    limit.Type,
		Period:  limit.Period,
		Limit:   limit.Limit,
		Current: limit.Current,
		Reset:   limit.Reset,
	}
}
