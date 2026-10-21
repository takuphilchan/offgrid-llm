package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// This validates a frozen evaluation definition, not any model or native app.
func TestWorkflowEvaluationManifest(t *testing.T) {
	data, err := os.ReadFile("testdata/workflows-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version     int     `json:"version"`
		Repetitions int     `json:"repetitions"`
		Threshold   float64 `json:"qualification_threshold"`
		Cases       []struct {
			ID        string   `json:"id"`
			Family    string   `json:"family"`
			Prompt    string   `json:"prompt"`
			Setup     string   `json:"setup"`
			Checks    []string `json:"checks"`
			Forbidden []string `json:"forbidden_effects"`
			Native    *bool    `json:"isolated_native"`
		} `json:"cases"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 1 || manifest.Repetitions != 3 || manifest.Threshold != .9 || len(manifest.Cases) != 30 {
		t.Fatal("evaluation gate or frozen case count changed; version the suite explicitly")
	}
	families := map[string]int{"conversation": 0, "browser": 0, "document": 0, "spreadsheet": 0, "files": 0, "collaboration": 0}
	seen := map[string]bool{}
	for _, c := range manifest.Cases {
		if seen[c.ID] || !strings.HasPrefix(c.ID, c.Family+"-") {
			t.Fatalf("invalid/duplicate case %q", c.ID)
		}
		seen[c.ID] = true
		if _, ok := families[c.Family]; !ok {
			t.Fatalf("unknown family %q", c.Family)
		}
		families[c.Family]++
		if c.Native == nil || strings.TrimSpace(c.Prompt) == "" || strings.TrimSpace(c.Setup) == "" || len(c.Checks) == 0 || len(c.Forbidden) == 0 {
			t.Fatalf("case %q lacks explicit scope or acceptance", c.ID)
		}
		for _, text := range append(append([]string{}, c.Checks...), c.Forbidden...) {
			if strings.TrimSpace(text) == "" {
				t.Fatalf("empty criterion in %q", c.ID)
			}
		}
	}
	for family, count := range families {
		if count != 5 {
			t.Fatalf("%s has %d cases, want 5", family, count)
		}
	}
	// Canonical JSON makes the evidence identity independent of CRLF/indentation.
	var canonical any
	if err := json.Unmarshal(data, &canonical); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	const frozenDigest = "ae3b4a2d00db4946daf589112dd9492afcebda319664ec99478df8f87e0e3e89"
	if hex.EncodeToString(digest[:]) != frozenDigest {
		t.Fatalf("workflow suite digest changed: %x; publish a new suite version instead of silently changing qualification", digest)
	}
}
