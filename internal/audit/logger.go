// Package audit provides security audit logging for air-gapped and compliance environments.
// All actions are logged locally with tamper-evident signatures.
package audit

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// EventType represents the type of audit event
type EventType string

const (
	EventTypeAuth        EventType = "AUTH"
	EventTypeQuery       EventType = "QUERY"
	EventTypeModel       EventType = "MODEL"
	EventTypeConfig      EventType = "CONFIG"
	EventTypeSystem      EventType = "SYSTEM"
	EventTypeAccess      EventType = "ACCESS"
	EventTypeError       EventType = "ERROR"
	EventTypeAdmin       EventType = "ADMIN"
	EventTypeP2P         EventType = "P2P"
	EventTypeMaintenance EventType = "MAINTENANCE"
)

// EventSeverity represents the severity of an audit event
type EventSeverity string

const (
	SeverityInfo     EventSeverity = "INFO"
	SeverityWarning  EventSeverity = "WARNING"
	SeverityCritical EventSeverity = "CRITICAL"
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	ID           string            `json:"id"`
	Timestamp    string            `json:"timestamp"`
	Type         EventType         `json:"type"`
	Severity     EventSeverity     `json:"severity"`
	Action       string            `json:"action"`
	User         string            `json:"user,omitempty"`
	Source       string            `json:"source,omitempty"` // IP or P2P node
	Target       string            `json:"target,omitempty"` // Resource affected
	Details      map[string]string `json:"details,omitempty"`
	Success      bool              `json:"success"`
	ErrorMessage string            `json:"error,omitempty"`
	PrevHash     string            `json:"prev_hash"` // Hash of previous entry for chain integrity
	Hash         string            `json:"hash"`      // HMAC of this entry
}

// AuditLogger provides secure audit logging
type AuditLogger struct {
	mu           sync.Mutex
	logDir       string
	currentFile  *os.File
	currentPath  string
	maxFileSize  int64
	lastHash     string
	hmacKey      []byte
	eventCounter uint64
	hostname     string
}

// AuditConfig configures the audit logger
type AuditConfig struct {
	LogDir        string `json:"log_dir"`
	MaxFileSize   int64  `json:"max_file_size_mb"` // Max size per file in MB
	RetentionDays int    `json:"retention_days"`
	HMACSecret    string `json:"hmac_secret"` // For tamper detection
}

// DefaultAuditConfig returns sensible defaults
func DefaultAuditConfig(dataDir string) AuditConfig {
	return AuditConfig{
		LogDir:        filepath.Join(dataDir, "audit"),
		MaxFileSize:   50, // 50MB per file
		RetentionDays: 365,
		HMACSecret:    "", // Generate on first run
	}
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(config AuditConfig) (*AuditLogger, error) {
	if err := os.MkdirAll(config.LogDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}

	hostname, _ := os.Hostname()

	// Use provided secret or generate one
	var hmacKey []byte
	if config.HMACSecret != "" {
		hmacKey = []byte(config.HMACSecret)
	} else {
		// Load or create HMAC key
		keyPath := filepath.Join(config.LogDir, ".hmac_key")
		if data, err := os.ReadFile(keyPath); err == nil {
			hmacKey = data
		} else {
			// Generate new key using cryptographically secure random
			hmacKey = make([]byte, 32)
			if _, err := rand.Read(hmacKey); err != nil {
				return nil, fmt.Errorf("failed to generate HMAC key: %w", err)
			}
			os.WriteFile(keyPath, hmacKey, 0600)
		}
	}

	logger := &AuditLogger{
		logDir:      config.LogDir,
		maxFileSize: config.MaxFileSize * 1024 * 1024,
		hmacKey:     hmacKey,
		hostname:    hostname,
	}

	// Load last hash from most recent log file
	logger.lastHash = logger.loadLastHash()

	// Open current log file
	if err := logger.rotateIfNeeded(); err != nil {
		return nil, err
	}

	return logger, nil
}

// Log logs an audit event
func (l *AuditLogger) Log(eventType EventType, severity EventSeverity, action string, details map[string]string) error {
	return l.LogEvent(AuditEvent{
		Type:     eventType,
		Severity: severity,
		Action:   action,
		Details:  details,
		Success:  true,
	})
}

// LogWithUser logs an audit event with user information
func (l *AuditLogger) LogWithUser(eventType EventType, severity EventSeverity, action, user, source string, details map[string]string) error {
	return l.LogEvent(AuditEvent{
		Type:     eventType,
		Severity: severity,
		Action:   action,
		User:     user,
		Source:   source,
		Details:  details,
		Success:  true,
	})
}

// LogError logs a failed action
func (l *AuditLogger) LogError(eventType EventType, action string, err error, details map[string]string) error {
	return l.LogEvent(AuditEvent{
		Type:         eventType,
		Severity:     SeverityWarning,
		Action:       action,
		Details:      details,
		Success:      false,
		ErrorMessage: err.Error(),
	})
}

// LogCritical logs a critical security event
func (l *AuditLogger) LogCritical(action, user, source string, details map[string]string) error {
	return l.LogEvent(AuditEvent{
		Type:     EventTypeAccess,
		Severity: SeverityCritical,
		Action:   action,
		User:     user,
		Source:   source,
		Details:  details,
		Success:  false,
	})
}

// LogEvent logs a complete audit event
func (l *AuditLogger) LogEvent(event AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Fill in automatic fields
	l.eventCounter++
	event.ID = fmt.Sprintf("%s-%d-%d", l.hostname, time.Now().Unix(), l.eventCounter)
	event.Timestamp = time.Now().Format(time.RFC3339Nano)
	event.PrevHash = l.lastHash

	// Calculate HMAC hash for tamper detection
	event.Hash = l.calculateHash(event)

	// Rotate if needed
	if err := l.rotateIfNeeded(); err != nil {
		return err
	}

	// Write event
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	if _, err := l.currentFile.Write(append(data, '\n')); err != nil {
		return err
	}

	// Sync to ensure durability
	l.currentFile.Sync()

	// Update last hash
	l.lastHash = event.Hash

	return nil
}

// calculateHash calculates an HMAC-SHA256 hash of the event
func (l *AuditLogger) calculateHash(event AuditEvent) string {
	// Create a copy without the hash field
	eventCopy := event
	eventCopy.Hash = ""

	data, _ := json.Marshal(eventCopy)

	h := hmac.New(sha256.New, l.hmacKey)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// rotateIfNeeded rotates the log file if it exceeds max size
func (l *AuditLogger) rotateIfNeeded() error {
	if l.currentFile != nil {
		info, err := l.currentFile.Stat()
		if err == nil && info.Size() < l.maxFileSize {
			return nil
		}
		l.currentFile.Close()
	}

	// Create new log file
	filename := fmt.Sprintf("audit_%s.jsonl", time.Now().Format("2006-01-02_15-04-05"))
	l.currentPath = filepath.Join(l.logDir, filename)

	file, err := os.OpenFile(l.currentPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	l.currentFile = file
	return nil
}

// loadLastHash loads the hash of the last event from the most recent log file
func (l *AuditLogger) loadLastHash() string {
	files, err := filepath.Glob(filepath.Join(l.logDir, "audit_*.jsonl"))
	if err != nil || len(files) == 0 {
		return "GENESIS"
	}

	// Find most recent file
	var mostRecent string
	var mostRecentTime time.Time
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.ModTime().After(mostRecentTime) {
			mostRecentTime = info.ModTime()
			mostRecent = f
		}
	}

	if mostRecent == "" {
		return "GENESIS"
	}

	// Read last line
	file, err := os.Open(mostRecent)
	if err != nil {
		return "GENESIS"
	}
	defer file.Close()

	var lastLine string
	// Seek to near end
	info, _ := file.Stat()
	if info.Size() > 4096 {
		file.Seek(-4096, io.SeekEnd)
	}

	var buf [4096]byte
	n, _ := file.Read(buf[:])
	if n > 0 {
		lines := string(buf[:n])
		for i := len(lines) - 1; i >= 0; i-- {
			if lines[i] == '\n' && i < len(lines)-1 {
				lastLine = lines[i+1:]
				break
			}
		}
	}

	if lastLine == "" {
		return "GENESIS"
	}

	var event AuditEvent
	if err := json.Unmarshal([]byte(lastLine), &event); err != nil {
		return "GENESIS"
	}

	return event.Hash
}

// VerifyChain verifies the integrity of the audit log chain
func (l *AuditLogger) VerifyChain(logPath string) (int, []string, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return 0, nil, err
	}
	defer file.Close()

	var verified int
	var errors []string
	var prevHash string = "GENESIS"

	decoder := json.NewDecoder(file)
	for {
		var event AuditEvent
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			errors = append(errors, fmt.Sprintf("Parse error: %v", err))
			continue
		}

		// Verify chain
		if event.PrevHash != prevHash {
			errors = append(errors, fmt.Sprintf("Chain break at %s: expected prev_hash %s, got %s",
				event.ID, prevHash, event.PrevHash))
		}

		// Verify HMAC
		expectedHash := l.calculateHash(event)
		if event.Hash != expectedHash {
			errors = append(errors, fmt.Sprintf("Tampered event %s: hash mismatch", event.ID))
		}

		prevHash = event.Hash
		verified++
	}

	return verified, errors, nil
}

// Query queries audit logs with filters
type QueryFilter struct {
	StartTime   time.Time
	EndTime     time.Time
	Types       []EventType
	Severities  []EventSeverity
	User        string
	Source      string
	SuccessOnly bool
	FailureOnly bool
	Limit       int
}

// Query searches audit logs
func (l *AuditLogger) Query(filter QueryFilter) ([]AuditEvent, error) {
	files, err := filepath.Glob(filepath.Join(l.logDir, "audit_*.jsonl"))
	if err != nil {
		return nil, err
	}

	var results []AuditEvent

	for _, f := range files {
		events, err := l.queryFile(f, filter)
		if err != nil {
			continue
		}
		results = append(results, events...)

		if filter.Limit > 0 && len(results) >= filter.Limit {
			results = results[:filter.Limit]
			break
		}
	}

	return results, nil
}

func (l *AuditLogger) queryFile(path string, filter QueryFilter) ([]AuditEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var results []AuditEvent
	decoder := json.NewDecoder(file)

	for {
		var event AuditEvent
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			continue
		}

		if l.matchesFilter(event, filter) {
			results = append(results, event)
		}
	}

	return results, nil
}

func (l *AuditLogger) matchesFilter(event AuditEvent, filter QueryFilter) bool {
	// Time range
	if !filter.StartTime.IsZero() || !filter.EndTime.IsZero() {
		eventTime, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil {
			return false
		}
		if !filter.StartTime.IsZero() && eventTime.Before(filter.StartTime) {
			return false
		}
		if !filter.EndTime.IsZero() && eventTime.After(filter.EndTime) {
			return false
		}
	}

	// Event types
	if len(filter.Types) > 0 {
		found := false
		for _, t := range filter.Types {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Severities
	if len(filter.Severities) > 0 {
		found := false
		for _, s := range filter.Severities {
			if s == event.Severity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// User
	if filter.User != "" && event.User != filter.User {
		return false
	}

	// Source
	if filter.Source != "" && event.Source != filter.Source {
		return false
	}

	// Success/failure
	if filter.SuccessOnly && !event.Success {
		return false
	}
	if filter.FailureOnly && event.Success {
		return false
	}

	return true
}

// Close closes the audit logger
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.currentFile != nil {
		return l.currentFile.Close()
	}
	return nil
}

// ExportForCompliance exports audit logs in a compliance-friendly format
func (l *AuditLogger) ExportForCompliance(outputPath string, filter QueryFilter) error {
	events, err := l.Query(filter)
	if err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	report := struct {
		GeneratedAt string       `json:"generated_at"`
		Host        string       `json:"host"`
		EventCount  int          `json:"event_count"`
		Events      []AuditEvent `json:"events"`
	}{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Host:        l.hostname,
		EventCount:  len(events),
		Events:      events,
	}

	return encoder.Encode(report)
}

// ExportToCSV exports audit logs in CSV format for spreadsheet analysis
func (l *AuditLogger) ExportToCSV(outputPath string, filter QueryFilter) error {
	events, err := l.Query(filter)
	if err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"ID", "Timestamp", "Type", "Severity", "Action",
		"User", "Source", "Target", "Success", "Error", "Details",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write events
	for _, event := range events {
		// Flatten details map to string
		detailParts := make([]string, 0, len(event.Details))
		keys := make([]string, 0, len(event.Details))
		for k := range event.Details {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			detailParts = append(detailParts, fmt.Sprintf("%s=%s", k, event.Details[k]))
		}
		detailsStr := strings.Join(detailParts, "; ")

		row := []string{
			event.ID,
			event.Timestamp,
			string(event.Type),
			string(event.Severity),
			event.Action,
			event.User,
			event.Source,
			event.Target,
			fmt.Sprintf("%t", event.Success),
			event.ErrorMessage,
			detailsStr,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// GetStats returns summary statistics for audit logs
func (l *AuditLogger) GetStats(filter QueryFilter) (*AuditStats, error) {
	events, err := l.Query(filter)
	if err != nil {
		return nil, err
	}

	stats := &AuditStats{
		TotalEvents:  len(events),
		ByType:       make(map[EventType]int),
		BySeverity:   make(map[EventSeverity]int),
		ByUser:       make(map[string]int),
		SuccessCount: 0,
		FailureCount: 0,
	}

	for _, event := range events {
		stats.ByType[event.Type]++
		stats.BySeverity[event.Severity]++
		if event.User != "" {
			stats.ByUser[event.User]++
		}
		if event.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}
	}

	return stats, nil
}

// AuditStats contains summary statistics
type AuditStats struct {
	TotalEvents  int                   `json:"total_events"`
	ByType       map[EventType]int     `json:"by_type"`
	BySeverity   map[EventSeverity]int `json:"by_severity"`
	ByUser       map[string]int        `json:"by_user"`
	SuccessCount int                   `json:"success_count"`
	FailureCount int                   `json:"failure_count"`
}
