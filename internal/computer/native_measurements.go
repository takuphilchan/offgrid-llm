package computer

import (
	"sync"
	"time"
)

// NativeMeasurements is opt-in instrumentation for owned workflow fixtures.
// No target IDs, text, arguments, paths, errors or provider strings are accepted.
// A nil stage means unmeasured, never a fabricated zero-latency measurement.
type NativeMeasurement struct {
	Calls        int     `json:"calls"`
	Milliseconds float64 `json:"milliseconds"`
}
type NativeMeasurementReport struct {
	Version      int                           `json:"version"`
	Stages       map[string]*NativeMeasurement `json:"stages"`
	ModelCalls   *int                          `json:"model_calls"`
	PromptTokens *int                          `json:"prompt_tokens"`
}
type NativeMeasurements struct {
	mu                       sync.Mutex
	stages                   map[string]NativeMeasurement
	modelCalls, promptTokens *int
}

func nativeMeasurementStage(stage string) bool {
	switch stage {
	case "discovery", "readiness", "observation", "preparation", "input", "verification", "model", "tool", "queue", "ui", "total":
		return true
	}
	return false
}
func (m *NativeMeasurements) Add(stage string, elapsed time.Duration) {
	if m == nil || !nativeMeasurementStage(stage) || elapsed < 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stages == nil {
		m.stages = map[string]NativeMeasurement{}
	}
	s := m.stages[stage]
	s.Calls++
	s.Milliseconds += float64(elapsed) / float64(time.Millisecond)
	m.stages[stage] = s
}
func (m *NativeMeasurements) RecordModel(calls int, tokens *int) {
	if m == nil || calls < 0 || (tokens != nil && *tokens < 0) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modelCalls = &calls
	m.promptTokens = nil
	if tokens != nil {
		copy := *tokens
		m.promptTokens = &copy
	}
}
func (m *NativeMeasurements) Report() NativeMeasurementReport {
	r := NativeMeasurementReport{Version: 1, Stages: map[string]*NativeMeasurement{}}
	for _, stage := range []string{"discovery", "readiness", "observation", "preparation", "input", "verification", "model", "tool", "queue", "ui", "total"} {
		r.Stages[stage] = nil
	}
	if m == nil {
		return r
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for stage, value := range m.stages {
		v := value
		r.Stages[stage] = &v
	}
	if m.modelCalls != nil {
		v := *m.modelCalls
		r.ModelCalls = &v
	}
	if m.promptTokens != nil {
		v := *m.promptTokens
		r.PromptTokens = &v
	}
	return r
}
