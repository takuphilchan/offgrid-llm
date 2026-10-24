package inference

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestOptionalToolFramingPreservesNativeProtocol(t *testing.T) {
	// Small synthetic fixture, not a bundled copy of a model's full template.
	source := `set tools_in_user_message = true
You have access to the following functions. To call a function, please respond with JSON for a function call.
Given the following functions, please respond with a JSON for a function call with its proper arguments that best answers the given prompt.
{{ system_message }} {{ messages }} {{ tools }} {{ tool_call.arguments | tojson }}
{% if message.content is mapping or message.content is iterable %}{{ message.content | tojson }}{% else %}{{ message.content }}{% endif %}
<|start_header_id|>ipython<|end_header_id|>{{ message.content }}<|eot_id|>`
	adapted, err := optionalToolFraming(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, retained := range []string{"{{ system_message }}", "{{ messages }}", "{{ tools }}", "{{ tool_call.arguments | tojson }}", "<|start_header_id|>ipython<|end_header_id|>{{ message.content }}<|eot_id|>"} {
		if !strings.Contains(adapted, retained) {
			t.Fatalf("lost %s", retained)
		}
	}
	if !strings.Contains(adapted, "set tools_in_user_message = false") || !strings.Contains(adapted, "If no function is needed, answer the user normally") {
		t.Fatal("optional-tool framing missing")
	}
	if !strings.Contains(adapted, "{% if message.content is mapping or message.content is iterable %}{{ message.content | tojson }}{% else %}{{ message.content }}{% endif %}") {
		t.Fatal("native result serialization changed")
	}
	if _, err := optionalToolFraming(adapted); err == nil {
		t.Fatal("accepted already altered framing")
	}
	if _, err := optionalToolFraming(source + "set tools_in_user_message = true"); err == nil {
		t.Fatal("accepted ambiguous framing")
	}
}

func TestToolTemplateSelectionIsExplicitAndPinned(t *testing.T) {
	t.Setenv("OFFGRID_TOOL_TEMPLATE_ADAPTER", "")
	args, cleanup, err := prepareToolTemplate(context.Background(), "unused-binary", "missing-model")
	cleanup()
	if err != nil || len(args) != 0 {
		t.Fatal("changed default path")
	}
	t.Setenv("OFFGRID_TOOL_TEMPLATE_ADAPTER", optionalToolsAdapter)
	args, cleanup, err = prepareToolTemplate(context.Background(), "unused-binary", "missing-model")
	cleanup()
	if err != nil || len(args) != 0 {
		t.Fatal("guessed an unknown template")
	}
	t.Setenv("OFFGRID_TOOL_TEMPLATE_ADAPTER", "untrusted-adapter")
	if _, _, err = prepareToolTemplate(context.Background(), "unused-binary", "missing-model"); err == nil {
		t.Fatal("accepted arbitrary adapter")
	}
	if !optionalToolsRuntime("version: 0.1.2-dev (build 1, commit b95502b)\nbuilt with fixture") {
		t.Fatal("pinned runtime not recognized")
	}
	if !optionalToolsRuntime("version: 0.1.2-dev (build 1, commit b95502b-offgrid-seq1)\nbuilt with fixture") {
		t.Fatal("reviewed sequential runtime revision not recognized")
	}
	if !optionalToolsRuntime("version: 0.1.2-dev (build 1, commit b95502b-offgrid-seq2)\nbuilt with fixture") {
		t.Fatal("reviewed bare JSON runtime revision not recognized")
	}
	for _, version := range []string{"b95502b", "version: 0.1.2-dev (build 2, commit b95502b)", "version: 0.1.2-dev (build 1, commit b95502bx)", "error: version: 0.1.2-dev (build 1, commit b95502b)"} {
		if optionalToolsRuntime(version) {
			t.Fatalf("matched unknown runtime %q", version)
		}
	}
}

func TestPrivateToolTemplateCleanup(t *testing.T) {
	path, cleanup, err := privateToolTemplate("reviewed fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "reviewed fixture" {
		t.Fatalf("%s %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatal("template not private")
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("template file not cleaned up")
	}
}
