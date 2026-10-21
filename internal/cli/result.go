package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

// UsageError carries an invalid-usage outcome across command families.
type UsageError struct{ message string }

// ReportedError preserves legacy commands that print their own diagnostic and
// exit with status 1. The boundary owns the exit without printing twice.
type ReportedError struct{}

func (ReportedError) Error() string { return "command error already reported" }

// LegacyReportedError carries old local-admin failures which historically
// printed an error but returned a successful process status. The contract is
// retained until a separately approved CLI behavior change.
type LegacyReportedError struct{}

func (LegacyReportedError) Error() string { return "legacy command error already reported" }

func (e *UsageError) Error() string { return e.message }

func Usage(format string, args ...any) error {
	return &UsageError{message: fmt.Sprintf(format, args...)}
}

// TerminalSafe removes control and format characters from untrusted output.
// Newlines and tabs remain so multiline errors retain their useful structure.
func TerminalSafe(value string) string {
	return strings.Map(func(r rune) rune {
		if (unicode.IsControl(r) && r != '\n' && r != '\t') || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, value)
}

// ReportError renders the existing CLI result envelope through injected
// streams. Only the process entry turns its returned code into os.Exit.
func ReportError(err error, jsonMode bool, stdout, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	var reported ReportedError
	if errors.As(err, &reported) {
		return 1
	}
	var legacy LegacyReportedError
	if errors.As(err, &legacy) {
		return 0
	}
	code := 1
	detail := &serviceclient.Error{Code: "operation_failed", Message: TerminalSafe(err.Error())}
	var invalid *UsageError
	var serviceErr *serviceclient.Error
	switch {
	case errors.Is(err, context.Canceled):
		code = 130
		detail = &serviceclient.Error{Code: "cancelled", Message: "Operation cancelled. Inspect saved state before retrying."}
	case errors.As(err, &invalid):
		code = 2
		detail.Code = "invalid_usage"
	case errors.As(err, &serviceErr):
		copy := *serviceErr
		copy.Message = TerminalSafe(err.Error())
		detail = &copy
		if detail.Code == "invalid_server" {
			code = 2
		}
	}
	if jsonMode {
		_ = json.NewEncoder(stdout).Encode(map[string]any{"error": detail})
	} else {
		_, _ = fmt.Fprintf(stderr, "ERR %s\n", TerminalSafe(detail.Message))
	}
	return code
}
