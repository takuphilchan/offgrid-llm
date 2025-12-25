// Package validation provides input validation utilities for API requests.
// This provides basic validation that improves error messages and prevents common issues.
package validation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult holds the result of validation
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validator provides chainable validation
type Validator struct {
	errors []ValidationError
}

// New creates a new Validator
func New() *Validator {
	return &Validator{errors: []ValidationError{}}
}

// Required validates that a field is not empty
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s is required", field),
		})
	}
	return v
}

// MinLength validates minimum string length
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if utf8.RuneCountInString(value) < min {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be at least %d characters", field, min),
		})
	}
	return v
}

// MaxLength validates maximum string length
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if utf8.RuneCountInString(value) > max {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be at most %d characters", field, max),
		})
	}
	return v
}

// Range validates a number is within range
func (v *Validator) Range(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be between %d and %d", field, min, max),
		})
	}
	return v
}

// Positive validates a number is positive
func (v *Validator) Positive(field string, value int) *Validator {
	if value <= 0 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s must be positive", field),
		})
	}
	return v
}

// Pattern validates against a regex pattern
func (v *Validator) Pattern(field, value, pattern, message string) *Validator {
	if value == "" {
		return v // Skip pattern check for empty values (use Required for that)
	}
	matched, err := regexp.MatchString(pattern, value)
	if err != nil || !matched {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: message,
		})
	}
	return v
}

// OneOf validates value is one of allowed values
func (v *Validator) OneOf(field, value string, allowed []string) *Validator {
	if value == "" {
		return v // Skip for empty values
	}
	for _, a := range allowed {
		if value == a {
			return v
		}
	}
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: fmt.Sprintf("%s must be one of: %s", field, strings.Join(allowed, ", ")),
	})
	return v
}

// NoPathTraversal validates a filename/path doesn't contain path traversal
func (v *Validator) NoPathTraversal(field, value string) *Validator {
	if strings.Contains(value, "..") || strings.Contains(value, "/") || strings.Contains(value, "\\") {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s contains invalid characters", field),
		})
	}
	return v
}

// SafeFilename validates a filename is safe
func (v *Validator) SafeFilename(field, value string) *Validator {
	if value == "" {
		return v
	}
	// Allow alphanumeric, dash, underscore, dot, but no path separators
	safePattern := `^[a-zA-Z0-9._-]+$`
	matched, _ := regexp.MatchString(safePattern, value)
	if !matched {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("%s contains invalid characters (only alphanumeric, dash, underscore, dot allowed)", field),
		})
	}
	return v
}

// Valid returns true if there are no validation errors
func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

// Result returns the validation result
func (v *Validator) Result() ValidationResult {
	return ValidationResult{
		Valid:  len(v.errors) == 0,
		Errors: v.errors,
	}
}

// WriteError writes validation errors as JSON response
func (v *Validator) WriteError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "validation_failed",
		"message": "Request validation failed",
		"details": v.errors,
	})
}

// FirstError returns the first error message, or empty string if valid
func (v *Validator) FirstError() string {
	if len(v.errors) > 0 {
		return v.errors[0].Message
	}
	return ""
}

// Common validators as standalone functions for convenience

// IsValidUsername checks if username is valid (alphanumeric + underscore, 3-32 chars)
func IsValidUsername(username string) bool {
	if len(username) < 3 || len(username) > 32 {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, username)
	return matched
}

// IsValidModelID checks if a model ID looks valid (basic sanity check)
func IsValidModelID(modelID string) bool {
	if modelID == "" || len(modelID) > 256 {
		return false
	}
	// No path traversal
	if strings.Contains(modelID, "..") {
		return false
	}
	return true
}

// SanitizeFilename removes or replaces unsafe characters from a filename
func SanitizeFilename(filename string) string {
	// Remove path separators and traversal
	filename = strings.ReplaceAll(filename, "..", "")
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")
	// Replace other problematic characters
	unsafe := []string{"<", ">", ":", "\"", "|", "?", "*"}
	for _, char := range unsafe {
		filename = strings.ReplaceAll(filename, char, "_")
	}
	// Trim spaces and dots from start/end
	filename = strings.Trim(filename, " .")
	return filename
}
