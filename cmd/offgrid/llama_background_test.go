package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestReviewedLlamaLaunchKeepsModelPathAsOneArgument(t *testing.T) {
	modelPath := `C:\models\test model's $(never-run).gguf`
	args, err := reviewedLlamaServerArgs(modelPath, "42382", 4, 4096, 99, 512, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-m", modelPath, "--port", "42382", "--host", "127.0.0.1", "-t", "4", "-c", "4096", "-ngl", "99", "-b", "512", "--no-mmap", "--mlock", "-fa", "on", "--cont-batching", "--cache-type-k", "q8_0", "--cache-type-v", "q8_0", "--cache-reuse", "256"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("argv=%#v", args)
	}
	low, err := reviewedLlamaServerArgs(modelPath, "42382", 4, 2048, 0, 256, true)
	if err != nil || strings.Join(low, " ") == "" {
		t.Fatalf("low-memory argv=%#v err=%v", low, err)
	}
	for _, value := range low {
		if value == "--no-mmap" || value == "--mlock" {
			t.Fatalf("high-memory flag in low-memory argv: %#v", low)
		}
	}
}

func TestReviewedLlamaLaunchRejectsInvalidParametersWithoutSpawning(t *testing.T) {
	for _, test := range []struct {
		model, port string
		threads     int
	}{
		{"", "42382", 4},
		{"model.gguf", "not-a-port", 4},
		{"model.gguf", "70000", 4},
		{"model.gguf", "42382", 0},
		{"bad\x00model.gguf", "42382", 4},
	} {
		if _, err := reviewedLlamaServerArgs(test.model, test.port, test.threads, 4096, 0, 512, false); err == nil {
			t.Fatalf("accepted invalid launch %#v", test)
		}
	}
	if err := runLlamaBackground("", []string{"--help"}); err == nil {
		t.Fatal("empty executable was accepted")
	}
}
