package document

import "testing"

func TestSynthesisSummaryGrammarMatchesExistingCompiler(t *testing.T) {
	for _, workflow := range []string{"summary", "requirements"} {
		schema := synthesisResponseSchema(&SynthesisBatch{ID: "batch", Workflow: workflow})
		properties := schema["properties"].(map[string]any)
		fields := properties["items"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
		want := 8
		if workflow == "summary" {
			want = 0
		}
		for _, field := range []string{"conditions", "exceptions"} {
			if fields[field].(map[string]any)["maxItems"] != want {
				t.Fatal(workflow, field)
			}
		}
		if fields["evidence"].(map[string]any)["minItems"] != 1 {
			t.Fatal("summary must retain primary citations")
		}
		_, constrained := properties["no_findings"].(map[string]any)["const"]
		if constrained != (workflow == "summary") {
			t.Fatal("workflow disposition mismatch")
		}
	}
}
