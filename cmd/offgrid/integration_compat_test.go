package main

import (
	"encoding/json"
	"testing"

	"github.com/takuphilchan/offgrid-llm/internal/integrations"
)

func TestIntegrationAliasJSONKeepsLegacyFieldTypes(t *testing.T) {
	r := integrations.HostReport{Ready: true, Phase: "preflight_ready", Runtime: integrations.HostState{Installed: true, Plugin: true}, Service: integrations.ServiceStatus{Ready: true}, Checks: []integrations.Check{{ID: "service", State: "passed"}, {ID: "configuration", State: "passed"}, {ID: "context", State: "passed"}}}
	for _, id := range []string{"hermes", "openclaw"} {
		payload := legacyIntegrationProjection("status", id, r, integrations.HostOptions{}, nil)
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err = json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		key := "service"
		if id == "openclaw" {
			key = "service_ready"
		}
		if decoded[key] != true || decoded["assessment"] == nil || decoded["installed"] != true {
			t.Fatalf("legacy contract changed: %s", data)
		}
	}
}
