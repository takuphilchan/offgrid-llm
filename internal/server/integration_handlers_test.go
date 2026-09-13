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
