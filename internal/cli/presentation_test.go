package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestPresenterUsesInjectedStreamsAndJSONSuppression(t *testing.T) {
	var stdout, stderr bytes.Buffer
	p := Presenter{Stdout: &stdout, Stderr: &stderr, Style: PresentationStyle{Primary: "P", Bold: "B", Muted: "M", Reset: "R", Success: "S", Check: "OK", Error: "E", Cross: "ERR", Accent: "A"}}
	p.Banner("0.4.18")
	p.Section("Models")
	p.Success("ready")
	p.Error("failed")
	p.Info("info")
	p.Warning("warn")
	if !strings.Contains(stdout.String(), "PBOffGrid LLMR M0.4.18R") ||
		!strings.Contains(stdout.String(), "PBModelsR") ||
		!strings.Contains(stdout.String(), "SOKR ready") ||
		!strings.Contains(stdout.String(), "PiR info") ||
		stderr.String() != "EERRR failed\nA!R warn\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	stdout.Reset()
	p.JSON = true
	p.Banner("ignored")
	p.Section("ignored")
	if stdout.Len() != 0 {
		t.Fatalf("JSON mode emitted presentation text: %q", stdout.String())
	}
}

func TestPresenterLegacyHelpersUseInjectedStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	p := Presenter{
		Stdout: &stdout,
		Stderr: &stderr,
		Style: PresentationStyle{
			Primary: "P", Secondary: "S", Muted: "M", Bold: "B", Dim: "D", Reset: "R",
			Error: "E", Cross: "X", Check: "OK", Chevron: ">", Separator: "-",
			BoxTL: "+", BoxTR: "+", BoxBL: "+", BoxBR: "+", BoxH: "-", BoxV: "|", BoxVR: "+", BoxVL: "+",
		},
	}
	p.Item("Model", "ready")
	p.Option("--json", "machine-readable")
	p.Example("offgrid version")
	p.SectionHeader("Models")
	p.Box("Result", "ok")
	p.Card("Detail", []string{"line"})
	p.Divider()
	p.SubtleDivider()
	p.HelpfulError(fmt.Errorf("connection refused"), "Connect")
	got := stdout.String()
	for _, want := range []string{
		"MModel:              R BreadyR\n",
		"S--json                    R machine-readable\n",
		"Doffgrid versionR\n",
		"\nP> ModelsR\n\n",
		"P+----------------------------------------------------------+R\n",
		"M+--R BPDetailR M",
		"M------------------------------------------------------------R\n",
		"PiR Possible causes:\n",
		"  • OffGrid server not running\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stdout does not contain %q; got %q", want, got)
		}
	}
	if stderr.String() != "EXR Connect failed: connection refused\n" {
		t.Errorf("stderr=%q", stderr.String())
	}
	stdout.Reset()
	p.JSON = true
	p.SectionHeader("hidden")
	p.Card("hidden", []string{"hidden"})
	p.SubtleDivider()
	if stdout.Len() != 0 {
		t.Fatalf("JSON mode emitted decorative output: %q", stdout.String())
	}
}
