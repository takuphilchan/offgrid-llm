package agents

import "testing"

func TestSpecialistNamesNeverForceDelegation(t *testing.T) {
	for _, prompt := range []string{
		"Do not use Researcher or Writer. Answer yourself.",
		"Explain the difference between Researcher and Writer.",
		"Use Researcher then Writer if useful.",
	} {
		task := &Task{Prompt: prompt, Config: AgentConfig{Profile: &AgentProfile{}, DelegateProfiles: map[string]AgentConfig{
			"research": {Profile: &AgentProfile{Settings: ProfileSettings{Name: "Researcher"}}},
			"write":    {Profile: &AgentProfile{Settings: ProfileSettings{Name: "Writer"}}},
		}}}
		if DelegationRequired(task) {
			t.Fatalf("prompt forced delegation: %q", prompt)
		}
		tool := profileDelegationTool(task)
		tasks := tool.Function.Parameters["properties"].(map[string]interface{})["tasks"].(map[string]interface{})
		if tasks["minItems"] != 1 {
			t.Fatalf("names forced a multi-worker graph: %v", tasks["minItems"])
		}
	}
}
