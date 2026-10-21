package server

import (
	"errors"
	"testing"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestSyntheticProbeUsageRejectsReportedOveruseAndInconsistency(t *testing.T) {
	for _, tc := range []struct {
		name  string
		usage api.Usage
		want  bool
	}{
		{name: "within limit", usage: api.Usage{PromptTokens: 10, CompletionTokens: 12, TotalTokens: 22}},
		{name: "legacy unmeasured", usage: api.Usage{}},
		{name: "over limit", usage: api.Usage{CompletionTokens: 193}, want: true},
		{name: "inconsistent total", usage: api.Usage{PromptTokens: 10, CompletionTokens: 12, TotalTokens: 20}, want: true},
		{name: "negative", usage: api.Usage{CompletionTokens: -1}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkSyntheticProbeUsage(&api.ChatCompletionResponse{Usage: tc.usage}, 192)
			if errors.Is(err, errProbeOutputBudget) != tc.want {
				t.Fatalf("usage=%+v error=%v, want rejected=%t", tc.usage, err, tc.want)
			}
		})
	}
}
