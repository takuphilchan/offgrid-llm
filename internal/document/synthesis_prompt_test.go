package document

import (
	"encoding/json"

	"strings"
	"testing"
)

func TestSynthesisPromptSeparatesModelQuotesFromSourceIdentities(t *testing.T) {
	traversal, batch := synthesisFixture(t, "requirements")
	prompt, err := traversal.Prompt(batch, "ar")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"text_sha256", "start_byte", "revision", "citation"} {
		if strings.Contains(prompt.User, `"`+forbidden+`"`) {
			t.Fatal("internal citation format exposed", forbidden)
		}
	}
	if !strings.Contains(prompt.User, "31 July") || !strings.Contains(prompt.User, `"e0"`) {
		t.Fatal("source text or evidence ID lost")
	}
	properties := prompt.Schema["properties"].(map[string]any)
	if properties["batch_id"].(map[string]any)["const"] != batch.ID {
		t.Fatal("batch ID not constrained")
	}
	items := properties["items"].(map[string]any)["items"].(map[string]any)
	if items["additionalProperties"] != false {
		t.Fatal("unknown fields admitted")
	}
	raw, err := json.Marshal(prompt.Schema)
	if err != nil || len(raw) > 64<<10 {
		t.Fatal("invalid schema bounds", err)
	}
}
