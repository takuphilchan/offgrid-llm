package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/cli"
	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

func TestIntegrationPresentationActionableAndBounded(t *testing.T) {
	var out bytes.Buffer
	r := integrations.HostReport{ID: "hermes", Phase: "inspect", ModelID: "exact-model-id", SelectionSource: "service_catalog",
		Checks: []integrations.Check{
			{ID: "runtime", State: "blocked", Message: "Runtime was not found."},
			{ID: "plugin", State: "not_checked", Message: "Dependent check"},
			{ID: "model", State: "blocked", Message: "Context capacity is insufficient."},
		}}
	renderIntegrationReport(cli.Presenter{Stdout: &out, Style: cli.PresentationStyle{Columns: 60}}, "status", r, integrations.HostOptions{ServerURL: "http://localhost:11611"})
	got := out.String()
	for _, want := range []string{"Hermes", "Setup needed", "Needs attention", "exact-model-id", "--model", "Context capacity"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q: %s", want, got)
		}
	}
	for _, unwanted := range []string{"service_catalog", "Dependent check", "preflight", "! ", "offgrid hermes install"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q: %s", unwanted, got)
		}
	}
	if strings.Count(got, "Runtime was not found.") != 1 {
		t.Fatal(got)
	}
}

func TestIntegrationSetupActionUsesSupportedCommand(t *testing.T) {
	for _, id := range []string{"hermes", "openclaw", "third"} {
		var out bytes.Buffer
		r := integrations.HostReport{ID: id, Checks: []integrations.Check{{ID: "runtime", State: "blocked", Message: "Runtime missing"}}}
		renderIntegrationReport(cli.Presenter{Stdout: &out, Style: cli.PresentationStyle{Columns: 100}}, "status", r, integrations.HostOptions{})
		want := "offgrid " + id + " install"
		if id == "third" {
			want = "offgrid integrations configure third"
		}
		if !strings.Contains(out.String(), want) {
			t.Fatal(out.String())
		}
	}
}
