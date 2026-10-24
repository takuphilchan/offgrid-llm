package server

import (
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

func TestEvaluateIntegrationReportsContextAndModelReadiness(t *testing.T) {
	profile := integrations.NewDefaultRegistry().List()[0].Profile()
	models := []api.Model{{ID: "agent-model", Type: "llm"}}

	ready := evaluateIntegration(profile, models, profile.RecommendedContext, "agent-model")
	if !ready.Ready || ready.Status != "ready" || len(ready.Warnings) != 0 {
		t.Fatalf("expected ready integration, got %#v", ready)
	}
	if ready.ReadinessScope != "service_model" || ready.HostState != "not_inspected" || ready.WorkflowState != "unverified" || len(ready.Checks) == 0 {
		t.Fatalf("service readiness must not qualify a host or tool workflow: %#v", ready)
	}

	usable := evaluateIntegration(profile, models, profile.MinimumContext, "agent-model")
	if !usable.Ready || usable.Status != "ready" || len(usable.Warnings) != 1 {
		t.Fatalf("expected usable integration with recommendation, got %#v", usable)
	}

	notReady := evaluateIntegration(profile, models, 4096, "missing-model")
	if notReady.Ready || len(notReady.Warnings) != 2 {
		t.Fatalf("expected model and context warnings, got %#v", notReady)
	}
}

func TestHermesReadinessMatchesRuntimeContextFloor(t *testing.T) {
	adapter, ok := integrations.NewDefaultRegistry().Get("hermes")
	if !ok {
		t.Fatal("Hermes integration is missing")
	}
	profile := adapter.Profile()
	if profile.MinimumContext != 64000 {
		t.Fatalf("Hermes context minimum = %d, want 64000", profile.MinimumContext)
	}
	models := []api.Model{{ID: "agent-model", Type: "llm"}}
	if got := evaluateIntegration(profile, models, 8192, "agent-model"); got.Ready {
		t.Fatalf("8192-token Hermes integration must not be ready: %#v", got)
	}
	if got := evaluateIntegration(profile, models, 64000, "agent-model"); !got.Ready {
		t.Fatalf("64000-token Hermes integration should be ready: %#v", got)
	}
}

func TestIntegrationSelectionUsesInstalledModelContext(t *testing.T) {
	models := []api.Model{
		{ID: "a-32k", Type: "llm"},
		{ID: "b-64k", Type: "llm"},
		{ID: "c-8k", Type: "llm"},
	}
	windows := map[string]int{"a-32k": 32768, "b-64k": 65536, "c-8k": 8192}
	contextFor := func(id string) int { return windows[id] }
	if got := selectIntegrationModel(models, "", 64000, contextFor); got != "b-64k" {
		t.Fatalf("Hermes default = %q, want eligible downloaded model", got)
	}
	if got := selectIntegrationModel(models, "a-32k", 64000, contextFor); got != "a-32k" {
		t.Fatalf("explicit model changed to %q", got)
	}
	if got := selectIntegrationModel(models, "absent", 64000, contextFor); got != "" {
		t.Fatalf("unknown model %q was selected", got)
	}
	delete(windows, "b-64k")
	if got := selectIntegrationModel(models[:1], "", 64000, contextFor); got != "a-32k" {
		t.Fatalf("constrained model = %q", got)
	}
}

func TestValidateIntegrationBaseURL(t *testing.T) {
	for _, value := range []string{"http://127.0.0.1:11611", "https://offgrid.example/v1"} {
		if err := validateIntegrationBaseURL(value); err != nil {
			t.Fatalf("%s: %v", value, err)
		}
	}
	for _, value := range []string{"127.0.0.1:11611", "file:///tmp/offgrid", "http://user:secret@localhost"} {
		if err := validateIntegrationBaseURL(value); err == nil {
			t.Fatalf("expected %s to be rejected", value)
		}
	}
}
