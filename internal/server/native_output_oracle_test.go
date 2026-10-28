package server

import "testing"

func TestObservedCommandOutputRepresentations(t *testing.T) {
	for _, tc := range []struct {
		name, answer, output string
		valid                bool
	}{
		{"plain", "Output:\n```\nok  fixture 0.002s\n```", "ok\tfixture\t0.002s\n", true},
		{"quoted", `Its output is: "ok  \tfixture\t0.002s\n".`, "ok  \tfixture\t0.002s\n", true},
		{"different", `Its output is: "ok  \tfixture\t0.003s\n".`, "ok  \tfixture\t0.002s\n", false},
		{"invented", "It succeeded with exit 0.", "ok fixture 0.002s\n", false},
		{"unquoted escapes", `Output: ok\tfixture\t0.002s\n`, "ok\tfixture\t0.002s\n", false},
		{"literal backslashes", `Output: "literal \\t bytes"`, `literal \t bytes`, true},
		{"cannot reinterpret literal", `Output: "literal \t bytes"`, `literal \t bytes`, false},
		{"truncated quote", `Output: "ok\tfixture\t0.002s\n`, "ok\tfixture\t0.002s\n", false},
		{"missing", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if reportsObservedOutput(tc.answer, tc.output) != tc.valid {
				t.Fatal("incorrect output judgment")
			}
		})
	}
}
