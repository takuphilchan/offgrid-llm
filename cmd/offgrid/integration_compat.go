package main

import "github.com/takuphilchan/offgrid-llm/internal/integrations"

// Legacy aliases retain their JSON field types. New clients use the generic
// lifecycle commands, or the additive assessment field on an alias response.
func legacyIntegrationProjection(action, id string, r integrations.HostReport, o integrations.HostOptions, err error) map[string]any {
	payload := map[string]any{"installed": r.Runtime.Installed, "executable": r.Runtime.Executable,
		"plugin_path": r.Runtime.PluginPath, "model_id": r.ModelID, "base_url": r.Runtime.BaseURL,
		"ready": r.Ready, "assessment": r}
	if action == "configure" {
		if id == "hermes" {
			payload["provider"] = "offgrid"
			payload["doctor_checked"] = o.RunDoctor && r.Phase == "configured"
			payload["doctor_ok"] = o.RunDoctor && err == nil && r.Phase == "configured"
		}
	} else if id == "hermes" {
		payload["plugin"], payload["service"], payload["runtime"] = r.Runtime.Plugin, checkPassed(r, "service"), r.Service
	} else {
		payload["plugin_loaded"], payload["configured"] = r.Runtime.Plugin, checkPassed(r, "configuration") && checkPassed(r, "context")
		payload["service_ready"] = r.Service.Ready && checkPassed(r, "service")
		if r.Runtime.ConfigError != "" {
			payload["config_error"] = r.Runtime.ConfigError
		}
	}
	payload["warnings"] = r.Service.Warnings
	if err != nil {
		payload["error"] = err.Error()
	}
	return payload
}

func checkPassed(r integrations.HostReport, id string) bool {
	for _, check := range r.Checks {
		if check.ID == id {
			return check.State == "passed"
		}
	}
	return false
}
