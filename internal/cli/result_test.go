package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/serviceclient"
)

func TestReportErrorCompatibility(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		code int
		wire string
	}{
		{"usage", Usage("Missing %s", "run ID"), 2, "invalid_usage"},
		{"operational", errors.New("failure"), 1, "operation_failed"},
		{"cancelled", context.Canceled, 130, "cancelled"},
		{"service", &serviceclient.Error{Code: "forbidden", Message: "Forbidden"}, 1, "forbidden"},
		{"server-url", &serviceclient.Error{Code: "invalid_server", Message: "Bad URL"}, 2, "invalid_server"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := ReportError(test.err, true, &stdout, &stderr); got != test.code || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", got, stdout.String(), stderr.String())
			}
			var result struct {
				Error serviceclient.Error `json:"error"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Error.Code != test.wire {
				t.Fatalf("wire=%q err=%v", stdout.String(), err)
			}
		})
	}
}

func TestTerminalSafePreservesReadableTextWithoutControls(t *testing.T) {
	got := TerminalSafe("before\x1b[31m\u202e\r\n\tafter")
	if strings.ContainsAny(got, "\x1b\u202e\r") || got != "before[31m\n\tafter" {
		t.Fatalf("sanitized=%q", got)
	}
}
