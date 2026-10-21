package main

import "testing"

func TestAgentLegacyServiceSelection(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want bool
	}{
		{"interactive", nil, true},
		{"legacy list", []string{"tasks"}, true},
		{"legacy mcp", []string{"mcp"}, true},
		{"profile", []string{"profiles"}, false},
		{"task", []string{"run"}, false},
		{"artifact", []string{"artifact"}, false},
		{"control", []string{"resume"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := agentUsesLegacyService(tt.args); got != tt.want {
				t.Fatalf("agentUsesLegacyService(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
