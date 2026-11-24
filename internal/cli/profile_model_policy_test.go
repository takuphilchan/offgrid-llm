package cli

import (
	"strings"
	"testing"
)

func TestProfileSettingsReaderPreservesLocalModelPolicy(t *testing.T) {
	input := `{"name":"Local","model":"primary","max_iterations":30,"model_policy":{"version":1,"mode":"guided","fallback_models":["fallback"],"perception_model":"vision"}}`
	settings, err := ReadProfileSettings("-", strings.NewReader(input))
	if err != nil || settings.ModelPolicy == nil || settings.ModelPolicy.Mode != "guided" || settings.ModelPolicy.FallbackModels[0] != "fallback" || settings.ModelPolicy.PerceptionModel != "vision" {
		t.Fatal(settings, err)
	}
	invalid := strings.Replace(input, `"mode":"guided"`, `"mode":"guided","grant":true`, 1)
	if _, err = ReadProfileSettings("-", strings.NewReader(invalid)); err == nil {
		t.Fatal("unknown policy fields accepted")
	}
}
