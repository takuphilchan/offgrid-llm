package inference

import "errors"

const (
	// ErrCodeMissingMmproj indicates the backend rejected an image request because the model is missing its mmproj adapter
	ErrCodeMissingMmproj = "missing_mmproj"
)

// EngineError wraps structured errors returned by inference engines so callers can react intelligently
// to known failure modes (e.g., missing adapters for VLMs) without string matching everywhere.
type EngineError struct {
	Code    string
	Message string
	Details string
}

func (e *EngineError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Details
}

// AsEngineError returns the EngineError if the provided error chain contains one.
func AsEngineError(err error) *EngineError {
	var engineErr *EngineError
	if errors.As(err, &engineErr) {
		return engineErr
	}
	return nil
}
