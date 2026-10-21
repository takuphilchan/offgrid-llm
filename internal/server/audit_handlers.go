package server

import (
	"log"

	"github.com/takuphilchan/offgrid-llm/internal/audit"
)

// auditLog logs an event if audit logging is enabled (enterprise feature)
func (s *Server) auditLog(eventType audit.EventType, action string, user string, source string, details map[string]string, success bool) {
	if s.auditLogger == nil {
		return
	}
	event := audit.AuditEvent{
		Type:    eventType,
		Action:  action,
		User:    user,
		Source:  source,
		Details: details,
		Success: success,
	}
	if success {
		event.Severity = audit.SeverityInfo
	} else {
		event.Severity = audit.SeverityWarning
	}
	if err := s.auditLogger.LogEvent(event); err != nil {
		log.Printf("Warning: Failed to write audit log: %v", err)
	}
}
