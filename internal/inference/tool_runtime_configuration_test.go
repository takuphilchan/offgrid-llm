package inference

import (
	"strings"
	"testing"
)

func TestToolRuntimeConfigurationPreservesSemanticsAcrossTransportRelocation(t *testing.T) {
	args := []string{"runtime", "-m", "model.gguf", "--port", "1234", "--chat-template-file", "temporary-a", "-c", "8192", "-ngl", "auto", "--cache-type-k", "q8_0", "-np", "1"}
	env := []string{"CUDA_VISIBLE_DEVICES=0", "UNRELATED_CREDENTIAL=not-in-fingerprint"}
	env = append(env, "LLAMA_API_KEY=private", "LLAMA_ARG_API_KEY=private")
	digest := strings.Repeat("a", 64)
	want, err := toolRuntimeConfiguration(args, env, "model.gguf", "", 1234, digest)
	if err != nil {
		t.Fatal(err)
	}
	relocated := append([]string(nil), args...)
	relocated[0], relocated[2], relocated[4], relocated[6] = "another-runtime-path", "relocated.gguf", "4567", "temporary-b"
	got, err := toolRuntimeConfiguration(relocated, []string{"UNRELATED_CREDENTIAL=changed", "CUDA_VISIBLE_DEVICES=0"}, "relocated.gguf", "", 4567, digest)
	if err != nil || got != want {
		t.Fatal("transport/artifact paths changed semantic identity", got, err)
	}
	for _, index := range []int{8, 10, 12, 14} {
		changed := append([]string(nil), args...)
		changed[index] = "different"
		got, err := toolRuntimeConfiguration(changed, env, "model.gguf", "", 1234, digest)
		if err != nil || got == want {
			t.Fatal("execution setting lost from fingerprint", index, err)
		}
	}
	got, err = toolRuntimeConfiguration(args, []string{"CUDA_VISIBLE_DEVICES=1"}, "model.gguf", "", 1234, digest)
	if err != nil || got == want {
		t.Fatal("device selection lost from fingerprint", err)
	}
	got, err = toolRuntimeConfiguration(args, env, "model.gguf", "", 1234, strings.Repeat("b", 64))
	if err != nil || got == want {
		t.Fatal("template replacement lost from fingerprint", err)
	}
}

func TestToolRuntimeConfigurationRejectsAmbiguousLaunch(t *testing.T) {
	valid := []string{"runtime", "-m", "model", "--port", "1234"}
	for _, args := range [][]string{
		nil, {"runtime"}, {"runtime", "-m", "other", "--port", "1234"},
		{"runtime", "-m", "model", "--port", "4567"},
		{"runtime", "-m", "model"},
		{"runtime", "-m", "model", "--port"},
		append(append([]string(nil), valid...), "--model", "model"),
		append(append([]string(nil), valid...), "--mmproj", "unexpected"),
	} {
		if _, err := toolRuntimeConfiguration(args, []string{}, "model", "", 1234, "template"); err == nil {
			t.Fatal("ambiguous launch accepted", args)
		}
	}
	for _, env := range [][]string{nil, {"CUDA_VISIBLE_DEVICES=0", "CUDA_VISIBLE_DEVICES=1"}} {
		if _, err := toolRuntimeConfiguration(valid, env, "model", "", 1234, "template"); err == nil {
			t.Fatal("unmeasured environment accepted")
		}
	}
}
