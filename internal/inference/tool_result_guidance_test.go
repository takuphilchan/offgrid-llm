package inference

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestResultFramingPreservesOriginalNativeContent(t *testing.T) {
	source := `set tools_in_user_message = true
You have access to the following functions. To call a function, please respond with JSON for a function call.
Given the following functions, please respond with a JSON for a function call with its proper arguments that best answers the given prompt.
{{ system_message }} {{ messages }} {{ tools }} {{ tool_call.arguments | tojson }}
<|start_header_id|>ipython<|end_header_id|>{{ message.content | tojson }}<|eot_id|>`
	v1, err := selectedToolFraming(source, optionalToolsAdapter)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := selectedToolFraming(source, resultToolsAdapter)
	if err != nil {
		t.Fatal(err)
	}
	guidance := strings.TrimSpace(toolResultGuidance)
	if strings.Count(v2, guidance) != 1 || strings.Replace(v2, guidance+" ", "", 1) != v1 {
		t.Fatal("changed native messages, schemas or result serialization")
	}
	original, _ := optionalToolFraming(source)
	if v1 != original {
		t.Fatal("changed v1 behavior")
	}
	for _, required := range []string{"outer line-number prefix", "real data and must remain", "Unicode", "truncated", "not its agreement", "evidence differs", "not a file write"} {
		if !strings.Contains(guidance, required) {
			t.Fatalf("missing interpretation boundary: %s", required)
		}
	}
	if _, err := selectedToolFraming(source, "unknown"); err == nil {
		t.Fatal("accepted unknown revision")
	}
	t.Setenv("OFFGRID_TOOL_TEMPLATE_ADAPTER", resultToolsAdapter)
	args, cleanup, err := prepareToolTemplate(context.Background(), "unused", "missing-model")
	cleanup()
	if err != nil || len(args) != 0 {
		t.Fatal("overrode an unknown model template")
	}
}

func TestToolTemplateRevisionDigestsAreDistinct(t *testing.T) {
	for digest, expected := range map[string]string{
		optionalToolsAdaptedSHA: optionalToolsAdapter,
		resultToolsAdaptedSHA:   resultToolsAdapter,
		optionalToolsSourceSHA:  "", "unknown": "",
	} {
		if got := toolTemplateAdapterIdentity(digest); got != expected {
			t.Fatalf("%s: got %q want %q", digest, got, expected)
		}
	}
}

func TestResultFramingRequiresReviewedRuntimeRepair(t *testing.T) {
	for _, revision := range []string{"b95502b", "b95502b-offgrid-seq1", "b95502b-offgrid-seq2", "b95502b-unknown"} {
		version := "version: 0.1.2-dev (build 1, commit " + revision + ")"
		if got := toolTemplateRuntimeMatches(version, resultToolsAdapter); got != (revision == "b95502b-offgrid-seq2") {
			t.Fatalf("unexpected v2 runtime compatibility: %s", revision)
		}
		if got := toolTemplateRuntimeMatches(version, optionalToolsAdapter); got != (revision != "b95502b-unknown") {
			t.Fatalf("changed v1 runtime compatibility: %s", revision)
		}
	}
}

func TestPinnedToolTemplateFixture(t *testing.T) {
	path := os.Getenv("OFFGRID_TEST_PINNED_TEMPLATE")
	if path == "" {
		t.Skip("optional pinned upstream fixture; no model download")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := strings.TrimSuffix(string(data), "\n")
	if templateSHA(source) != optionalToolsSourceSHA {
		t.Fatal("fixture is not the reviewed source")
	}
	for _, selection := range []string{optionalToolsAdapter, resultToolsAdapter} {
		adapted, err := selectedToolFraming(source, selection)
		if err != nil || toolTemplateAdapterIdentity(templateSHA(adapted)) != selection {
			t.Fatalf("%s output pin: %v", selection, err)
		}
	}
}
